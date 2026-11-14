# Rivas-Martínez macrobioclimate & thermotype reference

> **Status.** This is the sourced basis of tempus's Rivas-Martínez
> classification (`internal/domain/macrobioclimate*.go`,
> `summer_drought.go`, `rm_indices.go`, `thermotype.go`). It resolves the
> deviations **D8–D12** of the [Köppen-Geiger & Rivas-Martínez dossier](koppen-reference.md).
> The reference stations and every threshold below are pinned by
> `internal/domain/rivas_martinez_reference_test.go`,
> `macrobioclimate_test.go` and `thermotype_test.go`.

Date: 2026-11-14.

## Which edition, and why

The classification is implemented **per the 2008 online edition** of the
Worldwide Bioclimatic Classification System (WBCS08). The 2011 print edition
(Rivas-Martínez, Rivas-Sáenz & Penas 2011, *Global Geobotany* 1:1–634, cited
as RM2011) could **not** be obtained. One part of RM2011 is used through a
secondary source: Pesaresi et al. 2014 (P14) tabulate RM2011's summer-drought
compensation table and the Mediterranean/temperate thermotype thresholds, and
both agree with WBCS08 where WBCS08 has values.

The previous code comment cited "Rivas-Martínez 2011; boundaries as tabulated
in *Bioclimate of Italy*" (D12). That citation was not verifiable at the time.
It is now partly verified: P14 is the *Bioclimate of Italy* paper, and its
Table 7 does give the Mediterranean and temperate thermotype bounds "according
to Rivas-Martínez et al. (2011)".

## Sources actually fetched

| Key | Document | URL used | Used for |
|---|---|---|---|
| **WBCS08** | Rivas-Martínez, *Global Bioclimatics (Clasificación Bioclimática de la Tierra)*, versión 01-12-2008 | globalbioclimatics.org is offline (HTTP 404 / no DNS). Wayback raw snapshots, 2013: `http://web.archive.org/web/2013id_/http://www.globalbioclimatics.org:80/book/bioc/global_bioclimatics-2008_02.htm` (ch. 2), `…_03.htm` (ch. 3), `…_05.htm` (ch. 5). Re-fetched the last 2021 snapshots (`web.archive.org/web/20210619223950id_/…_02.htm`, `20210619213837id_/…_03.htm`, `20210619214051id_/…_05.htm`): text identical apart from the page header. | ch. 5.1 key and footnote (1); ch. 2a parameter definitions; ch. 2c summer indices; ch. 2d and Fig. 7 (Itc); ch. 3d and Fig. 19 (thermotypes) |
| **P14** | Pesaresi, Galdenzi, Biondi & Casavecchia 2014, *Bioclimate of Italy: application of the worldwide bioclimatic classification system*, J. Maps 10(4):538–553, doi:10.1080/17445647.2014.891472 | tandfonline returns 403 to scripts. Wayback full-text snapshot: `http://web.archive.org/web/20250729035625id_/https://www.tandfonline.com/doi/full/10.1080/17445647.2014.891472` (tables embedded as JSON in the page) | Table 1 (index definitions), **Table 2** (Ios compensation), **Table 7** (thermotype thresholds), all "according to Rivas-Martínez et al. (2011)" |
| P17 | Pesaresi, Biondi & Casavecchia 2017, *Bioclimates of Italy*, J. Maps 13(2):955–960, doi:10.1080/17445647.2017.1413017 | Wayback: `http://web.archive.org/web/20220725193143id_/https://www.tandfonline.com/doi/full/10.1080/17445647.2017.1413017` | Checked; adds no key or threshold beyond P14 |
| OM | Open-Meteo Historical Weather API docs | https://open-meteo.com/en/docs/historical-weather-api | Meaning of the response field `elevation` |
| WMO | WMO Climatological Standard Normals 1991–2020, NCEI Accession 0253808, doi:10.25921/800j-vn07 | as in the Köppen dossier | Reference station normals and station elevations |

**Not obtainable (tried in this session):** RM2011 itself. ResearchGate
(publication 234015449) and tandfonline return 403; the Wayback CDX index of
`globalbioclimatics.org/book/*` lists only the 2004–2008 chapters and PDFs, no
2011 text; Unpaywall/OpenAlex list no open copy of the book. The figshare
record of P14 (942390) returned 403.

