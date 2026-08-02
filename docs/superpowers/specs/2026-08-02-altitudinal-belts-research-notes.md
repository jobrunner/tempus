# Altitudinal Belts — cross-session research notes (input for the plan)

> **Origin:** Written 2026-08-02 by the **ortus** Claude session. The Höhenstufe
> feature was brainstormed for ortus, then deliberately handed to Tempus because a
> scientifically defensible altitudinal belt is a *bioclimatic* classification and
> the climate data lives here. These are corroborating + additive notes for
> `docs/superpowers/plans/2026-08-02-altitudinal-belts.md`. Nothing here blocks the
> plan — treat it as a second pair of eyes.

## TL;DR

Independent literature research reached the **same anchors your plan already
uses** (Körner treeline isotherm, Rivas-Martínez thermotypes, climate-only primary
belts, region-gated + `approximate` CE meter-belt with Ortus-DEM authoritative).
That convergence is a confidence signal. Five additive points below — mostly
citation precision and two genuine design perspectives (Holdridge two-axis naming;
country-gating vs. bounding box).

## 1. Citation precision on the treeline isotherm (Task 1)

Your table uses `Twq ≤ 6.4` citing "Körner & Paulsen 2004". Two refinements:

- **6.4 °C is the 2014 value, not 2004.** Körner & Paulsen (2004) reported the
  treeline seasonal mean at **6.7 °C (± 0.8)**. The **6.4 °C (± 0.4)** figure is the
  *refinement* in **Paulsen & Körner (2014)** ("A climate-based model to predict
  potential treeline position around the globe", *Alpine Botany*), which added
  tropical + Arctic sites. Cite **both**: 6.7 (2004) → 6.4 (2014).
- **The season definition matters.** Paulsen & Körner define the growing season as
  *days with daily mean > 0.9 °C*, and the 6.4 °C is the mean over exactly those
  days (root-zone/ground temperature originally). Your `Twq` = warmest-quarter mean
  (Bio10) is a fixed-3-month proxy. The proxy diverges from the true thermal season
  **most at high latitudes and in the tropics** — i.e. precisely near the treeline
  you're trying to place. Fine as v1 (you flag it as a proxy), but if a validation
  station misses, the season definition is the first knob, not the 6.4 constant.

## 2. Holdridge two-axis honesty — the Arctic case (Task 1, Step 5)

Your decision tree is Holdridge-in-spirit (temperature unifies latitude + altitude),
which is correct. One naming subtlety worth an explicit decision:

Holdridge's system carries **two** axes — a *latitudinal region* (from **sea-level**
biotemperature) **×** an *altitudinal belt* (from **actual** biotemperature). Arctic
sea-level tundra is named via the *polar region*; it is **not** literally called
"alpine/nival". Your Step-5 archetype maps "Arctic → alpine/subnival", which is the
real **arctic–alpine analogy** and defensible — but a user who sees "alpine" at 20 m
elevation on Svalbard may read it as a bug.

Cheap ways to keep it honest, pick one:
- Add a boolean/enum indicator of *cause* — e.g. `causedBy: "latitude" | "altitude"`
  derived by comparing site MAT to a same-latitude sea-level MAT estimate; or
- Emit the **Holdridge biotemperature** (mean of monthly Tmean clamped to [0,30] °C)
  as one more `indicators` value — it's the canonical, citable scalar behind the
  whole thing and makes "why this belt" auditable; or
- Just document it: "belts follow the arctic–alpine analogy; a lowland Arctic point
  reads as its altitudinal analog." (Lowest effort.)

## 3. Region gate — country-gating beats the bounding box (Task 3)

Your CE gate is `lon ∈ [3,20], lat ∈ [43,55]`. In the ortus discussion we landed on
**gating by country ISO** (ortus already resolves the country), using the **FHL
"Die Käfer Mitteleuropas" area** as the authoritative allow-list:
`DE AT CH LI LU NL BE DK PL CZ SK HU` core, `+ SI FR IT` extension.

Why it matters — your bbox both **over- and under-shoots** the Hegi/FHL domain:
- **Drops** Denmark (mostly > 55 °N), northern Germany near the Danish border,
  **eastern Poland** (Warsaw ≈ 21 °E, outside lon 20) and **eastern Hungary**
  (Debrecen ≈ 21.6 °E).
- **Includes** Mediterranean **southern France** (Provence reaches 43 °N) and
  **central/northern Italy** (Po plain, Florence ≈ 43.8 °N) — exactly where the
  Alpine-calibrated meter thresholds are *wrong* (higher/oro-Mediterranean treeline).

Tempus is keyed by lat/lon and may not carry admin polygons like ortus does, so a
country lookup could be a new dependency. Two pragmatic options:
- **If a cheap country lookup is acceptable** (coarse country polygons, or a reverse
  call to ortus's gazetteer), gate by the FHL country list — it matches the scale's
  true validity domain far better than any box.
- **If you keep the bbox**, at least widen lat to ~57 (Denmark) and *document* that
  the box knowingly includes Mediterranean FR/IT where the meter-belt is least
  valid — the `approximate: true` flag already covers you, but say it explicitly.

Either way the primary climate belts are global and unaffected; this only scopes the
secondary Hegi/FHL meter-belt.

## 4. Rivas-Martínez table transcription (Task 2)

The Tp/It horizon boundaries you plan to transcribe are published in
**Rivas-Martínez's Worldwide Bioclimatic Classification System** — his own site
**globalbioclimatics.org** hosts the definitive tables and worked example stations.
For validation stations (Task 2, Step 4) the paper **"Bioclimate of Italy:
application of the worldwide bioclimatic classification system"** gives a clean set
of Mediterranean + temperate stations with their published thermotypes — good
ground truth to lock the table against. The 11 European thermotypes
(thermo/meso/supra/oro/cryoro-temperate + in/thermo/meso/supra/oro/cryoro-
mediterranean) match your plan.

## 5. Why Tempus is the right home (affirmation)

Your **period-appropriate** belt (point 7: a 1954 find gets its era's belt,
revealing treeline shift) is something ortus structurally *cannot* do — ortus is
geometry + a single present-day DEM. That single feature is the strongest argument
that this belongs here, not in ortus. The ortus decision is recorded so it won't be
re-proposed there; ortus's DEM stays the authoritative *elevation* source, Tempus
owns the *bioclimatic* belt.

## Sources

- Körner & Paulsen (2004), *A world-wide study of high altitude treeline
  temperatures*, J. Biogeography — treeline seasonal mean **6.7 °C ± 0.8**.
- Paulsen & Körner (2014), *A climate-based model to predict potential treeline
  position around the globe*, Alpine Botany — refined **6.4 °C ± 0.4**, season =
  days with daily mean > 0.9 °C.
- Rivas-Martínez, *Worldwide Bioclimatic Classification System* — globalbioclimatics.org
  (thermotype Tp/It tables); "Bioclimate of Italy" application paper (validation stations).
- Holdridge, *Life Zone Ecology* — biotemperature; latitudinal region × altitudinal belt.
- Merriam — life zones / the arctic–alpine analogy.
- Central-European belt convention (planar/collin/montan/subalpin/alpin/nival):
  Spektrum "Höhengliederung"; de.wikipedia "Höhenstufe (Ökologie)"; Hegi / Flora Helvetica.
