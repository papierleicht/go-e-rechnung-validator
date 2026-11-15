# Hinweise zu Drittmaterial

Der Code dieses Moduls steht unter der Mozilla Public License 2.0 (`LICENSE`),
Copyright papierleicht. Die folgenden Bestandteile behalten ihre eigenen Lizenzen.
Keine Projektlizenz ersetzt die Bedingungen der Urheber.

## XRechnung-Prüfmaterial (`rules/xrechnung`)

CII-16B- und UBL-2.1-Schemas (Invoice, CreditNote) sowie die kompilierten
Schematron-Regeln von CEN (EN 16931) und KoSIT (XRechnung 3.0.2) aus dem offiziellen
Release `xrechnung-3.0.2-validator-configuration-2026-08-31` der
[KoSIT](https://github.com/itplr-kosit/validator-configuration-xrechnung).
`scenarios.xml` ist unverändert enthalten; die Schweregrade der Standardszenarien
werden daraus gelesen. Die Dateien sind unverändert. Nur beim Laden wird im
Speicher das Muster `match="root()"` zu `match="/"`.

- EN-16931-Regeln: EUPL 1.2, `licenses/EN16931-LICENSE.txt`
- XRechnung-Schematron: Apache-2.0, `licenses/XRechnung-Schematron-LICENSE.txt`
- KoSIT-Konfiguration: Apache-2.0, `licenses/XRechnung-configuration-LICENSE.txt`
  und `licenses/XRechnung-configuration-NOTICE.txt`
- UBL- und UN/CEFACT-Hinweise stehen in den Köpfen der Schemadateien.

## Factur-X/ZUGFeRD-Prüfmaterial (`rules/facturx-*`)

Schemas, kompilierte Regel-Stylesheets und Codelisten der Profile BASIC, EN 16931
und EXTENDED aus dem deutschen Informationspaket ZUGFeRD 2.5.2 / Factur-X 1.09.2
von [FeRD](https://www.ferd-net.de/downloads/veroeffentlichungen) (4. August 2026),
unverändert. Doppelte Codelisten im Profilordner sind weggelassen; die Stylesheets
laden ihre eigenen Kopien. FeRD nennt Apache-2.0 als Lizenz der technischen
Artefakte (`licenses/Apache-2.0.txt`). Die Spezifikation selbst ist nicht enthalten.

Die Hashes aller Regeldateien stehen in `rules.sha256.json`.

## Go-Abhängigkeiten

- [knroy/go-xml](https://github.com/knroy/go-xml) – MIT
- [speedata/pdfdisassembler](https://github.com/speedata/pdfdisassembler) – MIT
- deren Abhängigkeiten laut `go.sum`

## Testkorpus (`testdata/corpus`)

Mit E-Rechnung-Studio erzeugte Beispielrechnungen mit erfundenen Daten
(Muster GmbH, Beispielkunde AG, example.de). Sie stehen unter derselben Lizenz wie
der Code.
