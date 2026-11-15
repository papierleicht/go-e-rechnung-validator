"""Compare go-e-rechnung-validator with the latest official reference validators.

KoSIT (XRechnung): downloads the newest itplr-kosit/validator, validator-configuration-xrechnung
and xrechnung-testsuite and checks
  1. drift:    bundled scenarios.xml and Schematron XSLs equal the latest configuration,
  2. corpus:   testdata/corpus/xrechnung-*.xml get the same verdict, rule IDs and severities,
  3. suite:    the official testsuite instances (standard + CIUS technical cases) likewise.
Mustang (ZUGFeRD / Factur-X): downloads the newest ZUGFeRD/mustangproject CLI and checks
  4. corpus:   testdata/corpus/facturx-*.xml get the same verdict and rule IDs,
  5. broken:   deliberately broken copies are rejected by both for the same rule IDs,
  6. PDF:      testdata/corpus/facturx-*.pdf are valid for Mustang (PDF/A-3 via veraPDF and the
               embedded XML) and accepted by this library.

Writes parity/results.json, from which `go run ./internal/paritysvg` draws the README graphics.
Requires Java 21+, Go and network access (GitHub API). Python stdlib only.

  python3 parity/parity.py [--cache DIR] [--offline] [--only kosit|mustang]

Exit code 0 = parity, 1 = drift or deviation, 2 = could not run.
"""
from collections import Counter
from concurrent.futures import ThreadPoolExecutor
from datetime import date
from pathlib import Path
import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
import urllib.request
import xml.etree.ElementTree as ET
import zipfile

root = Path(__file__).resolve().parents[1]
corpus = root / "testdata/corpus"
rules = root / "rules/xrechnung"
DEFAULT_CACHE = Path.home() / ".cache/go-e-rechnung-validator-parity"

# ---------------------------------------------------------------- helpers

# Options that silence warnings of newer JDKs; each is kept only if the
# installed java accepts it, so an older runtime still starts the validators.
JAVA_OPTIONS = ["--enable-native-access=ALL-UNNAMED", "--enable-final-field-mutation=ALL-UNNAMED"]


def java_command():
    accepted = []
    for option in JAVA_OPTIONS:
        probe = subprocess.run(["java", option, "-version"], capture_output=True, text=True, check=False)
        if probe.returncode == 0 and "Unrecognized option" not in probe.stderr:
            accepted.append(option)
    return ["java", *accepted]


def api(path):
    request = urllib.request.Request("https://api.github.com/" + path, headers={"Accept": "application/vnd.github+json"})
    if os.environ.get("GITHUB_TOKEN"):
        request.add_header("Authorization", "Bearer " + os.environ["GITHUB_TOKEN"])
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def fetch(repo, asset_ok, cache, offline):
    """Download the matching asset of the latest release of repo (cached by tag, sha256-verified)."""
    name = repo.split("/")[1]
    if offline:
        cached = [p for p in sorted(cache.glob(name + "-*/*")) if asset_ok(p.name)]
        if not cached:
            sys.exit(f"offline: nothing cached for {repo}")
        return cached[-1].parent.name.removeprefix(name + "-"), cached[-1]
    release = api(f"repos/{repo}/releases/latest")
    asset = next(a for a in release["assets"] if asset_ok(a["name"]))
    target = cache / f"{name}-{release['tag_name']}" / asset["name"]
    if not target.exists():
        target.parent.mkdir(parents=True, exist_ok=True)
        urllib.request.urlretrieve(asset["browser_download_url"], target)
    digest = (asset.get("digest") or "").removeprefix("sha256:")
    if digest and sha256(target) != digest:
        target.unlink()
        sys.exit(f"sha256 mismatch for {asset['name']}")
    return release["tag_name"], target


def build_cli(work):
    binary = work / ("e-rechnung-validate" + (".exe" if os.name == "nt" else ""))
    subprocess.run(["go", "build", "-o", str(binary), "./cmd/e-rechnung-validate"], cwd=root, check=True)
    return binary


def validate(cli, paths):
    """Run this library's CLI on files; returns {path: result dict} (state, format, issues)."""
    results = {}
    paths = list(paths)
    for start in range(0, len(paths), 200):
        chunk = paths[start:start + 200]
        run = subprocess.run([str(cli), "-json", *map(str, chunk)], capture_output=True, text=True, check=False)
        try:
            for item in json.loads(run.stdout):
                results[Path(item["file"])] = item
        except json.JSONDecodeError:
            sys.exit("validator CLI failed: " + run.stderr[-500:])
    return results