## The macrobioclimate key (WBCS08 ch. 5.1)

Verbatim from "5.1. Clave para los macrobioclimas" (Spanish), with the tempus
reading. Code: `macrobioclimateFrom`, `subtropicalKey`, `extratropicalKey`.

| Step | Source text | tempus |
|---|---|---|
| 1 | "Territorios intertropicales entre los paralelos 23º N y S" → Tropical | `abs(lat) < 23` → tropical |
| 2 | "Territorios entre los paralelos 23° y 35° N y S" → 3; "al N y S del paralelo 35°" → 11 | `23 ≤ abs(lat) ≤ 35` → step 3, else step 11 |
| 3 | "Eurasia: 26 a 35° N, longitud 70° E a 120° E, altitud >= 2.000 m" → 10 | `26 ≤ lat ≤ 35`, `70 ≤ lon ≤ 120`, elevation `≥ 2000` m (known elevation only) → step 10 |
| 4 | "al menos dos de … T >= 25°, … m >= 10°, … It, Itc >= 580" → Tropical | two of `T ≥ 25`, `m ≥ 10`, `Itc ≥ 580` (unreduced; the step has no altitude note) |
| 5 | "En territorios de altitud inferior a 200 m (1). Al menos dos de … T > 21°, … M > 18°, … It, Itc >= 470" → 6, else 8 | two of `T₂₀₀ > 21`, `M₂₀₀ > 18`, `Itc₂₀₀ ≥ 470` |
| 6 | "Pss > Psw" → Tropical | strict `>` |
| 7 | "Ios2 > 2 o … Iosc > 2" → Tropical, else 9 | no Mediterranean summer drought (see below) → tropical |
| 8, 10, 14 | "Verano sin déficit hídrico; … Ios2 > 2 o … Iosc > 2 [Iosc4 in 14]" → Templado, else Mediterráneo | `mediterraneanSummer` false → temperate |
| 9 | "Máxima precipitación durante el cuatrimestre más cálido del año, Pcm3 < Pcm1 > Pcm2" → Tropical, else Mediterráneo | `Pcm1 > Pcm3 && Pcm1 > Pcm2` |
| 11 | "latitud Norte > 72º o … Sur > 56º" → Polar; else at < 200 m "Tp < 380" → Polar, "Tp > 380" → 12 | `lat > 72 || lat < −56`; `Tp₂₀₀ < 380` → polar; `Tp₂₀₀ = 380` (undefined in the key) → step 12 |
| 12 | "Ic < 11, … Tps <= 290, y … Tmax <= 10º" → Boreal | as written; Tps and Tmax unreduced (no rate given) |
| 13 | "Ic < 21 , T < 5.3º y Tp < 720; … Ic = 21-28 , T < 4.8º y Tp < 740; … Ic > 28 , T < 3.8º y Tp < 800" → Boreal | `Ic < 21`; `21 ≤ Ic ≤ 28`; `Ic > 28`; T and Tp at 200 m |

**Footnote (1)**, verbatim: "Si la localidad se halla a más de 200 m de
altitud, hay que calcular teóricamente los valores de temperatura a tal
altitud, incrementando T en 0.6º, M en 0.5º, e It o Itc en 13 unidades, cada
100 m que supere dicha altitud; si está situada al norte del paralelo 48º N o
al sur del 51º S, hay que calcular teóricamente los valores de la temperatura
media anual y de la temperatura positiva anual Tp, incrementando T en 0.4º y Tp
en 12 unidades por cada 100 m que exceda la altitud indicada. Cuando Ic >= 21
(continental) o cuando It o Itc < 120 el termotipo se calcula en función de la
temperatura positiva anual, y los valores teóricos de Tp a 200 m incrementando
55 unidades cada 100 m que exceda dicha altitud."

tempus (`hundredsAbove200m`, `reducedTTp`): `e = (elevation − 200)/100` when
the elevation is known and above 200 m, else 0. Step 5 adds `0.6·e` to T,
`0.5·e` to M and `13·e` to Itc. Steps 11 and 13 use `T + 0.4·e`, `Tp + 12·e`
north of 48° N / south of 51° S (strictly beyond), and `T + 0.6·e`,
`Tp + 55·e` elsewhere.

