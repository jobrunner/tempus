# Altitudinal Belts (Höhenstufen) Implementation Plan — FINAL

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (or executing-plans) to implement this plan task-by-task.

**Goal:** Give every location a **climate-derived altitudinal belt with the classic names** (kollin … alpin … nival), valid **everywhere in Europe (incl. Spain/Pyrenees) and globally**, so a user can **confirm or refute** the belt that FHL *Die Käfer Mitteleuropas*, Ökologieband 3 states for a species. The belt is the **source of truth**; a Rivas-Martínez thermotype is added as a formal cross-Europe second label.

**Use case (drives every decision):** FHL Ökologieband 3 labels many weevil species by belt (e.g. "alpin"). The user queries tempus for a find's location + period, gets the climatic belt (e.g. "montan"), and confirms/refutes the FHL statement. For this to work the belt must (a) use the **classic nomenclature FHL uses**, (b) be **calibrated to the Central-European/Alpine convention** so "alpin" means the same thing, and (c) be derived from **climate** so it stays valid outside the Alps.

**Architecture:** Pure function of the 12-month climate normals (temperature integrates elevation, latitude, continentality). Computed **inside the existing bioclim provider**, reusing its 30-year ERA5 fetch and `MonthlyClimate`. **No elevation input, no meter-belt, no region gate, no new dependency.** Period-appropriate (a 1954 find gets its era's belt, revealing treeline shift — something Ortus's single present-day DEM structurally cannot do).

**Tech Stack:** Go 1.25, `internal/domain` (pure math) + `internal/adapters/bioclim` (provider) + `internal/adapters/http/index.html` (frontend). No new deps.

## What changed from the draft (and why)

- **Meter-belt (Hegi/FHL meter table) dropped entirely.** It is region-bound (Alpine-calibrated) and wrong outside the Alps — the opposite of the goal (Europe-wide, incl. Spain). Any bbox/country gate is fuzzy; the climate belt with classic names is the single consistent label. Precise *elevation in metres* remains Ortus's job (DEM).
- **Belt calibrated to the Central-European/Alpine convention** so tempus "alpin" ≈ FHL "alpin", then the same isotherms apply globally.
- **Indicators + borderline flag added** for auditable confirm/refute.

## Global Constraints

- **Climate-only.** Uses only `domain.MonthlyClimate`. No elevation, no meter-belt, no region gate.
- **Classic nomenclature, CE-calibrated.** Belt names: `planar/kollin`, `submontan`, `montan`, `hochmontan`, `subalpin`, `alpin`, `subnival`, `nival`. Thresholds calibrated so Central-Alpine reference sites reproduce the FHL/Hegi belts; validated by tests.
- **Period-appropriate.** Same reference-period `MonthlyClimate` bioclim selected (auto-contemporaneous / `refPeriod`).
- **Bilingual** `{de,en}` for every name.
- **Auditable.** Emit the deciding indicators (warmest-month mean, warmest-quarter mean, MAT, Holdridge biotemperature) and a **borderline** flag + adjacent belt when the deciding indicator is within `beltBorderlineMarginC` (0.5 °C) of a threshold.
- **Citable, one-constant-per-threshold.** Sources: Paulsen & Körner (2014, treeline 6.4 °C ± 0.4; season = days with mean > 0.9 °C; earlier 6.7 °C ± 0.8 in Körner & Paulsen 2004), Köppen (tree limit), Rivas-Martínez (2004, WBCS), Holdridge (biotemperature).

## Scientific definitions

### A. Thermoclimatic belt — classic names, isotherm boundaries (the source of truth)

Indicators from `MonthlyClimate`:
- `Twarm` = mean temp of the warmest month = `max_m Tmean[m]`
- `Twq` = mean temp of the warmest quarter (= `Bioclim.Bio10`; growing-season proxy)
- `MAT` = `Bioclim.Bio1`
- `Tbio` = Holdridge biotemperature = mean over 12 months of `clamp(Tmean[m], 0, 30)`

Decision tree (top-down; treeline ≈ growing-season mean 6.4 °C — Paulsen & Körner 2014):