def accepts(result):
    return result["state"] in ("valid", "warning")


def check(label, total, failures, unit="documents"):
    """One row of the parity graphic; matching counts the documents (or files) without deviation."""
    return {"label": label, "unit": unit, "total": total, "matching": total - len({f.split(":")[0] for f in failures})}


# ---------------------------------------------------------------- KoSIT

REP = "{http://www.xoev.de/de/validator/varl/1}"
SCN = "{http://www.xoev.de/de/validator/framework/1/scenarios}"
# bundled file (relative to rules/xrechnung) -> file in the KoSIT configuration zip
BUNDLED = {
    "scenarios.xml": "scenarios.xml",
    "resources/cii/16b/xsl/EN16931-CII-validation.xsl": "resources/cii/16b/xsl/EN16931-CII-validation.xsl",
    "resources/ubl/2.1/xsl/EN16931-UBL-validation.xsl": "resources/ubl/2.1/xsl/EN16931-UBL-validation.xsl",
}
# Testsuite folders for the supported profiles (extension and CVD scenarios are not bundled).
SUITE = ["instances/standard", "instances/technical-cases/cius"]


def kosit_messages(report):
    """(accepted, [(ruleId, effective severity)]) from a KoSIT report.

    The report keeps the raw level in rep:message/@level; the matched scenario's customLevel
    entries override it (rep:custom-level in KoSIT's default-report.xsl), and acceptance uses that.
    """
    tree = ET.parse(report).getroot()
    accepted = tree.find(f"{REP}assessment/{REP}accept") is not None
    custom = {}
    for entry in tree.iter(f"{SCN}customLevel"):
        for code in (entry.text or "").split():
            custom[code] = entry.get("level")
    found = []
    for message in tree.iter(f"{REP}message"):
        code = message.get("code")
        level = custom.get(code) or message.get("level", "error")
        found.append((code, "error" if level == "fatal" else level))
    return accepted, found


def run_kosit(java, jar, config, out, files):
    run = subprocess.run([*java, "-jar", str(jar), "-s", str(config / "scenarios.xml"), "-r", str(config), "-o", str(out), *map(str, files)],
                         stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True, check=False)
    return f"exit {run.returncode}: " + "\n".join(run.stderr.strip().splitlines()[-15:])


def compare_kosit(name, result, accepted, reference):
    """Deviations between one library result and one KoSIT report; empty list = parity."""
    actual = [(i["ruleId"], "error" if i["severity"] == "fatal" else i["severity"]) for i in result["issues"] if i["stage"] == "rules"]
    # KoSIT lists schema findings in the same message list; the library reports them as another stage.
    schema_codes = {i["ruleId"] for i in result["issues"] if i["stage"] != "rules"}
    expected = Counter({k: v for k, v in Counter(reference).items() if k[0] not in schema_codes})
    problems = []
    if accepts(result) != accepted:
        problems.append(f"{name}: KoSIT {'accepts' if accepted else 'rejects'}, library state={result['state']}")
    if Counter(actual) != expected:
        problems.append(f"{name}: only library {dict(Counter(actual) - expected)}; only KoSIT {dict(expected - Counter(actual))}")
    return problems


