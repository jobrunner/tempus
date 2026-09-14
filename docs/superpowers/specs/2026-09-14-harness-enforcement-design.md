# Harness-Erzwingung und Architektur-Drift-Schutz — Design

**Datum:** 2026-09-14
**Status:** Entwurf zur Review
**Betrifft:** `claude-skills/skills/new-go-service` (Submodul) und `tempus`

## Kontext und Ziel

tempus ist als einziges Projekt der Familie auf Go 1.25 stehengeblieben,
während expertus (1.26.8), hostus, ortus und situs (je 1.26.0) angehoben
wurden. Die Ursachenanalyse ergab keinen technischen Blocker, sondern eine
fehlende Maschinerie:

- tempus hat **kein `.github/dependabot.yml`**. Die Schwesterprojekte wurden
  durch Dependabot-PRs angehoben (`chore(deps): bump …`); in tempus wurde
  `go.mod` seit dem Initial-Commit nur dreimal angefasst, jedes Mal durch ein
  Feature, nie durch einen Dependency-Bump.
- tempus hat **kein zizmor / keine Actions-Security-Prüfung**, obwohl der
  Skill `new-go-service` sie in seiner Workflow-Tabelle führt.
- Die Go-Version ist in den tempus-Workflows **elfmal gepinnt** und zwar in zwei
  Formen: zehnmal literal als `go-version: '1.25'` (ci.yml siebenmal, dazu
  security.yml, openapi.yml, release-please.yml) und einmal indirekt in
  codecharta.yml als `GO_VERSION: '1.25'` plus `go-version: ${{ env.GO_VERSION }}`.
  Selbst ein `go.mod`-Bump hätte die CI nicht mitgezogen. Die indirekte Form ist
  die gefährlichere: sie liest sich wie Konfiguration und wurde von der ersten
  Fassung der Prüfung übersehen.

Das ist kein tempus-Einzelfall, sondern ein Skill-Defekt: `reference/ci-and-release.md`
beschreibt die Workflows als *„a representative set"* — eine Formulierung, die
sich als Menü liest. Genau unter dieser Formulierung ist beim Scaffolding von
tempus die Hälfte weggefallen, ohne dass irgendetwas rot wurde.

Parallel dazu ist die Architektur-Absicherung schwächer, als sie aussieht. Sie
besteht heute ausschließlich aus depguard — und depguard ist eine **Denylist**.

**Ziel:** Beide Lücken strukturell schließen. Fehlende Harness-Bestandteile und
Architektur-Drift sollen nicht mehr durch Aufmerksamkeit verhindert werden,
sondern durch ein Gate, das rot wird.

## Befund 1 — Warum depguard als Denylist strukturell unzureichend ist

`.golangci.yml` definiert pro bekanntem Pfad eine `deny:`-Liste. Daraus folgen
drei Löcher:

1. **Unbekannte Pakete sind ungeschützt.** Ein neues `internal/services/`
   matcht keine `files:`-Regel und darf damit alles importieren. Die Lücke ist
   unsichtbar, weil nichts fehlschlägt.
2. **Externe Imports sind nicht abgedeckt.** `internal/domain` dürfte heute
   `github.com/gorilla/mux`, `net/http` oder viper importieren. Die Deny-Liste
   nennt nur die vier internen Nachbarn.
3. **Jede neue Schicht muss von Hand in vier Deny-Listen nachgetragen werden.**
   Wird das vergessen, entsteht dieselbe stille Lücke wie in (1).

Eine belastbare Fitnessfunktion muss **deny-by-default** arbeiten: jedes Paket
wird einer Schicht zugeordnet, ein nicht zuordenbares Paket ist ein Fehler, und
nur explizit erlaubte Kanten sind zulässig.

## Befund 2 — Der Ist-Zustand des tempus-Import-Graphen

Erhoben über `go list` (37 interne Kanten). Der Graph ist sauber hexagonal;
die Kernregeln (`domain→∅`, `ports→domain`, `application→domain,ports`,
`app→*`) werden ausnahmslos eingehalten. Es gibt genau **zwei** Kanten, die die
dokumentierte Regel `adapters → domain, ports` verletzen:

```
internal/adapters/metrics   -> internal/config
internal/adapters/telemetry -> internal/config
```