| # | Condition | Belt (de / en) |
|---|---|---|
| 1 | `Twarm ≤ 0` | nival / nival |
| 2 | `Twarm ≤ 5` | subnival / subnival |
| 3 | `Twq ≤ 6.4` | alpin / alpine (treeless, above treeline) |
| 4 | `Twq ≤ 10` | subalpin / subalpine |
| 5 | `MAT ≤ 4` | hochmontan / high-montane |
| 6 | `MAT ≤ 7` | montan / montane |
| 7 | `MAT ≤ 11` | submontan-kollin / colline |
| 8 | else | planar / lowland |

Constants (calibratable): `beltNivalTwarm=0`, `beltSubnivalTwarm=5`, `beltTreelineTwq=6.4`, `beltSubalpineTwq=10`, `beltHighMontaneMAT=4`, `beltMontaneMAT=7`, `beltCollineMAT=11`, `beltBorderlineMarginC=0.5`.

> **Proxy caveat (documented, not blocking):** `Twq` is a fixed-3-month proxy for Paulsen & Körner's true thermal season (days with mean > 0.9 °C). The proxy diverges most at high latitudes and in the tropics — exactly near the treeline. If a validation site misses, the **season definition** is the first knob, not the 6.4 constant.

### B. Rivas-Martínez thermotype (formal cross-Europe second label)

Coldest month `c = argmin(Tmean)`:
- Thermicity index `It = (MAT + M + m) × 10`, `M = Tmax[c]`, `m = Tmin[c]`.
- Positive annual temperature `Tp = 10 × Σ Tmean[m]` over months with `Tmean[m] > 0`.

Macrobioclimate from the already-computed Köppen main class (A→Tropical, C/D→Temperate/Boreal, E→Polar; Köppen dry-summer `s`→Mediterranean). Output `{code, de, en}` (e.g. `orotemperate`, `oromediterranean`). This complements the classic belt: it distinguishes e.g. an Alpine "alpin" (orotemperate) from a Sierra-Nevada "alpin" (oromediterranean).

> **As built (v1):** the horizon uses a **Tp-only** ladder (thermo > 2000, meso 1400–2000, supra 800–1400, oro 380–800, cryoro ≤ 380; Rivas-Martínez 2011 / *Bioclimate of Italy*). `It` (thermicity index) is computed and available but not used for horizon selection; no per-hemisphere horizon branching and no station-table calibration beyond the unit tests. Refining to the full `Tp`/`It` per-macrobioclimate tables + station validation is deferred.

## Output shape (added to the bioclim feature `properties`)

```json
"altitudinalBelt": {
  "belt": {"de": "subalpin", "en": "subalpine"},
  "basis": "thermoclimatic isotherm, Central-Europe-calibrated",
  "borderline": true,
  "adjacentBelt": {"de": "alpin", "en": "alpine"},
  "indicators": {"warmestMonthC": 9.1, "warmestQuarterC": 8.0, "matC": 1.2, "biotemperatureC": 3.4},
  "thermotype": {"code": "orotemperate", "de": "orotemperat", "en": "orotemperate"},
  "source": "Höhenstufe thermoklimatisch berechnet (Paulsen & Körner 2014; Rivas-Martínez 2004), tempus"
}
```

---

### Task 1: Thermoclimatic belt + indicators + borderline (domain)

**Files:** Create `internal/domain/belt.go`, `internal/domain/belt_test.go`.

**Interfaces:**
- Produces: `type Belt struct { De, En, AdjDe, AdjEn string; Borderline bool; WarmestMonthC, WarmestQuarterC, MATC, BiotemperatureC float64 }`; `func AltitudinalBelt(c MonthlyClimate) Belt`; the threshold constants; `const BeltSource`.

- [ ] Step 1: Failing tests — the 8-row tree with unambiguous synthetic inputs per belt (nival: warmest month −2; alpine: warmest-quarter 5; subalpine: warmest-quarter 8; high-montane: MAT 3; montane: MAT 6; colline: MAT 9; lowland: MAT 15); assert `De`.
- [ ] Step 2: Run → FAIL. Implement `AltitudinalBelt` (compute Twarm, Twq, MAT, Tbio; evaluate tree; set indicators) + constants + `BeltSource`. Run → PASS.
- [ ] Step 3: Failing tests for **borderline**: an input whose deciding indicator is within 0.5 °C of a threshold sets `Borderline=true` and the adjacent belt; one comfortably inside sets `Borderline=false`. Implement. Run → PASS.
- [ ] Step 4: Failing test for `BiotemperatureC` (Holdridge): input with a month at 35 °C and one at −10 °C clamps to 30 and 0; hand-computed mean. Implement clamp. Run → PASS.
- [ ] Step 5: **CE-calibration validation** — synthetic monthly normals approximating known Central-Alpine belt sites (a ~2200 m Central-Alps subalpine/alpine boundary → alpin or subalpin borderline; an ~1000 m montane valley → montan; a lowland ~300 m → colline/planar). Adjust constants until the names match the FHL/Hegi convention. Commit.