**Interpretation (marked in code):** the footnote gives no Tp rate for
latitudes below 48° N / 51° S except the 55 units of its last sentence, which
is stated for the case where the thermotype is computed from Tp. tempus uses
55 for every Tp reduction there. The reference stations that depend on it
(Säntis, Zugspitze) are far from every threshold under either rate (Säntis
Tp₂₀₀ = 1500 with 55, 510 with 12; both > 380; T₂₀₀ ≥ 8.4 °C ≫ 5.3).

**Interpretation:** the thermotype is computed from the site's own values;
the reduction to 200 m is used only inside the key. Fig. 19 bands are meant to
change with altitude (Säntis is cryorotemperate, not a lowland thermotype).

### Index definitions (WBCS08 ch. 2a; P14 Table 1)

| Index | Definition used | Source |
|---|---|---|
| T | annual mean of the monthly means | "temperatura media anual" |
| Ic | `Tmax − Tmin`, warmest minus coldest monthly mean | "Ic : índice de continentalidad simple … (Tmax-Tmin …)" |
| It | `(T + M + m)·10`, M and m of the coldest month | "It : índice de termicidad (T + M + m) 10" |
| Itc | `It + Ci`, Fig. 7, only beyond 23° N/S | "En las zonas extratropicales … (al norte y al sur del paralelo 23º N y S), el índice de termicidad compensado (Itc) …" |
| Tp | `10·Σ Ti` over months with `Ti > 0` | "Tp : temperatura positiva anual … Ti1-12 > 0ºC" |
| Tps | Tp of the summer quarter | "Tps : temperatura positiva del trimestre estival" |
| Io | `(Pp/Tp)·10`, Pp = precipitation of the months with `Ti > 0` | "Io : índice ombrotérmico anual (Pp/Tp) 10" |
| Ios2/3/4 | `(Pp/Tp)·10` of the warmest summer two months / the summer quarter / the summer quarter plus the month before; summer quarter Tr3 = Jun–Aug (N), Dec–Feb (S); month before = May / Nov | ch. 2a ("Ios2 : … bimestre más cálido del trimestre estival (Tr3)", "Ios4 : … trimestre estival (Tr3) y del mes inmediatamente anterior"); ch. 2c for the hemispheres |
| Pss, Psw | precipitation of the warmer / colder half-year | "Pss : precipitación del semestre más cálido del año" |
| Pcm1/2/3 | precipitation of the warmest four-month run, the four after it, the four before it | "Pcm1 : precipitación del cuatrimestre más cálido del año"; "Pcm2 : … siguiente …"; "Pcm3 … anterior …" |

**Interpretations (marked in code):**
- *Half-years.* The source does not say which six months; tempus uses the two
  semesters of WBCS08's semestral index Iosm ("N: 4-9", "N: 10-3"), the warmer
  by temperature being Pss — the same split as Köppen's `halfYears` (Apr–Sep
  on a tie).
- *Warmest bimonth.* "Bimestre" is read as two consecutive months, so the
  candidates are Jun–Jul and Jul–Aug (Dec–Jan / Jan–Feb); the earlier wins a
  tie. The Mediterranean definition asks for "al menos durante dos meses
  consecutivos" (ch. 2c).
- *Frozen months.* Ios sums only months above 0 °C (Pp and Tp are "positive").
  A summer window with no month above 0 °C gets Ios = +∞: the drought test
  "P ≤ 2T" (ch. 2c) cannot hold with T ≤ 0.
- *Warmest four-month run.* Cyclic over the year, earliest start on a tie.
- *Fig. 7 beyond Ic 65.* The table ends at 65; the last factor (30) is applied
  beyond it.

### Itc compensation (WBCS08 Fig. 7)

| Ic | Ci | tempus |
|---|---|---|
| ≤ 8 | `C0 = f0·(8 − Ic)`, f0 = 10, subtracted | `−10·(8 − Ic)` for `Ic < 8` (0 at 8) |
| 8–18 | 0 ("Itc = It") | 0 for `8 ≤ Ic ≤ 18` |
| 18 < Ic ≤ 21 | `5·(Ic − 18)` | same |
| 21 < Ic ≤ 28 | `15 + 15·(Ic − 21)` | same |
| 28 < Ic ≤ 46 | `15 + 105 + 25·(Ic − 28)` | `120 + 25·(Ic − 28)` |
| 46 < Ic ≤ 65 | `15 + 105 + 450 + 30·(Ic − 46)` | `570 + 30·(Ic − 46)` |