Beide sind Setup-Code (OTel-Meter- bzw. Tracer-Konfiguration). depguard bemerkt
sie nicht, weil `internal/config` in keiner Deny-Liste steht.

Das ist der Grund für die Ratchet-Konstruktion statt eines harten Sofort-Bruchs:
Die zwei Kanten werden als deklarierte Ausnahme eingefroren und dürfen nur
verschwinden, nie wachsen.

## Befund 3 — tempus gegen das Manifest, Ist-Stand

Erhoben am 2026-09-14. Das geplante Gate hätte heute folgende Treffer:

| Artefakt | Stand |
|---|---|
| `.github/dependabot.yml` | **fehlt** |
| `.github/zizmor.yml`, `actions-security.yml` | **fehlt** |
| `dependabot-auto-merge.yml` | **fehlt** |
| `LICENSE` | **fehlt** — s.u. |
| `.mutation-thresholds` | **fehlt** — Schwellen inline dupliziert |
| `scripts/mutation-gate.sh` | **fehlt** — s.u. |
| Workflows ohne gepinnte Go-Version | **verletzt**, 11 Fundstellen (10 literal, 1 via `GO_VERSION`) |
| `.commitlintrc.yml`, release-please-Konfig, `.goreleaser.yml` | vorhanden |
| `.debt-budget`, `.coverage-floors`, `.codecharta-ratchet.json` + Skripte | vorhanden |
| `.gremlins.yaml` | vorhanden |

Zwei Punkte verdienen Erläuterung.

**`LICENSE` fehlt.** `gh repo view` meldet `licenseInfo: null`. Der Skill warnt
an dieser Stelle bereits, drei Schwesterservices seien ohne Lizenzdatei in
Produktion gegangen — tempus ist der vierte. Für jeden, der die Bedingungen
prüft, ist das der Unterschied zwischen einer offenen Lizenz und „alle Rechte
vorbehalten". Anders als bei den Schwestern behauptet tempus allerdings auch
nirgends eine Lizenz: weder README noch OpenAPI führen einen `license`-Eintrag
für den Dienst selbst (die `license`-Blöcke in der OpenAPI beschreiben die
*Datenquellen*, nicht tempus). Entschieden am 2026-09-14: **MIT für den
Go-Code**; die Abgrenzung zu Algorithmen und Daten steht unter
„Entscheidungen und Alternativen".

**Das Mutation-Gate ist dupliziert statt geteilt.** Die Schwellen stehen
inline an *zwei* Stellen — im `mutation`-Target des Makefiles und noch einmal
in `.github/workflows/mutation.yml`. Beide Listen sind derzeit inhaltlich
identisch (`internal/domain` 90/95, `internal/application` 77/94), können aber
jederzeit auseinanderlaufen; genau dagegen führt der Skill die eine gemeinsame
Einsprungstelle `scripts/mutation-gate.sh` ein. Immerhin werden die Pakete
explizit aufgezählt, die `...`-Wildcard-Falle liegt also nicht vor.

Nebenbefund ohne eigenen Handlungsbedarf in diesem Entwurf: `mutation.yml` und
das Makefile-Target tragen beide den Kommentar *„gremlins panics on macOS"*.
Der Skill weist diese Behauptung als gemessen falsch aus (darwin/arm64,
v0.6.0). Der Kommentar wird in PR 2 mitkorrigiert, da er Entwickler davon
abhält, das Gate lokal zu fahren.

## Befund 4 — Die `on: release`-Regel des Skills ist zu absolut

Der Skill verbietet `on: release`-Workflows pauschal: sie feuerten bei einem
release-please-Release nie, weil GitHubs Rekursionssperre greift. Am realen
tempus widerlegt: `.github/workflows/docker-release.yml` ist `on: release` und
**funktioniert** — alle Läufe erfolgreich bis v0.24.1.

Der Grund: die Sperre gilt für den Default-`GITHUB_TOKEN`. tempus' release-please
läuft unter einem **GitHub-App-Token** (`actions/create-github-app-token`), und
damit lösen die erzeugten Events sehr wohl weitere Workflows aus.

Für dieses Vorhaben folgt daraus zweierlei. Erstens hätte die ursprünglich
geplante `no_on_release`-Prüfung einen funktionierenden Workflow rot gemacht —
sie gilt jetzt nur im Default-Token-Fall. Zweitens ist die Skill-Referenz
entsprechend qualifiziert worden.

