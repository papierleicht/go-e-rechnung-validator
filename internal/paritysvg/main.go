// Command paritysvg draws the README graphics from parity/results.json:
// parity.svg (one row per check), banner.svg and shields.io endpoint badges.
//
//	go run ./internal/paritysvg -in parity/results.json -out docs/generated
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type check struct {
	Label    string `json:"label"`
	Unit     string `json:"unit"`
	Total    int    `json:"total"`
	Matching int    `json:"matching"`
}

type reference struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Details    string   `json:"details"`
	Checks     []check  `json:"checks"`
	Deviations []string `json:"deviations"`
}

type results struct {
	Checked    string            `json:"checked"`
	Parity     bool              `json:"parity"`
	Bundle     map[string]string `json:"bundle"`
	References []reference       `json:"references"`
}

// Brand tokens of the papierleicht design system.
const (
	paper    = "#faf8f3"
	ink      = "#293f3a"
	sage     = "#397e6b"
	leaf     = "#72b5a0"
	muted    = "#586b63"
	darkBG   = "#1c1e21"
	darkCard = "#26292d"
	darkText = "#f0f1f2"
	darkMute = "#bbc0c6"
	darkMint = "#9dd6c2"
	warnLite = "#9a5b00"
	warnDark = "#edcf96"
	fonts    = `Manrope, -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif`
)

func main() {
	in := flag.String("in", "parity/results.json", "parity results")
	out := flag.String("out", "docs/generated", "output directory")
	flag.Parse()
	data, err := os.ReadFile(*in)
	if err != nil {
		fail(err)
	}
	var r results
	if err := json.Unmarshal(data, &r); err != nil {
		fail(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	files := map[string]string{"parity.svg": paritySVG(r), "banner.svg": bannerSVG(r)}
	for _, ref := range r.References {
		files["badge-"+ref.ID+".json"] = badge(ref)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(*out, name), []byte(content), 0o644); err != nil {
			fail(err)
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "paritysvg:", err)
	os.Exit(1)
}

func esc(s string) string { return html.EscapeString(s) }

func germanDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("02.01.2006")
}

// documents sums the document checks of a reference (rule files excluded).
func documents(ref reference) (matching, total int) {
	for _, c := range ref.Checks {
		if c.Unit != "files" {
			matching += c.Matching
			total += c.Total
		}
	}
	return
}

func badge(ref reference) string {
	m, t := documents(ref)
	color := sage
	if m != t || len(ref.Deviations) > 0 {
		color = "#b26b00"
	}
	b, _ := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"label":         ref.Name + " " + ref.Version,
		"message":       fmt.Sprintf("%d/%d identisch", m, t),
		"color":         strings.TrimPrefix(color, "#"),
	})
	return string(b) + "\n"
}

// icon draws the status mark at (x, y) centre: a check for parity, an
// exclamation mark for a deviation. It always sits next to a text label.
func icon(x, y float64, ok bool) string {
	if ok {
		return fmt.Sprintf(`<circle class="ok" cx="%.1f" cy="%.1f" r="9"/><path class="mark" d="M%.1f %.1f l3.2 3.2 l6 -6.4" fill="none" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>`,
			x, y, x-4.6, y+0.2)
	}
	return fmt.Sprintf(`<circle class="warn" cx="%.1f" cy="%.1f" r="9"/><path class="mark" d="M%.1f %.1f v6 M%.1f %.1f v0.2" fill="none" stroke-width="2.4" stroke-linecap="round"/>`,
		x, y, x, y-5, x, y+4)
}

