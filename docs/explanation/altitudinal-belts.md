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

Alongside the classic belt, tempus emits the **Rivas-Martínez macrobioclimate
and thermotype** (Worldwide Bioclimatic Classification System, implemented per
its 2008 online edition; the 2011 print edition could not be obtained).

1. **Macrobioclimate** (`altitudinalBelt.macrobioclimate`: tropical,
   mediterranean, temperate, boreal, polar) comes from the system's own key, not
   from Köppen: latitude belts (everything within 23° N/S is tropical), thermal
   limits evaluated as if the site lay at 200 m, continentality `Ic`, and the
   summer-drought indices `Ios2/Ios3/Ios4` that separate Mediterranean from
   temperate climates. The reduction to 200 m needs the site elevation; tempus
   uses the request's `elevation` — the height the normals are corrected to —
   and returns it as `elevationM`. Since the elevation is a required request
   parameter, the reduction is always applied.
2. **Thermotype** (`altitudinalBelt.thermotype`): a prefix — infra-, thermo-,
   meso-, supra-, oro-, cryoro- — on the ladder of that macrobioclimate, or
   **gelid** when no month is above 0 °C. Each macrobioclimate has its own
   ladder: polar has only thermo-, meso- and supra-; boreal has no infra-. The
   thermicity index `It = 10·(T + M + m)` decides (compensated for
   continentality as `Itc` outside 23° N/S); the positive temperature
   `Tp = 10 × Σ Tmean⁺` takes over when It/Itc < 120, Ic ≥ 21, and always for
   boreal and polar. `index` and `value` say which one decided.

This distinguishes an Alpine "alpin" (**orotemperate** or **cryorotemperate**)
from a Sierra-Nevada "alpin" (**oromediterranean**) — the same classic belt, a
different bioclimate. Every criterion, its source, and the interpretations
where the source is silent are in the
[Rivas-Martínez reference](../research/rivas-martinez-reference.md).

## Sources

- Paulsen & Körner (2014), *A climate-based model to predict potential treeline
  position around the globe*, Alpine Botany — treeline 6.4 °C ± 0.4.
- Körner & Paulsen (2004), *A world-wide study of high altitude treeline
  temperatures*, J. Biogeography — 6.7 °C ± 0.8.
- Rivas-Martínez, *Global Bioclimatics (Clasificación Bioclimática de la
  Tierra)*, versión 01-12-2008 (globalbioclimatics.org, via the Internet
  Archive) — key 5.1, Fig. 7, Fig. 19.
- Pesaresi, Galdenzi, Biondi & Casavecchia (2014), *Bioclimate of Italy:
  application of the worldwide bioclimatic classification system*, J. Maps
  10(4) — Tables 2 and 7 after Rivas-Martínez et al. (2011).
- Holdridge, *Life Zone Ecology* — biotemperature.
- Central-European belt convention: Hegi; Flora Helvetica.