### Task 2: Rivas-Martínez thermotype (domain)

**Files:** Create `internal/domain/thermotype.go`, `internal/domain/thermotype_test.go`.

**Interfaces:** `func RivasMartinezThermotype(c MonthlyClimate, latDeg float64) (code, de, en string)`; `func ThermicityIndex(c MonthlyClimate) float64`; `func PositiveTemperature(c MonthlyClimate) float64`.

- [ ] Step 1: Failing tests for `ThermicityIndex` and `PositiveTemperature` vs hand-computed values. Implement. Run → PASS.
- [ ] Step 2: Transcribe the Rivas-Martínez (2004) horizon boundary table (globalbioclimatics.org) as package data; implement `RivasMartinezThermotype` (macrobioclimate via a small helper reusing the Köppen main letter).
- [ ] Step 3: Validation tests — 4–5 stations from "Bioclimate of Italy" (+ one temperate, one alpine) → documented thermotype; calibrate table until PASS. Commit.

### Task 3: Wire belt + thermotype into the bioclim feature (provider)

**Files:** Modify `internal/adapters/bioclim/bioclim.go`; extend `internal/adapters/bioclim/bioclim_test.go`.

- [ ] Step 1: In `buildFeature`, compute `AltitudinalBelt(clim)` and `RivasMartinezThermotype(clim, data.Latitude)`; assemble the `altitudinalBelt` object (belt, basis, borderline/adjacent, indicators, thermotype, source) into `props`.
- [ ] Step 2: Extend the provider test (canned Berlin body) to assert `altitudinalBelt.belt`, `.indicators.matC`, `.thermotype.code`. Run affected tests → PASS. Commit.

### Task 4: Frontend

**Files:** Modify `internal/adapters/http/index.html` (`renderBioclimFeature`).

- [ ] Step 1: Add a prominent "Höhenstufe" block at the top of the bioclim card: belt name (large) + `(grenznah: <adjacent>)` when borderline, the thermotype, and the indicators as small stats. German labels.
- [ ] Step 2: Live-verify: Alpine coordinate → subalpin/alpin (borderline shown); Berlin → planar/kollin; a Spanish Sierra-Nevada coordinate → alpin + oromediterranean thermotype. Screenshot. Commit.

### Task 5: OpenAPI + docs

**Files:** Modify `internal/adapters/http/openapi.yaml` **and** `api/openapi/openapi.yaml` (byte-identical); new `docs/explanation/altitudinal-belts.md` + `mkdocs.yml` nav.

- [ ] Step 1: Extend `BioclimFeatureProperties` with `altitudinalBelt`; `cp` to api copy; `diff -q` identical.
- [ ] Step 2: Write the explanation page: the use case (FHL confirm/refute), the isotherm thresholds + citations (Paulsen & Körner 2014; 6.7 °C 2004 → 6.4 °C 2014; season caveat), thermotype indices, Holdridge biotemperature, and why there is **no meter-belt** (Ortus owns elevation). Add nav. `make docs` builds. Commit.

### Task 6: Ship

- [ ] `make verify` green; `make debt-coverage` (internal/domain ≥ floor); `make docs` green.
- [ ] Branch `feat/altitudinal-belts`, PR, Copilot auto-review loop (fix → re-review), squash-merge, release v0.14.0, verify published image.

## Notes carried from research (docs/superpowers/specs/2026-08-02-altitudinal-belts-research-notes.md)

- Treeline citation: 6.7 °C (Körner & Paulsen 2004) refined to 6.4 °C (Paulsen & Körner 2014); document the season-definition proxy caveat.
- Holdridge honesty: biotemperature emitted as an indicator; the arctic–alpine analogy is documented (a lowland Arctic point reads as its altitudinal analog "alpin/subnival").
- Rivas-Martínez validation ground truth: "Bioclimate of Italy" stations; tables on globalbioclimatics.org.
- Meter-belt/country-gating discussion resolved by **dropping the meter-belt** (goal is Europe-wide classic names, not an Alpine meter table).