def kosit(java, cli, work, cache, offline):
    jar_tag, jar = fetch("itplr-kosit/validator", lambda n: n.endswith("-standalone.jar"), cache, offline)
    cfg_tag, cfg_zip = fetch("itplr-kosit/validator-configuration-xrechnung",
                             lambda n: n.endswith(".zip") and "validator-configuration" in n, cache, offline)
    suite_tag, suite_zip = fetch("itplr-kosit/xrechnung-testsuite", lambda n: n.endswith(".zip") and "testsuite" in n, cache, offline)
    print(f"KoSIT validator {jar_tag}, configuration {cfg_tag}, testsuite {suite_tag}")
    config, suite = work / "config", work / "testsuite"
    for archive_path, target in ((cfg_zip, config), (suite_zip, suite)):
        with zipfile.ZipFile(archive_path) as archive:
            archive.extractall(target)

    drift = []
    sheets = sorted((rules / "resources/xrechnung").glob("*/xsl/XRechnung-*-validation.xsl"))
    for bundled, upstream in BUNDLED.items():
        if sha256(rules / bundled) != sha256(config / upstream):
            drift.append(f"{bundled}: differs from KoSIT configuration {cfg_tag}")
    for sheet in sheets:
        upstream = config / "resources/xrechnung" / sheet.relative_to(rules / "resources/xrechnung")
        if not upstream.exists():
            drift.append(f"{sheet.name}: version {sheet.parent.parent.name} missing in configuration {cfg_tag}")
        elif sha256(sheet) != sha256(upstream):
            drift.append(f"{sheet.name}: differs from KoSIT configuration {cfg_tag}")
    upstream_versions = {p.parent.parent.name for p in (config / "resources/xrechnung").glob("*/xsl/XRechnung-*-validation.xsl")}
    bundled_versions = {p.parent.parent.name for p in sheets}
    if upstream_versions != bundled_versions:
        drift.append(f"XRechnung versions: bundled {sorted(bundled_versions)}, KoSIT {sorted(upstream_versions)}")

    def against(files, label, prefix):
        out = work / ("out-" + label)
        log = run_kosit(java, jar, config, out, files)
        results = validate(cli, files)
        failures = []
        for source in files:
            report = out / f"{source.stem}-report.xml"
            if not report.exists():
                failures.append(f"{prefix}{source.name}: KoSIT produced no report")
                continue
            accepted, reference = kosit_messages(report)
            failures += compare_kosit(prefix + source.name, results[source], accepted, reference)
        if any(f.endswith("KoSIT produced no report") for f in failures):
            print(f"KoSIT ({label}) {log}", file=sys.stderr)
        return failures

    own = sorted(corpus.glob("xrechnung-*.xml"))
    instances = sorted(p for folder in SUITE for p in (suite / folder).rglob("*.xml"))
    if not own or not instances:
        sys.exit("no XRechnung documents found")
    own_failures = against(own, "corpus", "")
    suite_failures = against(instances, "testsuite", "testsuite ")
    return {
        "id": "kosit",
        "name": "KoSIT-Validator",
        "version": jar_tag.removeprefix("v"),
        "details": f"Konfiguration {cfg_tag.removeprefix('v')}, Testsuite {suite_tag.removeprefix('v')}",
        "checks": [
            check("Offizielle KoSIT-Testsuite", len(instances), suite_failures),
            check("XRechnung-Korpus (CII + UBL)", len(own), own_failures),
            check("Regeldateien gleich dem KoSIT-Release", len(BUNDLED) + len(sheets), drift, "files"),
        ],
        "deviations": drift + own_failures + suite_failures,
    }


# ---------------------------------------------------------------- Mustang

MUTATIONS = {
    "wrong-total": (r"(<ram:GrandTotalAmount>)[^<]*", r"\g<1>1.00"),
    "bad-type-code": (r"(<ram:TypeCode>)\d+(</ram:TypeCode>)", r"\g<1>999\2"),
}
RULE_ID = re.compile(r"\[ID ([A-Za-z0-9-]+)\]")


def run_mustang(java, jar, path):
    """(valid, [rule IDs]) of the Mustang verdict for an XML or PDF file."""
    run = subprocess.run([*java, "-jar", str(jar), "--action", "validate", "--no-notices", "--disable-file-logging", "--source", str(path)],
                         capture_output=True, text=True, check=False)
    match = re.search(r"<\?xml.*?</validation>", run.stdout, re.S)
    if not match:
        return None, [run.stderr[-200:] or run.stdout[-200:]]
    tree = ET.fromstring(match.group(0).split("?>", 1)[1])
    ids = sorted(i for e in tree.iter("error") for i in RULE_ID.findall("".join(e.itertext())))
    return tree.findall("summary")[-1].get("status") == "valid", ids


def blocking_rules(result):
    return sorted(i["ruleId"] for i in result["issues"] if i["stage"] == "rules" and i["severity"] in ("error", "fatal"))