Nebenwirkung, die daran hängt: der **Release-PR bekommt in tempus sehr wohl
CI-Checks**, anders als der Skill für den Default-Token-Fall beschreibt. Die
lokale Vorab-Verifikation in Phase E bleibt trotzdem sinnvoll, ist aber kein
Ersatz für fehlende Checks, sondern eine zusätzliche Kontrolle.

## Entwurf

### Teil 1 — Harness-Manifest und Gate

Neu: `scripts/harness-check.sh` (Template im Skill) plus ein CI-Job. Das Gate
prüft **Existenz und Verdrahtung**. Existenz allein genügt nicht: eine
`mutation.yml`, die `gremlins unleash ./internal/...` aufruft, ist vorhanden und
misst nachweislich nichts (die `...`-Wildcard wird nicht expandiert — bereits im
Skill dokumentiert).

Das Manifest. Jede Zeile ist ein harter Fehler bei Fehlen:

| Gruppe | Artefakte | Verdrahtungsprüfung |
|---|---|---|
| Supply chain | `.github/dependabot.yml` | führt `gomod` **und** `github-actions`; `docker` wenn ein `Dockerfile` existiert; `gitsubmodule` wenn `.gitmodules` existiert |
| | `.github/workflows/dependabot-auto-merge.yml` | — |
| Actions-Security | `.github/zizmor.yml`, `.github/workflows/actions-security.yml` | Workflow übergibt `--config .github/zizmor.yml` |
| Vulns | govulncheck in einem Workflow | — |
| Mutation | `.gremlins.yaml`, `.mutation-thresholds`, `scripts/mutation-gate.sh`, `.github/workflows/mutation.yml` | Workflow ruft `mutation-gate.sh`; **kein** `unleash` mit `...`-Wildcard |
| Komplexität | `.codecharta-ratchet.json`, `scripts/codecharta-ratchet.py`, `.github/workflows/codecharta.yml` | alle drei Gates aktiv (Datei-Summe, Pro-Funktion, Hotspot) |
| Debt | `.debt-budget`, `scripts/debt-guard.sh` | in `make verify` verdrahtet |
| Coverage | `.coverage-floors`, `scripts/coverage-gate.sh` | in `make debt` verdrahtet |
| Commits | `.commitlintrc.yml`, `.github/workflows/commitlint.yml` | — |
| Release | `release-please-config.json`, `.release-please-manifest.json`, `.goreleaser.yml`, `release-please.yml` | `on: release`-Workflow nur erlaubt, wenn release-please **nicht** den Default-Token nutzt (s.u.) |
| Legal | `LICENSE` | — |
| Architektur | depguard-Regeln, `arch_test.go`, `.arch-baseline` | s. Teil 2 |
| OpenAPI | zwei Spec-Kopien | byte-identisch |
| Toolchain-Hygiene | alle Workflows | **kein hartkodiertes `go-version:`** — ausschließlich `go-version-file: go.mod` |

Die letzte Zeile ist die unmittelbare Lehre aus dem tempus-Befund.

**Ausstiegsluke als Ratchet.** `.harness-waivers` nimmt eine Zeile pro bewusst
ausgelassenem Punkt auf, mit Begründung. Das Gate zählt die Waiver und behandelt
die Zahl wie `.debt-budget`: sie darf nur sinken. Damit bleibt „dieses Projekt
braucht kein Docker-Ecosystem" möglich, aber sichtbar, begründet und nie
stillschweigend.

### Teil 2 — Architektur-Gate, drei Schichten

**(a) depguard** bleibt als schneller Lint-Vorfilter und wird um externe Pakete
ergänzt: `internal/domain` darf weder `net/http` noch `gorilla/mux` noch viper
importieren.

**(b) `internal/arch/arch_test.go`** — die eigentliche Fitnessfunktion,
deny-by-default. Sie liest den Graphen über `go list -json ./...` per `os/exec`.
Bewusst **nicht** über `golang.org/x/tools/go/packages`: das wäre eine neue
go.mod-Abhängigkeit für eine Information, die das Toolchain-CLI bereits liefert.

Drei Assertions:

1. **Vollständigkeit** — jedes `internal/...`-Paket wird per Pfadpräfix genau
   einer Schicht zugeordnet. Ein nicht zuordenbares Paket ist ein Fehler. Das
   schließt Loch (1) aus Befund 1.
2. **Kanten** — nur explizit erlaubte Schichtübergänge:
   `domain→∅`, `ports→domain`, `application→domain,ports`,
   `adapters→domain,ports`, `app,cmd→*`, `config→domain`.
   Alles andere ist ein Fehler, sofern es nicht in `.arch-baseline` steht.
3. **Domain-Reinheit** — `internal/domain` importiert ausschließlich stdlib.
   Externe Imports nur über eine benannte Allowlist im Test. Das schließt
   Loch (2).

Testdateien (`_test.go`) bleiben ausgenommen — eine Testdatei ist eine
Test-Kompositionswurzel und darf über Schichten hinweg verdrahten. Diese
Ausnahme entspricht der bestehenden depguard-Konvention.

**(c) `.arch-baseline`** — der Ratchet. Startwert für tempus:

```
# Deklarierte Architektur-Ausnahmen. RATCHET: darf nur schrumpfen.
# Jede Zeile: <paket> -> <import>  # begruendung
internal/adapters/metrics   -> internal/config  # OTel-Meter-Konfig; gehoert hinter einen Port
internal/adapters/telemetry -> internal/config  # dito, Tracer-Setup
2
```

Eine neue Kante ohne Baseline-Eintrag macht den Test rot. Die Baseline-Zahl zu
erhöhen erfordert einen bewussten, reviewbaren Commit. Verschwindet eine
Ausnahme, meldet das Gate „senke die Baseline auf 1", analog zu `debt-guard.sh`.

### Teil 3 — Skill-Änderungen (`new-go-service`)

Im Submodul `claude-skills`, eigener Branch und PR, da tempus nur den Pointer
hält.

- **`SKILL.md`** — neuer, prominenter Abschnitt *„Das Manifest — nicht
  verhandelbar"* mit der Tabelle aus Teil 1, plus eine Red-Flags-Tabelle im
  Hausstil (Muster: „Ist doch nur ein kleiner Service" → das Manifest skaliert
  nicht mit der Projektgröße). Step 9 wird von „füge die Workflows hinzu" auf
  die vollständige Pflichtliste umgestellt. Neuer **Step 10a**: `make harness`
  muss grün sein, bevor irgendetwas als fertig gilt.
- **Neue Templates** — `harness-check.sh`, `harness.yml`, `arch_test.go.tmpl`,
  `arch-baseline`, `dependabot.yml`, `zizmor.yml`, `actions-security.yml`,
  `dependabot-auto-merge.yml`, `vuln-scan.yml`.
- **`reference/architecture.md`** — neuer Abschnitt mit Befund 1 (die drei
  Löcher) und der Allowlist/Ratchet-Konstruktion als Gegenmittel.
- **`reference/ci-and-release.md`** — die Workflow-Tabelle bekommt eine Spalte
  **Pflicht/Optional**; die Formulierung *„a representative set"* entfällt.
  Dazu ein Absatz Toolchain-Hygiene: `go-version-file: go.mod`, nie hartkodiert,
  mit dem tempus-Fall als Beleg.
- **`reference/ratchets-and-harnesses.md`** — Ratchet 5
  (Architektur-Baseline) und die Fitnessfunktion „Harness-Vollständigkeit".

### Teil 4 — tempus-Umsetzung in drei PRs

| PR | Inhalt | Risiko |
|---|---|---|
| **1 — Supply chain** | diese Spec, `.github/dependabot.yml` (gomod, github-actions, docker, gitsubmodule), `.github/zizmor.yml`, `actions-security.yml`, `dependabot-auto-merge.yml` | niedrig, reine Additionen |
| **2 — Arch-Gate + Harness** | `internal/arch/arch_test.go`, `.arch-baseline`, depguard-Verschärfung, `scripts/harness-check.sh`, `.harness-waivers`, `make harness`, CI-Job; dazu die Manifest-Lücken aus Befund 3: `scripts/mutation-gate.sh` + `.mutation-thresholds` (Schwellen aus Makefile und Workflow dorthin zusammenführen), `LICENSE`, macOS-Kommentar korrigieren | mittel — die depguard-Verschärfung kann zunächst rot werden |
| **3 — Go-Bump** | `go 1.26.0` / `toolchain go1.26.6`, `Dockerfile` auf `golang:1.26.x-alpine` mit neuem SHA-Pin, README Zeile 96, alle 11 Go-Pins → `go-version-file: go.mod` | mittel — Toolchain-Bump, einzeln revertierbar |

