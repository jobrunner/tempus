# Altitudinal belts (Höhenstufen)

The bioclim feature carries a **climate-derived altitudinal belt** with the
classic names (kollin, submontan, montan, hochmontan, subalpin, alpin, subnival,
nival). It is designed to **confirm or refute** the belt that FHL *Die Käfer
Mitteleuropas*, Ökologieband 3, states for a species: query a find's location +
period, read the belt, compare.

## Why climate, not metres

Altitudinal belts are **not** a fixed function of elevation — the same belt lies
much higher near the equator than toward the poles (Massenerhebung effect,
latitude, continentality). The familiar Hegi/Flora-Helvetica **metre** scale is
calibrated for the Alps and is wrong elsewhere (a Pyrenean or Sierra-Nevada
"alpine" sits at a different elevation than an Alpine one). tempus therefore
derives the belt **thermoclimatically** from the monthly temperature normals —
temperature already integrates altitude, latitude and continentality — so the
belt uses the familiar names but is **valid across all of Europe and globally**.
Precise elevation in metres remains Ortus's job (high-resolution DEM); tempus
owns the *bioclimatic* belt.

Because it rides on the bioclim normals, the belt is **period-appropriate**: a
1954 find gets its era's belt, which can reveal treeline shift under climate
change — something a single present-day DEM cannot do.

## The classic belt (source of truth)

Indicators from the monthly normals: warmest-month mean `Twarm`, warmest-quarter
mean `Twq` (growing-season proxy), annual mean `MAT`, and Holdridge
biotemperature. The decision tree (top-down):

| Belt | Condition |
|---|---|
| nival | `Twarm ≤ 0 °C` |
| subnival | `Twarm ≤ 5 °C` |
| alpin | `Twq ≤ 6.4 °C` (above treeline) |
| subalpin | `Twq ≤ 10 °C` |
| hochmontan | `MAT ≤ 4 °C` |
| montan | `MAT ≤ 7 °C` |
| submontan-kollin | `MAT ≤ 11 °C` |
| planar | else |

The treeline isotherm **6.4 °C** is the growing-season mean of Paulsen & Körner
(2014) — a refinement of the 6.7 °C in Körner & Paulsen (2004). The thresholds
are **calibrated to the Central-European/Alpine convention** so tempus "alpin" ≈
FHL "alpin", then applied globally.

A **`borderline`** flag marks a location within 0.5 °C of a boundary and names
the adjacent belt, so an FHL "alpin" vs. tempus "subalpin (grenznah alpin)" is
read as agreement, not a hard contradiction. The deciding `indicators` are
returned for auditability.

!!! note "Season-proxy caveat"
    `Twq` is a fixed-3-month proxy for Paulsen & Körner's true thermal season
    (days with daily mean > 0.9 °C). The proxy diverges most at high latitudes
    and in the tropics — near the treeline. It is a v1 approximation; if a
    validation site misses, the season definition is the first knob, not the 6.4.

## The Rivas-Martínez thermotype (formal second label)

Alongside the classic belt, a **Rivas-Martínez thermotype** (Worldwide
Bioclimatic Classification System) is emitted. The horizon (thermo/meso/supra/
oro/cryoro) comes from the annual **positive temperature** `Tp = 10 × Σ Tmean⁺`
(boundaries: thermo > 2000, meso 1400–2000, supra 800–1400, oro 380–800, cryoro
≤ 380; Rivas-Martínez 2011 / *Bioclimate of Italy*). The macrobioclimate suffix
comes from the Köppen class. This distinguishes an Alpine "alpin"
(**orotemperate**) from a Sierra-Nevada "alpin" (**oromediterranean**) — the same
classic belt, a different bioclimate.

## Sources

- Paulsen & Körner (2014), *A climate-based model to predict potential treeline
  position around the globe*, Alpine Botany — treeline 6.4 °C ± 0.4.
- Körner & Paulsen (2004), *A world-wide study of high altitude treeline
  temperatures*, J. Biogeography — 6.7 °C ± 0.8.
- Rivas-Martínez, *Worldwide Bioclimatic Classification System*
  (globalbioclimatics.org); *Bioclimate of Italy* (Tp/It horizon tables).
- Holdridge, *Life Zone Ecology* — biotemperature.
- Central-European belt convention: Hegi; Flora Helvetica.