P14 Table 1 gives the same rule up to Ic 28 ("If (Ic ≤ 18) then no
compensation; if (18 < Ic ≤ 21) then Itc = It + (5 × (Ic − 18)); if (21 < Ic ≤
28) then Itc = It + (15 + (15 × (Ic − 21))").

### Summer drought (Ios compensation, RM2011 via P14 Table 2)

WBCS08 ch. 2c: "un territorio no es mediterráneo si … Ios2 es superior a 2 …
Si Ios2 es menor o igual a 2.0 … puede ser o no mediterráneo …"; then Ios3,
then Ios4: "(Ios4 <= 2.0) son definitivamente mediterráneos". Its Fig. 6
("Tabla de compensación") relaxes the limits by Io but does not say how the
three columns combine.

P14 Table 2 ("Compensation threshold values derived from Rivas-Martínez et
al. (2011)") has the same numbers plus an explicit procedure, which tempus
implements (`iosLimits`, `mediterraneanSummer`):

| Io | Ios2 limit | Ios3 limit | Ios4 limit |
|---|---|---|---|
| < 2.0 | 2.0 | – | – |
| 2.01–2.8 | 1.9 | 2.0 | – |
| 2.81–3.6 | 1.8 | 1.9 | 2.0 |
| 3.61–4.8 | 1.8 | 1.9 | 2.0 |
| 4.81–6.0 | 1.7 | 1.9 | 2.0 |
| 6.01–7.0 | 1.5 | 1.8 | 2.0 |
| 7.01–8.0 | 1.4 | 1.8 | 2.0 |
| 8.01–9.0 | 1.3 | 1.8 | 2.0 |
| 9.01–10.0 | 1.2 | 1.8 | 2.0 |
| 10.01–11.0 | 1.1 | 1.7 | 2.0 |
| 11.01–12.0 | 1.0 | 1.7 | 2.0 |
| > 12.0 | 0.9 | 1.7 | 2.0 |

Procedure, verbatim: "Ios2 values >2 always identify the temperate
macrobioclimate. On the contrary, Ios2 values lower than the value indicated in
the Table according to the Io range identifies the mediterranean
macrobioclimate. In the case of the Ios2 value between 2 and the value
indicated in the Table, there are the following possibilities: if Ios3 is
lower than those shown in the Table, the macrobioclimate is mediterranean,
otherwise if it is >2, the macrobioclimate is temperate, and if Ios3 is between
2 and that value shown in the Table we must consider Ios4. If this is <2, the
area finally belongs to the mediterranean macrobioclimate, if not to the
temperate macrobioclimate."

**Interpretations:** rows have inclusive upper bounds (Io = 2.0 is the first
row, 2.8 the second); "–" cells are read as the uncompensated 2.0. At exactly
Ios4 = 2.0 P14 ("<2 … mediterranean, if not … temperate") and WBCS08 ch. 2c
("Ios4 <= 2.0 … mediterráneos") disagree; tempus follows P14 (the 2011
procedure). The same predicate is used in steps 7, 8, 10 and 14 (the key
writes "Iosc" in 7, 8, 10 and "Iosc4" in 14).

## Thermotypes (WBCS08 ch. 3d, Fig. 19; P14 Table 7)

"se reconoce una secuencia … de termotipos … en cada uno de los macrobioclimas
…: tropical (infra-, termo-, meso-, supra-, oro-, crioro- y gélido);
mediterráneo (…igual…); templado (…igual…); boreal (termo-, meso-, supra-,
oro-, crioro- y gélido); polar (termo-, meso-, supra- y gélido)."

Each thermotype is the union of its lower and upper horizon in Fig. 19.
Values are lower bounds, exclusive (P14 writes "2001–2175", "291–350"), so a
value equal to a bound gets the colder thermotype.

| Thermotype | Tropical It / Tp | Mediterranean Itc / Tp | Temperate Itc / Tp | Boreal Tp | Polar Tp |
|---|---|---|---|---|---|
| infra- | > 710 / > 2900 | > 450 / > 2400 | > 410 / > 2350 | — | — |
| thermo- | > 490 / > 2300 | > 350 / > 2100 | > 290 / > 2000 | > 680 | > 280 |
| meso- | > 320 / > 1700 | > 220 / > 1500 | > 190 / > 1400 | > 580 | > 100 |
| supra- | > 160 / > 950 | ≥ 120 / > 900 | ≥ 120 / > 800 | > 480 | > 0 |
| oro- | ≥ 120 / > 450 | — / > 450 | — / > 380 | > 380 | — |
| cryoro- | — / > 0 | — / > 0 | — / > 0 | > 0 | — |
| gelid | Tp = 0 | Tp = 0 | Tp = 0 | Tp = 0 | Tp = 0 |

P14 Table 7 matches the Mediterranean and temperate columns exactly
(inframediterranean upper Itc 451–515 / Tp 2401–2600 … cryorotemperate upper
Tp 1–100, gelid 0). Fig. 19's temperate cryoro row "100-240-380 / 1-50-100" is
a typo on the page; P14 gives 101–380 / 1–100. Fig. 19's boreal thermo- top
"(740)-(800)" is in parentheses; tempus leaves it open-ended (a boreal site
with Tp > 800 is still thermoboreal).

**Which index decides** (`thermotypeFor`):
- WBCS08 ch. 3: "A cualquier latitud, cuando el índice de termicidad (It) es
  inferior a 120 o cuando el índice de continentalidad (Ic) es igual o superior
  a 21, para calcular el termotipo se utiliza … Tp". P14 Table 7: "Tp is used if
  Ic ≥ 21 or It, Itc < 120".
- WBCS08 ch. 2d says the same "en los territorios extratropicales … (> 23º N &
  S) con un índice de continentalidad >= 21". The two chapters differ on
  whether Ic ≥ 21 also switches to Tp inside the tropics; tempus follows ch. 3
  and P14 ("a cualquier latitud"). This affects only tropical sites with Ic ≥ 21.
- It inside 23° N/S, Itc beyond (ch. 2d). Boreal and polar have no It column
  in Fig. 19 ("-"), so they always use Tp.
- tempus reports the deciding index and its value (`thermotype.index`,
  `thermotype.value`).

## Elevation input

The key needs the site elevation. tempus reads it from the Open-Meteo archive
response, field `elevation`: "The elevation from a 90 meter digital elevation
model. Statistical downscaling is used to adapt weather conditions for this
elevation." (OM). The archive temperatures are downscaled to exactly that
elevation, so it is the right starting point for the reduction to 200 m.

**Fallback:** when the response has no (or a NaN) elevation, no altitude
reduction is applied — the key treats the site as lying at or below 200 m,
step 3 cannot fire, and `macrobioclimate.elevationM` is omitted. High-mountain
sites then lean polar or boreal (Säntis: polar instead of temperate; pinned by
`TestMacrobioclimate_WithoutElevation` and
`TestFetch_MissingElevationFallsBackToNoReduction`).

## Reference stations: before → after

WMO 1991–2020 normals; elevation from the normals metadata. "Before" is the
pre-PR `RivasMartinezThermotype` (Köppen-derived macrobioclimate, temperate Tp
ladder, after the Beck 2018 Köppen fix). Index values from an independent
script over the same normals.

| Station (WMO) | Elev. m | Before | After (macro / thermotype, index) | Deciding criterion |
|---|---|---|---|---|
| Säntis (06680) | 2501 | cryoropolar | temperate / cryorotemperate (Tp 234) | lat < 72; Tp₂₀₀ 1500 ≥ 380; T₂₀₀ 13.1 ≥ 5.3; Ios2 41 > 2 |
| Zugspitze (10961) | 2964.5 | cryoropolar | temperate / cryorotemperate (Tp 92) | Tp₂₀₀ 1612; T₂₀₀ 12.7; Ios2 51 |
| Almería (08487) | 21 | thermotemperate | mediterranean / (thermomediterranean, It 448 — borderline, not asserted) | Io 0.86 → limit 2.0; Ios2 0.05 |
| Madrid Retiro (08222) | 667 | mesotemperate | mediterranean / mesomediterranean (Itc 296) | Io 2.23 → limit 1.9; Ios2 0.37 |
| Kunming (56778) | 1892 | mesotemperate | tropical / mesotropical (Itc 362) | step 5 at 200 m: T 26.1 > 21, M 24.8 > 18; step 6: Pss 810 > Psw 182 |
| Mexico City Tacubaya (76680) | 2309 | thermotemperate | tropical / mesotropical (It 465) | lat 19.4 < 23 |
| Verkhoyansk (24266) | 136 | oromediterranean | boreal / supraboreal (Tp 499) | step 13, Ic 61 > 28: T −13.9 < 3.8, Tp 499 < 800 |
| Palmer AK (70274) | 70 | oromediterranean | boreal / mesoboreal (Tp 663) | step 13, Ic 24.3: T 2.86 < 4.8, Tp 663 < 740 |
| Sodankylä (02836) | 179 | oroboreal | boreal / supraboreal (Tp 519) | step 13, Ic 27.5: T 0.33, Tp 519 |
| Hobart (94970) | 51 | mesotemperate | temperate / thermotemperate (Itc 314) | Ios2 (Jan–Feb) 2.30 > 2; Ic 8.9 → Itc = It |
| Darwin (94120) | 30 | thermotropical | tropical / infratropical (It 783) | It > 710 |
| Bangkok (48455) | 3 | thermotropical | tropical / infratropical (It 844) | It > 710 |
| Potsdam (10379) | 81 | supratemperate | temperate / supratemperate (Tp 1175) | Itc 114 < 120 → Tp |
| Wien Hohe Warte (11035) | 198 | supratemperate | temperate / (supratemperate, Itc 151 — borderline: Ic 20.8, Tp 1383; not asserted) | Ios2 3.37 |
| Helsinki (02978) | 3 | supratemperate | temperate / supratemperate (Tp 860) | Ic 21.9 → step 13 row 21–28: T 6.48 ≥ 4.8; Ic ≥ 21 → Tp |
| Sevilla (08391) | 34 | thermomediterranean | mediterranean / thermomediterranean (Itc 416) | Ios2 0.06 |
| Roma Ciampino (16239) | 129 | mesomediterranean | mediterranean / mesomediterranean (Itc 312) | Io 3.96 → limit 1.8; Ios2 0.99 |
| Resolute (71018) | 30 | cryoropolar | polar / suprapolar (Tp 77) | lat 74.7 > 72; Tp 1–100 |
| South Pole (READER, not a WMO normal) | — (not used) | cryoropolar | polar / gelid (Tp 0) | lat −90 < −56; Tp = 0 |

Not asserted because borderline: Almería (It 448 vs 450), Wien (Ic 20.8 vs the
It/Tp switch at 21, Tp 1383 vs 1400), Perth (It 455 vs 450; dossier).

**Cross-check over all 4,339 WMO 1991–2020 stations with T, Tmin, Tmax, P and
elevation:** the Go implementation and an independent Python implementation of
this document agree at 4,338 stations; the one difference (Craiova, 15450: Tp is
exactly 1400 in decimal, and the two implementations round the binary sum to
opposite sides of the meso/supra bound) is a floating-point tie, not a rule
difference. Distribution: temperate 1,920, tropical 1,078,
mediterranean 991, boreal 322, polar 28. The thermotype code differs at 2,264 of
the 4,339 stations from the output of the code before both the Köppen and this
fix (tempus `main` @ d9213d9).

## Open gaps

- **RM2011 itself** was not obtained. Everything except the Ios compensation
  procedure and the Mediterranean/temperate thermotype bounds (both via P14)
  is from WBCS08; any change the print edition made to the key, the altitude
  footnote, or the tropical/boreal/polar ladders is unverified.
- **Tp rate below 48° N / 51° S** in steps 11 and 13: interpretation (55 per
  100 m), see above.
- **Tps and Tmax in step 12** are not reduced to 200 m: the footnote gives no
  rate for them.
- **Ic ≥ 21 inside the tropics:** WBCS08 ch. 2d vs ch. 3 disagree; ch. 3 and
  P14 followed.
- **Bioclimates, variants, ombrotypes, horizons** (WBCS08 ch. 5.2–5.7; the
  lower/upper horizon split of Fig. 19) are out of scope.
- The South Pole case is not an official normal.