Reihenfolge ist bindend: PR 2 vor PR 3, damit das Harness-Gate den Bump bereits
mitprüft — insbesondere die Toolchain-Hygiene-Regel, die die acht hartkodierten
Versionen überhaupt erst als Fehler sichtbar macht.

## Entscheidungen und Alternativen

**Lizenz: MIT für den Go-Code**, Rechteinhaber Jo Brunner. Entschieden am
2026-09-14. `LICENSE` wird in PR 2 angelegt.

Die Lizenzlage hat drei Ebenen, die nicht vermischt werden dürfen:

| Ebene | Regelung | Ort |
|---|---|---|
| Der Dienst (Go-Code) | MIT | `LICENSE`, neu |
| Die Algorithmen | Urheber-Attribution, keine Lizenz | bereits zur Laufzeit, s.u. |
| Die Daten | Lizenz der jeweiligen Quelle | bereits zur Laufzeit, s.u. |

Die unteren beiden Ebenen sind in tempus **bereits gelöst** und werden von
diesem Entwurf nicht angefasst: `domain.License` (`Name`, `URL`, `Attribution`)
hängt an jedem Feature, ein Feature ohne vollständige Lizenz wird an der
Port-Grenze abgewiesen. Die Algorithmen-Herkunft steht dort konkret drin —
Magnus-Tetens mit Sonntag-1990-Koeffizienten für den Taupunkt, die
WorldClim-Definitionen und Köppen-Geiger für die BIO-Variablen, dazu ERA5 über
Copernicus/ECMWF und Open-Meteo für die Daten.

`LICENSE` bleibt deshalb ein **unveränderter MIT-Text ohne Zusatzklauseln**.
Ein Ausnahme-Absatz für die Algorithmen würde MIT verwässern und dabei nichts
gewinnen: Formeln und Verfahren als solche sind ohnehin nicht Gegenstand des
Urheberrechts — geschuldet ist wissenschaftliche Zuschreibung, und genau die
leistet der `license`-Block pro Feature bereits verbindlicher, als eine
Textdatei es könnte. Stattdessen bekommt der README einen kurzen Abschnitt
*Lizenz*, der die drei Ebenen benennt und auf den Laufzeit-Mechanismus
verweist, damit niemand MIT auf Daten oder Fachliteratur bezieht.

**Go-Zielversion `1.26.0` mit `toolchain go1.26.6`**, nicht die neueste
Patch-Version. Begründung: Gleichstand mit ortus, situs und hostus wiegt
schwerer als ein paar Patch-Level, weil die Familie gemeinsam gewartet wird.

**`go list` statt `x/tools/go/packages`** im Arch-Test — keine neue
Abhängigkeit für eine Information, die die Toolchain ohnehin liefert.

**Ratchet statt Sofort-Bruch** bei den zwei `adapters→config`-Kanten. Ein
harter Bruch hätte PR 2 mit einer Refaktorierung (Config hinter einen Port)
vermengt, die eigenen Entwurf und eigene Tests verdient.

**Waiver-Zähler statt Boolean-Opt-out** beim Harness-Gate. Ein `skip: true`
wäre nach einmaligem Setzen unsichtbar; ein Zähler, der nur sinken darf, hält
die Auslassung im Blick.

## Nicht in diesem Entwurf

- Die Refaktorierung von `adapters/metrics` und `adapters/telemetry`, sodass die
  Config hinter einen Port wandert. Eigenes Vorhaben; die Baseline hält den
  Zustand bis dahin fest.
- Nachziehen des Harness in expertus, hostus, situs und triplepack. triplepack
  steht auf Go 1.24.4 und ist damit der nächste Kandidat.
- Ein required-status-checks-Ruleset für die neuen Jobs. Erst sinnvoll, wenn die
  Gates nachweislich stabil grün laufen.