func paritySVG(r results) string {
	const (
		width   = 880
		pad     = 28
		rowH    = 40
		headH   = 58
		barX    = 392
		barW    = 300
		resultX = 720
	)
	height := 112
	for _, ref := range r.References {
		height += headH + rowH*len(ref.Checks) + 18
	}
	height += 34
	var b strings.Builder
	title := "Parität mit den offiziellen Prüfwerkzeugen"
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-labelledby="t d">`, width, height, width, height)
	fmt.Fprintf(&b, `<title id="t">%s</title><desc id="d">%s</desc>`, esc(title), esc(describe(r)))
	fmt.Fprintf(&b, `<style>
svg{font-family:%s}
.bg{fill:%s}.card{fill:#fff;stroke:#e4e0d6}.text{fill:%s}.muted{fill:%s}.accent{fill:%s}
.track{fill:%s;fill-opacity:.12}.fill{fill:%s}.fill-warn{fill:%s}.ok{fill:%s}.warn{fill:%s}.mark{stroke:#fff}
@media (prefers-color-scheme:dark){.bg{fill:%s}.card{fill:%s;stroke:#3a3f45}.text{fill:%s}.muted{fill:%s}.accent{fill:%s}
.track{fill:%s;fill-opacity:.18}.fill{fill:%s}.fill-warn{fill:%s}.ok{fill:%s}.warn{fill:%s}.mark{stroke:%s}}
</style>`, fonts, paper, ink, muted, sage, ink, sage, warnLite, sage, warnLite,
		darkBG, darkCard, darkText, darkMute, darkMint, darkText, darkMint, warnDark, darkMint, warnDark, darkBG)
	fmt.Fprintf(&b, `<rect class="bg" width="%d" height="%d" rx="16"/>`, width, height)
	fmt.Fprintf(&b, `<text class="text" x="%d" y="50" font-size="22" font-weight="700">%s</text>`, pad, esc(title))
	sub := fmt.Sprintf("Gleiches Ergebnis, gleiche Regel-IDs und Schweregrade · Stand %s · neueste Releases", germanDate(r.Checked))
	fmt.Fprintf(&b, `<text class="muted" x="%d" y="76" font-size="14">%s</text>`, pad, esc(sub))

	y := 98
	for _, ref := range r.References {
		h := headH + rowH*len(ref.Checks)
		fmt.Fprintf(&b, `<rect class="card" x="%d" y="%d" width="%d" height="%d" rx="12"/>`, pad-8, y, width-2*(pad-8), h)
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="17" font-weight="700"><tspan class="text">%s</tspan><tspan class="accent" dx="8">%s</tspan></text>`,
			pad+8, y+28, esc(ref.Name), esc(ref.Version))
		fmt.Fprintf(&b, `<text class="muted" x="%d" y="%d" font-size="13">%s</text>`, pad+8, y+47, esc(ref.Details))
		for n, c := range ref.Checks {
			cy := y + headH + n*rowH + rowH/2
			ok := c.Matching == c.Total
			unit := "Dokumente"
			if c.Unit == "files" {
				unit = "Dateien"
			}
			fmt.Fprintf(&b, `<text class="text" x="%d" y="%d" font-size="14">%s</text>`, pad+8, cy+5, esc(c.Label))
			fmt.Fprintf(&b, `<rect class="track" x="%d" y="%d" width="%d" height="10" rx="5"/>`, barX, cy-5, barW)
			w := 0.0
			if c.Total > 0 {
				w = float64(barW) * float64(c.Matching) / float64(c.Total)
			}
			class := "fill"
			if !ok {
				class = "fill-warn"
			}
			if w > 0 {
				fmt.Fprintf(&b, `<rect class="%s" x="%d" y="%d" width="%.1f" height="10" rx="5"/>`, class, barX, cy-5, w)
			}
			b.WriteString(icon(resultX, float64(cy), ok))
			word := "identisch"
			if !ok {
				word = "abweichend"
			}
			fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="14"><tspan class="text" font-weight="700">%d/%d</tspan><tspan class="muted" dx="6">%s</tspan></text>`,
				resultX+16, cy+5, c.Matching, c.Total, word)
			fmt.Fprintf(&b, `<title>%s: %d von %d %s %s</title>`, esc(c.Label), c.Matching, c.Total, unit, word)
		}
		y += h + 18
	}
	fmt.Fprintf(&b, `<text class="muted" x="%d" y="%d" font-size="12.5">Wöchentlich neu geprüft mit parity/parity.py gegen die jeweils neuesten Releases von KoSIT und Mustang.</text>`, pad, height-22)
	b.WriteString("</svg>\n")
	return b.String()
}

func describe(r results) string {
	var parts []string
	for _, ref := range r.References {
		for _, c := range ref.Checks {
			parts = append(parts, fmt.Sprintf("%s %s, %s: %d von %d identisch", ref.Name, ref.Version, c.Label, c.Matching, c.Total))
		}
	}
	return "Stand " + germanDate(r.Checked) + ". " + strings.Join(parts, "; ") + "."
}

// feather is the papierleicht symbol (design-system/logo-v2), coloured for a
// dark ground: mint and leaf.
const feather = `<g transform="translate(100.37 0) scale(-1 1)"><path fill="` + darkMint + `" d="M21.27 65.73 C22.47 62.72 22.95 59.48 23.42 56.29 C23.88 52.85 24.2 49.62 24.58 46.39 C25.02 42.92 25.33 39.65 26.03 36.47 C27.44 35.73 29.31 35.73 30.59 35.1 C29.75 32.8 28.85 30.51 27.93 28.23 C28.09 26.95 28.65 25.73 29.02 24.49 C29.5 23.12 29.94 22.01 30.4 20.9 C31.01 19.5 31.55 18.34 32.14 17.21 C35.64 17.85 39.02 19.22 42.35 20.49 C41.18 16.53 39.41 12.6 38.19 8.59 C41.69 3.86 47.06 0.11 52.97 -1.06 C59.06 -2.31 65.11 -1.63 70.85 -0.24 C77.08 1.01 82.46 3.75 88 6 C77.45 5.44 68.39 12.41 59.91 17.84 C51.43 24 44.06 31.28 37.65 39.3 C31.09 47.54 25.63 56.37 21.27 65.73 Z"/><path fill="` + leaf + `" d="M24.27 67.12 C28.97 58.35 37.06 52.09 45.01 46.43 C53.19 40.62 61.85 35.73 69.09 29.02 C75.85 21.57 80.94 12.91 88 6 C79.65 11.44 69.68 14.06 61.85 20.51 C53.61 26.5 46.45 33.57 40.22 41.36 C33.84 49.39 28.51 58 24.27 67.12 Z"/><path fill="` + darkMint + `" d="M21.27 65.73 C19.21 70.17 17.4 74.73 15.9 79.39 C14.36 84.21 13.17 88.93 12.37 93.73 C12.21 94.55 12.8 95.43 13.62 95.61 C14.68 95.74 15.51 95.1 15.63 94.27 C16.4 89.63 17.55 85.06 18.98 80.59 C20.5 75.87 22.26 71.44 24.27 67.12 Z"/></g>`

func bannerSVG(r results) string {
	const width, height = 1280, 320
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-labelledby="t d">`, width, height, width, height)
	fmt.Fprintf(&b, `<title id="t">go-e-rechnung-validator</title><desc id="d">XRechnung und ZUGFeRD / Factur-X in purem Go prüfen, ohne Java. %s</desc>`, esc(describe(r)))
	fmt.Fprintf(&b, `<style>svg{font-family:%s}</style>`, fonts)
	fmt.Fprintf(&b, `<clipPath id="r"><rect width="%d" height="%d" rx="18"/></clipPath><g clip-path="url(#r)">`, width, height)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`, width, height, ink)
	fmt.Fprintf(&b, `<svg x="44" y="58" width="78" height="98" viewBox="9.22 -4.88 81.93 102.3">%s</svg>`, feather)
	fmt.Fprintf(&b, `<text x="140" y="112" font-size="44" font-weight="700" fill="%s" letter-spacing="-0.5">go-e-rechnung-validator</text>`, paper)
	fmt.Fprintf(&b, `<text x="142" y="150" font-size="21" fill="%s">XRechnung und ZUGFeRD / Factur-X prüfen –</text>`, darkMint)
	fmt.Fprintf(&b, `<text x="142" y="178" font-size="21" fill="%s">in purem Go, ohne Java, ohne Netz.</text>`, darkMint)

	chips := []string{"XRechnung " + r.Bundle["XRechnung"], "Factur-X " + r.Bundle["Factur-X"], "Library + CLI", "MPL-2.0"}
	x := 142.0
	for _, c := range chips {
		w := 22 + 8.2*float64(len([]rune(c)))
		fmt.Fprintf(&b, `<rect x="%.1f" y="214" width="%.1f" height="32" rx="16" fill="%s" fill-opacity=".10" stroke="%s" stroke-opacity=".35"/>`, x, w, paper, darkMint)
		fmt.Fprintf(&b, `<text x="%.1f" y="235" font-size="14.5" fill="%s" text-anchor="middle">%s</text>`, x+w/2, paper, esc(c))
		x += w + 10
	}

	cardW, cardX := 212, []int{800, 1028}
	for n, ref := range r.References {
		if n >= len(cardX) {
			break
		}
		m, t := documents(ref)
		ok := m == t && len(ref.Deviations) == 0
		cx := cardX[n]
		fmt.Fprintf(&b, `<rect x="%d" y="52" width="%d" height="196" rx="16" fill="%s" fill-opacity=".07" stroke="%s" stroke-opacity=".30"/>`, cx, cardW, paper, darkMint)
		fmt.Fprintf(&b, `<text x="%d" y="86" font-size="15" font-weight="700" fill="%s">%s</text>`, cx+20, paper, esc(ref.Name))
		fmt.Fprintf(&b, `<text x="%d" y="107" font-size="13.5" fill="%s">Version %s</text>`, cx+20, darkMute, esc(ref.Version))
		fmt.Fprintf(&b, `<text x="%d" y="166" font-size="46" font-weight="700" fill="%s" letter-spacing="-1">%d/%d</text>`, cx+20, paper, m, t)
		markColor, word := darkMint, "Dokumente identisch"
		if !ok {
			markColor, word = warnDark, "Abweichung"
		}
		fmt.Fprintf(&b, `<circle cx="%d" cy="208" r="9" fill="%s"/>`, cx+29, markColor)
		if ok {
			fmt.Fprintf(&b, `<path d="M%.1f 208.2 l3.2 3.2 l6 -6.4" fill="none" stroke="%s" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>`, float64(cx+29)-4.6, ink)
		} else {
			fmt.Fprintf(&b, `<path d="M%d 203 v6 M%d 212 v0.2" fill="none" stroke="%s" stroke-width="2.4" stroke-linecap="round"/>`, cx+29, cx+29, ink)
		}
		fmt.Fprintf(&b, `<text x="%d" y="213" font-size="14" fill="%s">%s</text>`, cx+46, darkMint, word)
	}
	fmt.Fprintf(&b, `<text x="%d" y="282" font-size="13" fill="%s" text-anchor="end">Parität mit KoSIT und Mustang · Stand %s</text>`, 1240, darkMute, germanDate(r.Checked))
	fmt.Fprintf(&b, `<rect x="0" y="%d" width="%d" height="10" fill="%s"/></g>`, height-10, width, sage)
	b.WriteString("</svg>\n")
	return b.String()
}