def mustang(java, cli, work, cache, offline):
    tag, jar = fetch("ZUGFeRD/mustangproject", lambda n: n.startswith("Mustang-CLI") and n.endswith(".jar"), cache, offline)
    print(f"Mustang {tag}")
    xmls = sorted(corpus.glob("facturx-*.xml"))
    pdfs = sorted(corpus.glob("facturx-*.pdf"))
    if not xmls or not pdfs:
        sys.exit("no facturx-*.xml/.pdf documents found")
    broken = []
    for source in (p for p in xmls if p.stem.endswith(("-standard", "-export"))):
        for name, (pattern, replacement) in MUTATIONS.items():
            text, count = re.subn(pattern, replacement, source.read_text(), count=1)
            if count:
                target = work / f"{source.stem}-{name}.xml"
                target.write_text(text)
                broken.append(target)
    results = validate(cli, [*xmls, *broken, *pdfs])
    jobs = [*xmls, *broken, *pdfs]
    with ThreadPoolExecutor(4) as pool:
        verdicts = dict(zip(jobs, pool.map(lambda p: run_mustang(java, jar, p), jobs)))

    xml_failures, broken_failures, pdf_failures = [], [], []
    for source in xmls:
        valid, ids = verdicts[source]
        lib = results[source]
        if valid is None:
            xml_failures.append(f"{source.name}: Mustang produced no result: {ids}")
        elif valid != accepts(lib) or ids != blocking_rules(lib):
            xml_failures.append(f"{source.name}: Mustang valid={valid} {ids}; library {lib['state']} {blocking_rules(lib)}")
    for source in broken:
        valid, ids = verdicts[source]
        lib = results[source]
        if valid is None:
            broken_failures.append(f"{source.name}: Mustang produced no result: {ids}")
        elif valid or accepts(lib):
            broken_failures.append(f"{source.name}: broken document accepted (Mustang valid={valid}, library {lib['state']})")
        elif ids and ids != blocking_rules(lib):
            broken_failures.append(f"{source.name}: Mustang rules {ids}; library rules {blocking_rules(lib)}")
    for source in pdfs:
        valid, ids = verdicts[source]
        if valid is not True or not accepts(results[source]):
            pdf_failures.append(f"{source.name}: Mustang valid={valid} {ids}; library {results[source]['state']}")
    return {
        "id": "mustang",
        "name": "Mustang",
        "version": tag.removeprefix("core-"),
        "details": "ZUGFeRD / Factur-X, PDF/A-3 per veraPDF",
        "checks": [
            check("Factur-X-Korpus (BASIC, EN 16931, EXTENDED)", len(xmls), xml_failures),
            check("Absichtlich fehlerhafte XML", len(broken), broken_failures),
            check("PDF/A-3 mit eingebetteter XML", len(pdfs), pdf_failures),
        ],
        "deviations": xml_failures + broken_failures + pdf_failures,
    }


# ---------------------------------------------------------------- main

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cache", type=Path, default=DEFAULT_CACHE)
    parser.add_argument("--offline", action="store_true", help="use the newest cached releases")
    parser.add_argument("--only", choices=["kosit", "mustang"])
    parser.add_argument("--out", type=Path, default=root / "parity/results.json")
    args = parser.parse_args()
    args.cache.mkdir(parents=True, exist_ok=True)
    java = java_command()
    work = Path(tempfile.mkdtemp(prefix="parity-"))
    cli = build_cli(work)
    references = []
    try:
        if args.only in (None, "kosit"):
            references.append(kosit(java, cli, work, args.cache, args.offline))
        if args.only in (None, "mustang"):
            references.append(mustang(java, cli, work, args.cache, args.offline))
    except Exception as err:  # network, GitHub API, broken download
        print("could not run:", err, file=sys.stderr)
        return 2

    parity = not any(r["deviations"] for r in references)
    for ref in references:
        for line in ref["deviations"]:
            print(f"[{ref['id']}] {line}")
        for c in ref["checks"]:
            print(f"{ref['name']} {ref['version']}: {c['label']}: {c['matching']}/{c['total']}")
    print("PARITY" if parity else "DEVIATION")
    if args.only is None:
        bundle = subprocess.run([str(cli), "-version"], capture_output=True, text=True, check=True).stdout
        args.out.write_text(json.dumps({
            "checked": date.today().isoformat(),
            "parity": parity,
            "bundle": dict(re.findall(r"^(XRechnung|Factur-X) (\S+)", bundle, re.M)),
            "references": references,
        }, ensure_ascii=False, indent=2) + "\n")
        print("wrote", args.out.relative_to(root) if args.out.is_relative_to(root) else args.out)
    return 0 if parity else 1


if __name__ == "__main__":
    sys.exit(main())
