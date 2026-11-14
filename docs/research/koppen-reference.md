# Köppen-Geiger & Rivas-Martínez reference dossier for tempus

> **Status of this dossier.**
> This is the sourced research behind tempus's Köppen-Geiger classifier
> (`internal/domain/koppen.go`). The Köppen deviations **D1–D7** listed below
> are fixed: the classifier now follows Beck et al. 2018, Table 2 plus the
> Methods text, and `internal/domain/koppen_reference_test.go` pins the
> reference stations from Part 3 against it.
>
> The Rivas-Martínez findings (**Part 2, D8–D12**, `thermotype.go`) are
> **not implemented yet**; they are pending a decision on how far the
> thermotype should follow WBCS08. The "code today" columns describe the code
> *before* the Köppen fixes (tempus `main` @ d9213d9).

Date: 2026-11-14. Code under test: `internal/domain/koppen.go` and `internal/domain/thermotype.go` (tempus `main` @ d9213d9).

**Method.** Every criterion below is quoted from a document I downloaded in this session. Every station value comes from a downloaded data file. To check code against source I did two things in the scratchpad, with no repo changes:
1. I compiled the **real** `koppen.go` and `thermotype.go` (copied verbatim, with the helpers copied from `bioclim.go`) into a small harness.
2. I wrote an independent reference implementation of Beck 2018 Table 2 plus the Methods text, and ran both over the complete WMO 1991–2020 normals collection: 4,540 stations with 12 valid Tmean and 12 valid precipitation values.

The harness files (`tools/beck.py`, `tools/rm.py`, the Go harness, `classified.json`, `go_literals.txt`) lived in a scratch directory and are not part of the repository. The Beck18 rules they implemented are the ones now in `koppen.go`; the station literals of §3.2 are in `koppen_reference_test.go`.

## Sources actually fetched

| Key | Document | URL used | Location in document |
|---|---|---|---|
| **Beck18** | Beck et al. 2018, *Sci. Data* 5:180214 | https://www.nature.com/articles/sdata2018214 (blocked by a paywall redirect). Full text fetched as JATS XML: https://www.ebi.ac.uk/europepmc/webservices/rest/PMC6207062/fullTextXML | **Table 2** "Overview of the Köppen-Geiger climate classes including the defining criteria", plus Methods § "Köppen-Geiger climate classification". **Note: the criteria are in Table 2. Table 1 lists the input datasets.** |
| **Peel07** | Peel, Finlayson & McMahon 2007, *HESS* 11:1633–1644 | https://hess.copernicus.org/articles/11/1633/2007/hess-11-1633-2007.pdf | Text p. 1635, Table 1 p. 1636 |
| **Kottek06** | Kottek et al. 2006, *Meteorol. Z.* 15:259–263 | https://opus.bibliothek.uni-augsburg.de/opus4/files/40083/metz_Vol_15_No_3_p259-263_World_Map_of_the_Koppen_Geiger_climate_classification_updated_55034.pdf | Table 1 p. 260; Table 2 and Eq. 2.1 p. 262 |
| **WBCS08** | Rivas-Martínez, *Global Bioclimatics (Clasificación Bioclimática de la Tierra)*, version 01-12-2008 (the online WBCS, in Spanish). The live site globalbioclimatics.org returns HTTP 404, so I used Wayback snapshots from 2013: | ch. 2 `…/global_bioclimatics-2008_02.htm`, ch. 3 `…_03.htm`, ch. 5 `…_05.htm`, prefix `http://web.archive.org/web/2013id_/http://www.globalbioclimatics.org:80/book/bioc/` | ch. 5.1 "Clave para los macrobioclimas"; ch. 3 macrobioclimate definitions and **Figura 19** (thermotype horizons); ch. 2c (Ios2/Ios3/Ios4), Fig. 7 (Itc), Fig. 14 (latitude belts) |
| **RM2011** | Rivas-Martínez, Rivas-Sáenz & Penas 2011, *Global Geobotany* 1:1–634 | **Not obtainable.** The PDF is not openly hosted. ResearchGate and Tandfonline (the "Bioclimate of Italy" paper cited in thermotype.go) returned bot challenges. | **All RM numbers below come from WBCS08. Any difference in the 2011 print edition is unverified.** |
| **WMO9120** | WMO Climatological Standard Normals 1991–2020, NCEI Accession 0253808 (doi:10.25921/800j-vn07). Official national submissions (DWD, MeteoSwiss, GeoSphere Austria, NOAA, BoM, Roshydromet, AEMET, …). | https://www.nodc.noaa.gov/archive/arc0216/0253808/2.2/data/0-data/data-composite-primary-parameters/wmo_normals_9120_{TAVG,TMAX,TMIN,PRCP}.csv ; readme: …/0-data/WMO-Normals-9120-Readme.pdf | Units per the readme: PRCP in mm, TAVG/TMAX/TMIN in °C. Missing values are coded as −99.9 (and negative precipitation), and I excluded them. |
| **READER** | BAS READER, Amundsen-Scott monthly mean temperatures | https://legacy.bas.ac.uk/met/READER/surface/Amundsen_Scott.All.temperature.txt | I averaged the 1991–2020 months myself (30/30 years present). This is **not** an official WMO normal, and it has no precipitation. |

---

## Part 1: Köppen-Geiger criteria vs. code

Verbatim **Beck18 Table 2** (criteria column; footnote quoted below):

```
A  Tropical            Not (B) & Tcold≥18
 f - Rainforest        Pdry≥60
 m - Monsoon           Not (Af) & Pdry≥100-MAP/25
 w - Savannah          Not (Af) & Pdry<100-MAP/25
B  Arid                MAP<10×Pthreshold
 W - Desert            MAP<5×Pthreshold
 S - Steppe            MAP≥5×Pthreshold
  h  - Hot             MAT≥18
  k  - Cold            MAT<18
C  Temperate           Not (B) & Thot>10 & 0<Tcold<18
 s - Dry summer        Psdry<40 & Psdry<Pwwet/3
 w - Dry winter        Pwdry<Pswet/10
 f - Without dry season Not (Cs) or (Cw)
  a  - Hot summer      Thot≥22
  b  - Warm summer     Not (a) & Tmon10≥4
  c  - Cold summer     Not (a or b) & 1≤Tmon10<4
D  Cold                Not (B) & Thot>10 & Tcold≤0
 s/w/f                 (same as C)
  a  - Hot summer      Thot≥22
  b  - Warm summer     Not (a) & Tmon10≥4
  c  - Cold summer     Not (a, b, or d)
  d  - Very cold winter Not (a or b) & Tcold<-38
E  Polar               Not (B) & Thot≤10
 T - Tundra            Thot>0
 F - Frost             Thot≤0
```
Footnote (Beck18): "Tmon10 = the number of months with air temperature >10 °C … Pthreshold=2×MAT if >70% of precipitation falls in winter, Pthreshold=2×MAT+28 if >70% of precipitation falls in summer, otherwise Pthreshold=2×MAT+14. Summer (winter) is the six-month period that is warmer (colder) between April-September and October-March."

Beck18 Methods: "This classification is identical to that presented by Köppen in 1936 with three differences. First, temperate (C) and cold (D) climates are distinguished using a 0 °C threshold instead of a 3 °C threshold … Second, the arid (B) sub-climates W (desert) and S (steppe) were identified depending on whether 70% of precipitation occurred in summer or winter. Third, the sub-climates s (dry summer) and w (dry winter) within the C and D climates were made mutually exclusive by assigning s when more precipitation falls in winter than in summer and assigning w otherwise. Note that the tropical (A), temperate (C), cold (D), and polar (E) climates are mutually exclusive but may intersect with the arid (B) class. To account for this, climate type B was given precedence over the other classes."

| # | Topic | Source criterion (quote) | Code | Verdict |
|---|---|---|---|---|
| 1 | **Precedence B vs E** | Beck18: "climate type B was given precedence over the other classes". Kottek06 Table 1 caption (p. 260) says the opposite: "This key implies that the polar climates (E) have to be determined first, followed by the arid climates (B)". Peel07 gives no explicit order. | `switch { case mapp < 10*pth: B; case thot < 10: E; …}`, so B comes first | **Matches Beck18** and differs from Kottek06. **Dry polar sites:** for a polar site Pthreshold is usually ≤ 0 (e.g. Eureka, Canada: MAT −18.1, 72.1 % summer precipitation, so Pth = 2·(−18.1)+28 = −8.2 and 10·Pth = −82, which MAP = 77.5 mm cannot be below). The site stays E under either order. Across all 4,540 WMO 1991–2020 stations, **no station** satisfies both Thot ≤ 10 and MAP < 10·Pth, so the ordering has no real-world test case. A synthetic case is needed: Pth > 0 needs MAT > −14 (summer-rain case) or MAT > −7 (even case). |
| 2 | **E threshold** | Beck18: E "Thot≤10", T "Thot>0", F "Thot≤0"; C and D both need "Thot>10". Peel07 Table 1: E "Thot <10" while C/D need "Thot >10", so **Thot = 10 falls in no class in Peel07**. Kottek06: "Tmax < +10 °C"; ET "0 °C ≤ Tmax < +10 °C"; EF "Tmax < 0 °C". | `case thot < 10:` → E; `if thot > 0 {ET} else {EF}` | **Deviation at Thot = 10.0**: Beck18 gives E, the code falls through to C/D. Real cases: **Mt Washington (US, WMO 72613)**, Thot 10.0 → Beck **ET**, code **Dfc**; **Nagqu (CN, 55299)** → Beck **ET**, code **Dwc**. ET/EF (`> 0`) matches Beck18. Kottek06 would put Thot = 0 in ET; Beck18 and the code put it in EF. |
| 3 | **A sub-types; "As"** | Beck18 and Peel07 define only **Af, Am, Aw** (w = "Not (Af) & Pdry<100-MAP/25"). Neither has an "As" row. Kottek06 Table 1 does define it: "As Equatorial savannah with dry summer — Pmin < 60 mm in summer"; "Aw … Pmin < 60 mm in winter". Kottek06's Am rule is "Pann ≥ 25 (100−Pmin)", which is algebraically the same as Pdry ≥ 100 − MAP/25. | Returns "As" when the driest month falls in the latitude-defined summer half | **Deviation vs Beck18**: "As" is not a Beck18 class. Beck18 gives **Aw**. The As/Aw choice is Kottek06's scheme, but even then Kottek06's As needs "Pmin < 60 mm in summer", not "driest month in summer". 40 of 684 A stations are affected (Lihue HI, Fortaleza, Natal, Acapulco, 9 Sri Lankan stations, …). Af ≥ 60 and Am ≥ 100 − MAP/25 match. |
| 4 | **C/D boundary** | Beck18: C "0<Tcold<18", D "Tcold≤0". Peel07 p. 1635: "used the temperature of the coldest month >0 °C, rather than >–3 °C as used by Köppen". Kottek06: C "−3 °C < Tmin < +18 °C", D "Tmin ≤ −3 °C". Both C and D need "Thot>10" (Beck18, Peel07). | `tcold >= 18` → A; `tcold > 0` → C; else D. Thot > 10 is not checked explicitly. | **Matches Beck18 (0 °C)**, with the Thot = 10 exception from row 2. Kottek06 uses −3 °C, so tests copied from Kottek06 maps will disagree on purpose. |
| 5 | **s / w / f, and precedence** | Beck18 and Peel07: s "Psdry<40 & Psdry<Pwwet/3"; w "Pwdry<Pswet/10". **Beck18 Methods: when both hold, "assigning s when more precipitation falls in winter than in summer and assigning w otherwise."** Kottek06 builds the mutual exclusion into the criteria: Cs "Psmin < Pwmin, Pwmax > 3 Psmin and Psmin < 40 mm"; Cw "Pwmin < Psmin and Psmax > 10 Pwmin". | `seasonLetter`: tests s first, with no half-year-total tie-break | Thresholds match (strict `<` everywhere). **Precedence deviates.** When both criteria hold and summer is wetter, Beck18 gives **w** and the code gives **s**. Real cases (13): **Kunming**, **Mexico City/Tacubaya**, **Dali**, Mpika (ZM) Cwb→code Csb; **Isoka, Kabwe, Lundazi (ZM)**, Mindat (MM), **Morelia, Guanajuato, Ciudad Guzmán (MX)**, **Mackay (AU)** Cwa→Csa; Ust-Barguzin (RU) Dwc→Dsc. These flow through to RM: the code then calls Kunming and Mexico City **"mediterranean"**. |
| 6 | **Heat letters / d precedence** | Beck18 and Peel07: d "Not (a or b) & Tcold<-38"; c "Not (a, b, or d)"; b "Not (a) & Tmon10≥4", **with Tmon10 = months ">10 °C"**. Kottek06 Table 2 (p. 262): b "not (a) and at least 4 Tmon ≥ +10 °C"; c "not (b) and Tmin > −38 °C"; d "like (c) but Tmin ≤ −38 °C". | `heatLetter`: `if allowD && tcold < -38 {d}` **before** a/b; `tmon10` counts `t >= 10` | **Two deviations.** (a) **d precedence**: in all three sources a and b win over d; the code puts d first. In the WMO 1991–2020 data **no station** has Tcold < −38 together with Thot ≥ 22 or Tmon10 ≥ 4. The warmest Tcold < −38 station is Ust-Maya, Thot 18.9 and Tmon10 = 3, so the bug only shows on synthetic input. **No real "Dxa/Dxb with Tcold < −38" station exists in this dataset.** (b) **Tmon10 counts ≥ 10, Beck18 says > 10**. The code follows Kottek06 here, not Beck18. 10 real stations flip from Beck **Dfc** to code **Dfb**, including MeteoSwiss **Napf**, **Ulrichen**, **La Brévine**; Finnish Kuopio-Maaninka and Seinäjoki; Swedish Edsbyn and Östmark; **Petrozavodsk**; Leusi; Fjærland. Each has exactly one month at 10.0 °C. The d threshold `< -38` matches Beck18 (strict); Kottek06 uses ≤ −38. |
| 7 | **B rules** | Beck18: Pthreshold uses "**>70%**" (strict); BW "MAP<5×Pthreshold"; BS "MAP≥5×Pthreshold"; h "MAT≥18". Peel07: "if 70% of MAP occurs in winter …" (no strictness given). Kottek06 Eq. 2.1: "**at least 2/3** of the annual precipitation", and BW "Pann ≤ 5 Pth", BS "Pann > 5 Pth". | `pWinter >= 0.7*mapp`, `pSummer >= 0.7*mapp`; BW `mapp < 5*pth`; h `mat >= 18` | BW/BS/h/k match Beck18. **Minor deviation**: the code uses `>=` 70 % where Beck18 says `>` 70 % (matters only at exactly 70.0 %). **Half-year dependence**: the summer/winter shares use the latitude-defined halves (row 8), which flips A↔B at real stations: **Tema (Ghana)** Beck **Aw**, code **BSh**; Lomé the same. |
| 8 | **Half-years** | Beck18: "Summer (winter) is the six-month period that is **warmer (colder)** between April-September and October-March." Peel07: "defined as the warmer (cooler) six month period of ONDJFM and AMJJAS." Kottek06: "summer and winter half-years **on the hemisphere considered**." | `halfYears(lat)`: lat < 0 gives summer ONDJFM, otherwise AMJJAS | **Deviation vs Beck18 and Peel07.** The code follows Kottek06. Results differ wherever the warmer half is not the hemisphere's nominal summer, mostly in the near-equatorial belt. Real cases (10): **Tema**, Lomé Aw→BSh; **Botana / Obonuco / Isla del Santuario (Colombian Andes)** Cfb→Csb; Llano Largo / Tunguavita / Villa Luisa Csb→Cfb; Ikeja BSh→As and Calabar Csb→Cwb. **Do not use Ikeja or Calabar**: their WMO values look erroneous (Calabar Thot 12.0 °C at sea level; Ikeja MAP 675 mm). |

**Totals across 4,540 WMO 1991–2020 stations:** the real Go `KoppenGeiger` disagrees with Beck18 at **75 stations (1.65 %)**:
- 40 Aw → As (no As in Beck18)
- 13 from s/w precedence
- 10 from the half-year definition (Ikeja also ends up as "As")
- 10 from Tmon10 counting ≥ instead of >
- 2 from Thot = 10.0

Sum: 75.

**Inequality summary, Beck18 vs code:**
- Tcold ≥ 18 ✔
- 0 < Tcold < 18 ✔
- Tcold ≤ 0 ✔
- Thot ≤ 10 for E ✘ (code `< 10`)
- Thot > 0 for ET ✔
- Pdry ≥ 60 ✔
- Pdry ≥ 100−MAP/25 ✔
- MAP < 10·Pth ✔
- MAP < 5·Pth ✔
- MAT ≥ 18 ✔
- > 70 % ✘ (code `>=`)
- Psdry < 40 ✔
- Psdry < Pwwet/3 ✔
- Pwdry < Pswet/10 ✔
- Thot ≥ 22 ✔
- Tmon10 counts > 10 ✘ (code `>= 10`)
- Tmon10 ≥ 4 ✔
- Tcold < −38 ✔ (but evaluated first ✘)

---

## Part 2: Rivas-Martínez macrobioclimates (WBCS08)

### 2.1 Definitions as given in the source (ch. 3 text and ch. 5.1 key, my translation, quotes in Spanish)

Latitude belts (Fig. 14): Ecuatorial 7°N–7°S; Eutropical 7–23°; Subtropical 23–35°; Eutemplada 35–52°; Subtemplada 52–66°N / 52–60°S; Ártica 66–90°N; Antártica 60–90°S.

**Key 5.1 "Clave para los macrobioclimas"** (condensed; the numbers are verbatim):
1. "Territorios intertropicales entre los paralelos 23º N y S" → **Tropical**. Otherwise go to 2.
2. 23°–35° N/S go to 3; beyond 35° go to 11.
3. Eurasia 26–35°N, 70–120°E, altitude ≥ 2000 m → 10. Otherwise → 4.
4. Two of "T >= 25°, m >= 10°, It, Itc >= 580" → **Tropical**. Otherwise → 5.
5. Values reduced to altitude < 200 m. If two of "T > 21°, M > 18°, It, Itc >= 470" → 6, otherwise → 8.
6. "Pss > Psw" (warm half-year wetter) → **Tropical**. Otherwise → 7.
7. "Ios2 > 2 o … Iosc > 2" → **Tropical**. Otherwise → 9.
8. "Ios2 > 2 o … Iosc > 2" → **Templado**, else **Mediterráneo**.
9. "Pcm3 < Pcm1 > Pcm2" → **Tropical**, else **Mediterráneo**.
10. Ios2 > 2 or Iosc > 2 → Templado, else Mediterráneo.
11. "latitud Norte > 72º o … Sur > 56º" → **Polar**. Otherwise, at < 200 m, "Tp < 380" → **Polar**, and Tp > 380 → 12.
12. "Ic < 11, Tps <= 290, y … Tmax <= 10º" → **Boreal**.
13. "Ic < 21, T < 5.3º y Tp < 720; … Ic = 21-28, T < 4.8º y Tp < 740; … Ic > 28, T < 3.8º y Tp < 800" → **Boreal**.
14. "Ios2 > 2 o … Iosc4 > 2" → **Templado**. Otherwise **Mediterráneo**.

Footnote (1) on altitude reduction: add 0.6 °C to T, 0.5 °C to M and 13 It units per 100 m above 200 m. North of 48°N (south of 51°S), add 0.4 °C to T and 12 Tp units per 100 m. For thermotype work with Ic ≥ 21 or It < 120, add 55 Tp units per 100 m.

**Ch. 3 latitude windows per macrobioclimate:**
- Tropical: 0–23°, plus parts of 23–35° as above.
- Mediterranean: "23º a 52º N & S … al menos dos meses consecutivos con aridez durante el período más cálido … (Ps2 < 2Ts2)".
- Temperate: "23º a 66º N y 23º a 55º S … (Ps2>=2Ts2)".
- Boreal: "43º a 71º N y 51º a 56º S", below the boreal–temperate thresholds after reduction to 200 m.
- Polar: "51º a 90º N y 53º a 90ºS … Tp … inferior a 380".

**Summer-drought definition (ch. 2c):** "un territorio no es mediterráneo si … Ios2 es superior a 2 (Ios2 > 2)". If Ios2 ≤ 2, check Ios3 (JJA, or DJF in the south) > 2, then Ios4 (MJJA, or NDJF) > 2. Only "Ios4 <= 2" makes it "definitivamente mediterráneo". Fig. 6 additionally relaxes the Ios2/Ios3 thresholds by annual Io (e.g. Io > 12 → Ios2 ≥ 0.9). Ios = P(mm) / ΣT(°C, positive months), because Io = (Pp/Tp)·10 with Tp in tenths of a degree.

**Summary:** the RM macrobioclimate depends on **latitude band**, **altitude reduction**, **Ic, T, Tp, Tps, Tmax**, **It/Itc** and **summer ombrothermic indices** (Ios2/3/4, Pss vs Psw). None of these is a Köppen letter.

### 2.2 Thermotype Tp thresholds (WBCS08 Fig. 19, "Tp" column; It/Itc column in brackets)

| Thermotype | Tropical | Mediterranean | **Temperate** | Boreal | Polar |
|---|---|---|---|---|---|
| infra- | > 2900 (It > 710) | > 2400 (It 450–580) | > 2350 (It > 410) | — | — |
| thermo- | 2300–2900 (490–710) | 2100–2400 (350–450) | **2000–2350** (290–410) | 680–(800) | 280–380 |
| meso- | 1700–2300 (320–490) | 1500–2100 (220–350) | **1400–2000** (190–240–290) | 580–680 | 100–280 |
| supra- | 950–1700 (160–320) | 900–1500 (120–220) | **800–1400** ((120)–190) | 480–580 | 1–100 |
| oro- | 450–950 ((120)–160) | 450–900 | **380–800** | 380–480 | — |
| cryoro- | 1–450 | 1–450 | **1–380** | 1–380 | — |
| gelid | 0 | 0 | 0 | 0 | 0 |

The code's ladder (thermo > 2000, meso 1400–2000, supra 800–1400, oro 380–800, cryoro ≤ 380) is **exactly the temperate column**, minus infra and gelid. The thresholds **do differ by macrobioclimate**. The source states it directly: "los termoclimáticos (It, Itc, Tp) difieren algo en la mayoría de los macrobioclimas" (ch. 3, Pisos bioclimáticos).

Also from the source:
- The polar sequence is "termo-, meso-, supra- y gélido". There is **no oro- or crioro-polar**.
- The boreal sequence has no infra-.
- Tp is the primary measure **only** "cuando el índice de termicidad (It) es inferior a 120 o cuando el índice de continentalidad (Ic) es igual o superior a 21". Otherwise It/Itc decides.

The Tp definition matches: "Tp … Ti1-12 > 0ºC" (ch. 2 parameter list); the code uses `t > 0`. The ch. 3 prose writes "Σ Ti >= 0º", which makes no numerical difference. Fig. 19's temperate cryoro row reads "100-240-380 / 1-50-100", which looks like a transcription glitch on the source page.

### 2.3 How far the "derive from Köppen" shortcut deviates

**Structural problems:**
1. All **B** climates fall to `default` → "temperate". RM classifies arid areas by latitude and summer drought: Sahara and Sahel sites (< 23°) are **Tropical**; Spanish, Maghreb and Near-East steppes and deserts are **Mediterranean**.
2. Tropical is tied to Köppen A (Tcold ≥ 18), but RM makes **everything between 23°N and 23°S tropical, at any altitude**. Tropical highlands come out as temperate, mediterranean or even **polar**.
3. `code[1]=='s'` → mediterranean also catches **Ds** sites in eastern Siberia and Alaska (RM: boreal), and Köppen's s/w precedence error (Part 1 #5) adds Kunming and Mexico City.
4. E → polar ignores RM's latitude window (≥ 51°N / 53°S). Alpine and Tibetan ET summits are **temperate** in RM.
5. Dc/Dd → boreal ignores latitude and altitude reduction: Alpine Dfc sites are temperate in RM. Dfb sites with T < 3.8–4.8 °C and Tp < 740–800 are **boreal** in RM.

**Measured on the WMO 1991–2020 set (4,339 stations with T, P, Tmin and Tmax):** my implementation of key 5.1 disagrees with `macrobioclimateSuffix` at **1,197 stations (27.6 %)**:

| code → RM | n | examples |
|---|---|---|
| temperate → mediterranean | 578 | Almería (BSh), Madrid-Retiro (BSk), Murcia, Algerian steppes |
| temperate → tropical | 423 | Tamanrasset, Djanet (BWh), Sahel BSh |
| boreal → temperate | 49 | Tibetan Dwc, Mt Washington-type mountains |
| mediterranean → temperate | 43 | Bandar Anzali, Ramsar (Caspian Csa: Ios not ≤ 2) |
| temperate → boreal | 38 | Krasnoyarsk, Yuzhno-Sakhalinsk (Dfb) |
| polar → temperate | 21 | Säntis, Zugspitze, Tibetan ET |
| mediterranean → tropical | 17 | Kunming, Dali, Zambia |
| mediterranean → boreal | 17 | Verkhoyansk, Magadan, Palmer AK (Ds) |
| polar → boreal / tropical | 7 | sub-Antarctic islands / Colombian páramo |
| boreal → polar, mediterranean → polar | 4 | Khatanga, Kugluktuk, Kangerlussuaq; Ilirnej |

Caveat on these counts: my key implementation approximates Iosc by the Ios2→Ios3→Ios4 sequence of ch. 2c, without the Io-dependent Fig. 6 relaxations. It does not implement step 9 (Pcm). Step 4 uses It rather than Itc. Counts are **indicative**; the individual counter-examples in Part 3 are clear-cut, far from any threshold, unless flagged.

**Thermotype:** even with the macrobioclimate fixed, the code's single Tp ladder and "always Tp" rule disagree with Fig. 19 at **2,229 of 4,339 stations**. Typical cases:
- every boreal site gets "oro-": Sodankylä Tp 519 → **supraboreal** per RM;
- polar names that don't exist: Resolute Tp 77 → code "cryoropolar", RM **suprapolar**;
- Tp = 0 gives "cryoro" instead of **gelid**: South Pole → code "cryoropolar", RM **gélido**;
- the hottest lowland tropics never become **infra**: Darwin It 783 → code thermotropical, RM **infratropical**;
- It-based sites diverge: Hobart It 314 → RM **thermotemperate**, code meso.

---

## Part 3: Reference stations

All data are **WMO 1991–2020 official normals** (WMO9120 composite files above, keyed by station ID), except the South Pole.
- Expected Köppen = **computed with Beck18 Table 2 and the Methods text** (reference implementation `tools/beck.py`).
- "code today" = output of the **real** `KoppenGeiger` / `RivasMartinezThermotype`.
- RM = my implementation of WBCS08 key 5.1 and Fig. 19 (approximate where noted).

### 3.1 Overview

| Branch | Station (WMO id, country) | lat | Beck18 expected | Code today | RM expected (WBCS08) | Note / computation |
|---|---|---|---|---|---|---|
| **EF** | Amundsen-Scott South Pole (READER) | −90 | **EF** | EF / cryoropolar | polar, **gélido** (Tp = 0) | Thot = −27.3 ≤ 0. MAT = −49.15, so Pth ≤ 2·MAT+28 < 0 and B is impossible for any P ≥ 0. Use P = 0. |
| **ET** (Europe) | Säntis (06680, CH), 2501 m | 47.25 | **ET** | ET / cryoropolar | **temperate**, cryorotemperate | Thot 6.8 (0 < 6.8 ≤ 10). MAP 2840 ≫ 10·Pth = 10·(2·−0.72+14) = 126. RM: lat < 51 so not polar; step 14 with T(200 m) = 13.1 and Ios2 = 41 gives temperate. **Counter-example to E → polar.** |
| ET (Europe) | Zugspitze (10961, DE), 2964 m | 47.42 | **ET** | ET / cryoropolar | **temperate**, cryorotemperate | Thot 3.8. Same RM reasoning. |
| ET + polar | Resolute (71018, CA) | 74.72 | **ET** | ET / cryoropolar | **polar** (lat > 72), **suprapolar** (Tp 77) | Thot 4.8. Pth = 2·(−15.09)+14 = −16.2 (summer share 67.6 % < 70 %), so not B. Code thermotype name is invalid in RM. |
| ET at Thot = 10.0 | Mt Washington (72613, US), 1910 m | 44.27 | **ET** (Thot ≤ 10) | **Dfc** ✘ / cryoroboreal | temperate, cryorotemperate | Boundary test for `thot < 10` vs `<= 10`. |
| **Am** | Miami Intl (72202, US) | 25.79 | **Am** | Am / thermotropical | tropical (step 4: T 25.2 ≥ 25, m 16.1 ≥ 10), thermotropical (It 659) | Tcold 20.3 ≥ 18. Pdry 46.6 < 60. 100 − 1712.3/25 = 31.5 ≤ 46.6, so Am. |
| Am (S. hemisphere) | Cairns Aero (94287, AU) | −16.87 | **Am** | Am / thermotropical | tropical, thermotropical (It 689) | Pdry 26.6 ≥ 100 − 1957/25 = 21.7. |
| **"As"** (NH) | Lihue (91165, US-HI) | 21.98 | **Aw** (Beck has no As) | **As** ✘ | tropical, thermotropical | Pdry 44.4 (Jul, summer half) < 100 − 919.9/25 = 63.2, so w. Not B: winter 66 % → Pth = 2·24.6+14 = 63.2, 10·Pth = 632 < 920. |
| "As" (SH) | Fortaleza (82397, BR) | −3.82 | **Aw** | **As** ✘ / thermotropical | tropical, **infratropical** (It 804) | Pdry 9.5 (Oct). Beck summer = ONDJFM (warmer half, matching the latitude rule here). |
| **Aw** (SH, half-year bonus) | Darwin Airport (94120, AU) | −12.42 | **Aw** | Aw / thermotropical | tropical, **infratropical** (It 783) | Pdry 0.8 (Aug, SH winter). If the hemisphere logic were wrong, the code would say "As". |
| Aw | Bangkok Metropolis (48455, TH) | 13.73 | **Aw** | Aw / thermotropical | tropical, **infratropical** (It 844) | Pdry 11.6 < 100 − 1702/25 = 31.9. |
| **Dfd** | Oymyakon (24688, RU) | 63.25 | **Dfd** | Dfd / oroboreal | boreal, oroboreal (Tp 449) | Tcold −45.7 < −38, Thot 15.3, Tmon10 = 3, Pwdry 5.3 ≥ Pswet/10 = 4.65, so f. |
| Dfd, near −38 | Ust-Maya (24966, RU) | 60.38 | **Dfd** | Dfd / oroboreal | boreal, **mesoboreal** (Tp 648) | Tcold −38.3 (0.3 K from the threshold). |
| Dsd (s borderline) | Verkhoyansk (24266, RU) | 67.55 | **Dsd** | Dsd / **oromediterranean** ✘ | **boreal**, supraboreal (Tp 499) | s: Psdry 4.0 (Apr) < 40 and < Pwwet/3 = 12.8/3 = 4.27. w: Pwdry 4.6 is not < Pswet/10 = 3.47. Very sensitive to April. Shows the Ds → mediterranean shortcut is wrong. |
| Tmon10 boundary (Europe) | Napf (06639, CH), 1404 m | 47.0 | **Dfc** (3 months > 10) | **Dfb** ✘ / orotemperate | temperate, orotemperate | Jul = Aug = 13.9, Jun 11.9, **Sep 10.0** |
| Tmon10 boundary + boreal | Petrozavodsk (22820, RU) | 61.82 | **Dfc** | **Dfb** ✘ / orotemperate ✘ | **boreal**, thermoboreal (Tp 714) | **Sep 10.0**. RM step 13: Ic 25.5 (21–28), T 3.64 < 4.8, Tp 714 < 740, so boreal. |
| s/w precedence | Kunming (56778, CN) | 25.02 | **Cwb** | **Csb** ✘ / mesomediterranean ✘ | **tropical** (step 6, Pss > Psw), mesotropical (It 362) | s holds (Apr 25.4 < 40 and < 82.4/3); w holds (Dec/Feb 11.9 < 215.7/10). Ps 809.5 > Pw 181.5, so Beck gives w. |
| s/w precedence | Mexico City Tacubaya (76680, MX) | 19.40 | **Cwb** | **Csb** ✘ / thermomediterranean ✘ | **tropical** (lat < 23), mesotropical (It 465) | Ps 561.4 > Pw 93.8. |
| s/w precedence (SH) | Isoka (67481, ZM) | −10.17 | **Cwa** | **Csa** ✘ / thermomediterranean ✘ | tropical, thermotropical | Tcold 17.7 → C. Summer ONDJFM, Psdry 8.4 (Oct). |
| Half-year (A↔B) | Tema (65473, GH) | 5.62 | **Aw** | **BSh** ✘ / thermotemperate ✘ | tropical, infratropical | Warmer half = ONDJFM. Beck: winter (AMJJAS) gets 74 % → Pth = 2·27.61 = 55.2, 10·Pth = 552 < 708, so not B; Pdry 8.4 → Aw. Code: "summer" AMJJAS gets 74 % → Pth = 83.2, 10·Pth = 832 > 708, so B. |
| Half-year (s) | Botana (52055210, CO), 2820 m | 1.16 | **Cfb** | **Csb** ✘ / mesomediterranean ✘ | tropical, mesotropical (It 377) | Warmer half ONDJFM (12.75 vs 12.51 °C mean). Code uses AMJJAS, where Aug 39.6 < 40 and < 119.1/3. |
| **boreal** (Europe) | Sodankylä Tähtelä (02836, FI) | 67.37 | **Dfc** | Dfc / **oroboreal** ✘ | **boreal**, **supraboreal** (Tp 519) | RM step 13: Ic 27.5, T 0.33 < 4.8, Tp 519 < 740. |
| **mediterranean** (Europe) | Sevilla San Pablo (08391, ES) | 37.42 | **Csa** | Csa / thermomediterranean | mediterranean, thermomediterranean (Itc 416) | Ios2 0.06, Ios3 0.11, Ios4 0.42 ≤ 2 |
| mediterranean (Europe) | Roma Ciampino (16239, IT) | 41.81 | **Csa** | Csa / mesomediterranean | mediterranean, mesomediterranean (Itc 312) | Ios4 1.50 ≤ 2 |
| mediterranean (SH) | Perth Airport (94610, AU) | −31.93 | **Csa** | Csa / thermomediterranean | mediterranean, **inframediterranean** (It 455, just above 450) | SH summer quarter DJF used for Ios |
| B → should be med. | Almería (08487, ES) | 36.85 | **BSh** | BSh / **thermotemperate** ✘ | **mediterranean**, thermomediterranean (Itc 448) | Ios2 0.05 |
| B → should be med. | Madrid Retiro (08222, ES), 667 m | 40.41 | **BSk** | BSk / **mesotemperate** ✘ | **mediterranean**, mesomediterranean (Itc 296) | Ios2 0.37 |
| Ds → should be boreal | Palmer (70274, US-AK) | 61.60 | **Dsc** | Dsc / **oromediterranean** ✘ | **boreal**, mesoboreal (Tp 663) | Step 13: Ic 24.3, T 2.86 < 4.8, Tp 663 < 740 |
| **temperate** (Europe) | Potsdam (10379, DE) | 52.38 | **Cfb** | Cfb / supratemperate | temperate, supratemperate (It 111 < 120, so Tp 1175) | — |
| temperate (Europe) | Wien Hohe Warte (11035, AT) | 48.25 | **Cfb** | Cfb / supratemperate | temperate, supratemperate (Itc 151) | Tcold 1.1 > 0 |
| temperate, Dfb | Helsinki Kaisaniemi (02978, FI) | 60.18 | **Dfb** | Dfb / supratemperate | temperate (T 6.48 > 4.8), supratemperate (Tp 860) | — |
| temperate (SH) | Hobart (94970, AU) | −42.89 | **Cfb** | Cfb / mesotemperate ✘ | temperate, **thermotemperate** (It 314, Ic 8.9) | Shows the It-vs-Tp rule |

**D station with Thot ≥ 22 and Tcold < −38:** none exists in the WMO 1991–2020 collection of 4,540 complete stations. The 4 stations with Tcold < −38 are Oymyakon, Verkhoyansk, Shelagontsy and Ust-Maya, with maximum Thot 18.9 and Tmon10 = 3. Test the d-precedence bug (Part 1 #6) with synthetic data only. I did not search outside this collection, so a worldwide "none exists" is **unverified**. **Dwd:** no Dwd station exists in this collection either.

### 3.2 Go-ready station literals

Format: Tmin and Tmax are the mean daily min and max (WMO TMIN/TMAX). Precip is in mm. For the South Pole only Tmean is available; set P = 0 and Tmin = Tmax = Tmean, which is safe because Köppen ignores Tmin/Tmax and the thermotype uses Tp only when Tp = 0.

```go
{ // Amundsen-Scott South Pole, 1991–2020 mean of BAS READER monthly means (not an official WMO normal; no precip)
	name: "South Pole", lat: -90,
	tmean:  [12]float64{-28.1, -40.7, -53.6, -57.4, -57.7, -58.1, -60.2, -59.6, -58.9, -50.9, -37.2, -27.3},
	precip: [12]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	wantKoppen: "EF", wantMacro: "polar", wantThermotypeRM: "gelidpolar" /* RM: "gélido" */,
},
```

The other 31 stations were generated straight from the WMO9120 files: `wantKoppen` is the Beck18 reference result, and `wantMacro`/`wantThermotypeRM` come from my WBCS08 implementation. RM values marked "≈" are approximate (see §2.3 caveat), and I would not assert them in tests except for the clear-cut counter-examples.

```go
{ // WMO 6680 Saentis (Switzerland), elev 2501.0 m — Beck2018=ET, code-today=ET/cryoropolar, RM≈cryorotemperate [Tp=234]
	name: "Saentis", lat: 47.249,
	tmean:  [12]float64{-6.8, -7.5, -5.6, -3, 1, 4.5, 6.4, 6.8, 3.6, 1.1, -3.3, -5.9},
	tmin:   [12]float64{-9.3, -10, -8, -5.3, -1.3, 2.1, 4, 4.5, 1.3, -1.3, -5.7, -8.4},
	tmax:   [12]float64{-4.1, -4.8, -3.3, -1, 3.2, 6.9, 9.2, 9.6, 6.2, 3.7, -0.7, -3.2},
	precip: [12]float64{269.5, 221.9, 254.6, 176.2, 224, 242.8, 278.6, 266.3, 214.3, 193.4, 204.9, 293.6},
	wantKoppen: "ET", wantMacro: "temperate", wantThermotypeRM: "cryorotemperate",
},
{ // WMO 10961 Zugspitze (Germany), elev 2964.5 m — Beck2018=ET, code-today=ET/cryoropolar, RM≈cryorotemperate [Tp=92]
	name: "Zugspitze", lat: 47.421,
	tmean:  [12]float64{-10.1, -10.9, -9, -6.2, -2, 1.5, 3.4, 3.8, 0.5, -2, -6.3, -9.2},
	tmin:   [12]float64{-12.7, -13.5, -11.6, -8.7, -4.4, -0.9, 0.9, 1.4, -1.9, -4.5, -8.8, -11.8},
	tmax:   [12]float64{-7.3, -8, -6.3, -3.2, 0.9, 4.5, 6.5, 6.9, 3.3, 0.6, -3.6, -6.4},
	precip: [12]float64{176.5, 157.4, 208.1, 171.7, 192.7, 181.4, 184.5, 184.6, 150.2, 127.6, 157.4, 183.3},
	wantKoppen: "ET", wantMacro: "temperate", wantThermotypeRM: "cryorotemperate",
},
{ // WMO 71018 ResoluteCs (Canada), elev 30.0 m — Beck2018=ET, code-today=ET/cryoropolar, RM≈suprapolar [Tp=77]
	name: "ResoluteCs", lat: 74.716,
	tmean:  [12]float64{-31.4, -32.1, -29.4, -21.2, -10, 0.5, 4.8, 2.4, -3.7, -12.2, -21.5, -27.3},
	tmin:   [12]float64{-34.7, -35.4, -32.8, -24.7, -12.9, -1.7, 2, 0.2, -5.6, -15, -24.6, -30.7},
	tmax:   [12]float64{-28, -28.7, -26, -17.7, -7.1, 2.8, 7.6, 4.6, -1.7, -9.4, -18.3, -23.8},
	precip: [12]float64{4.9, 4.4, 7.4, 8.2, 9.5, 15.7, 29.3, 32.7, 19, 15, 16.4, 6.7},
	wantKoppen: "ET", wantMacro: "polar", wantThermotypeRM: "suprapolar",
},
{ // WMO 72613 MT_WASHINGTON (United_States), elev 1910.0 m — Beck2018=ET, code-today=Dfc/cryoroboreal, RM≈cryorotemperate [Tp=354]
	name: "MT_WASHINGTON", lat: 44.267,
	tmean:  [12]float64{-14.5, -14.5, -10.6, -4.6, 2.4, 7.5, 10, 9.3, 6.2, -0.4, -6.2, -11.2},
	tmin:   [12]float64{-19.6, -19.4, -15, -8.5, -1, 4.2, 7, 6.2, 2.9, -4, -10.5, -15.9},
	tmax:   [12]float64{-9.5, -9.5, -6.2, -0.7, 5.8, 10.8, 13, 12.4, 9.5, 3.2, -2, -6.6},
	precip: [12]float64{145.9, 138.3, 170.7, 185.6, 195, 218.2, 226.7, 196, 194.5, 253.6, 205.6, 186.7},
	wantKoppen: "ET", wantMacro: "temperate", wantThermotypeRM: "cryorotemperate",
},
{ // WMO 72202 MIAMI_INTL_AP (United_States), elev 9.0 m — Beck2018=Am, code-today=Am/thermotropical, RM≈thermotropical [It/Itc=659]
	name: "MIAMI_INTL_AP", lat: 25.791,
	tmean:  [12]float64{20.3, 21.5, 22.8, 24.8, 26.7, 28.2, 28.9, 29, 28.3, 26.7, 23.8, 21.8},
	tmin:   [12]float64{16.1, 17.3, 18.7, 21, 23, 24.6, 25.3, 25.4, 24.9, 23.4, 20.2, 17.9},
	tmax:   [12]float64{24.6, 25.7, 27, 28.7, 30.4, 31.8, 32.6, 32.6, 31.7, 30, 27.4, 25.6},
	precip: [12]float64{46.6, 54.5, 62.5, 85.5, 160.6, 266.9, 187, 243.2, 259.6, 194.3, 89.6, 62},
	wantKoppen: "Am", wantMacro: "tropical", wantThermotypeRM: "thermotropical",
},
{ // WMO 94287 CairnsAero (Australia), elev 2.0 m — Beck2018=Am, code-today=Am/thermotropical, RM≈thermotropical [It/Itc=689]
	name: "CairnsAero", lat: -16.874,
	tmean:  [12]float64{28, 27.9, 27.2, 25.9, 24.1, 22.6, 21.8, 22.3, 23.9, 25.5, 26.9, 27.8},
	tmin:   [12]float64{24, 24.1, 23.3, 21.9, 20.1, 18.5, 17.3, 17.4, 18.9, 20.9, 22.5, 23.7},
	tmax:   [12]float64{31.9, 31.6, 31.1, 29.8, 28.1, 26.7, 26.3, 27.2, 28.9, 30, 31.3, 32},
	precip: [12]float64{388.7, 475.4, 367, 178.6, 81.2, 42.2, 35.9, 26.6, 28.4, 63.3, 85.1, 184.6},
	wantKoppen: "Am", wantMacro: "tropical", wantThermotypeRM: "thermotropical",
},
{ // WMO 91165 LIHUE_WSO_AP (United_States), elev 31.0 m — Beck2018=Aw, code-today=As/thermotropical, RM≈thermotropical [It/Itc=692]
	name: "LIHUE_WSO_AP", lat: 21.984,
	tmean:  [12]float64{22.4, 22.3, 22.7, 23.7, 24.6, 25.7, 26.4, 26.8, 26.7, 26, 24.6, 23.3},
	tmin:   [12]float64{18.9, 19, 19.6, 20.8, 21.6, 23, 23.6, 24, 23.8, 23.1, 21.8, 20.3},
	tmax:   [12]float64{25.9, 25.6, 25.8, 26.5, 27.6, 28.5, 29.2, 29.6, 29.6, 28.8, 27.3, 26.2},
	precip: [12]float64{70.5, 92.1, 142.4, 51.6, 55.3, 45.5, 44.4, 59.2, 55.4, 83.2, 102.5, 117.8},
	wantKoppen: "Aw", wantMacro: "tropical", wantThermotypeRM: "thermotropical",
},
{ // WMO 82397 FORTALEZA (Brazil), elev 29.9 m — Beck2018=Aw, code-today=As/thermotropical, RM≈infratropical [It/Itc=804]
	name: "FORTALEZA", lat: -3.815,
	tmean:  [12]float64{27.4, 27.3, 27, 26.8, 26.8, 26.4, 26.2, 26.6, 27, 27.4, 27.7, 27.8},
	tmin:   [12]float64{24.5, 24.3, 23.9, 23.8, 23.8, 23.2, 22.8, 22.9, 23.6, 24.3, 24.7, 24.9},
	tmax:   [12]float64{31.2, 31.1, 30.8, 30.6, 30.8, 30.5, 30.6, 31.1, 31.4, 31.6, 31.7, 31.7},
	precip: [12]float64{156.4, 187, 336.9, 385, 229, 130, 69.7, 20, 13.6, 9.5, 9.8, 37.1},
	wantKoppen: "Aw", wantMacro: "tropical", wantThermotypeRM: "infratropical",
},
{ // WMO 94120 DarwinAirport (Australia), elev 30.0 m — Beck2018=Aw, code-today=Aw/thermotropical, RM≈infratropical [It/Itc=783]
	name: "DarwinAirport", lat: -12.424,
	tmean:  [12]float64{28.5, 28.3, 28.5, 28.6, 27.3, 25.6, 25.2, 25.9, 28, 29.3, 29.6, 29.3},
	tmin:   [12]float64{25, 25, 24.8, 24.2, 22.3, 20, 19.3, 19.8, 22.9, 24.8, 25.4, 25.5},
	tmax:   [12]float64{32, 31.7, 32.2, 33, 32.3, 31.1, 31.2, 31.9, 33.1, 33.8, 33.8, 33},
	precip: [12]float64{470.7, 412.4, 313.7, 105.1, 20.7, 2.2, 0.9, 0.8, 14.3, 68.9, 143.5, 279.3},
	wantKoppen: "Aw", wantMacro: "tropical", wantThermotypeRM: "infratropical",
},
{ // WMO 48455 BangkokMetropolis (Thailand), elev 3.0 m — Beck2018=Aw, code-today=Aw/thermotropical, RM≈infratropical [It/Itc=844]
	name: "BangkokMetropolis", lat: 13.726,
	tmean:  [12]float64{27.4, 28.6, 29.7, 30.7, 30.3, 29.7, 29.2, 29, 28.6, 28.4, 28.4, 27.3},
	tmin:   [12]float64{23.4, 24.8, 26.4, 27.2, 26.9, 26.4, 26.1, 25.9, 25.4, 25.2, 24.7, 23.2},
	tmax:   [12]float64{32.7, 33.7, 34.7, 35.7, 35.1, 34.1, 33.5, 33.3, 33.2, 33, 33.1, 32.3},
	precip: [12]float64{23.6, 21.4, 51, 93.3, 216.8, 198.5, 189.7, 227.1, 335.9, 288.7, 44.6, 11.6},
	wantKoppen: "Aw", wantMacro: "tropical", wantThermotypeRM: "infratropical",
},
{ // WMO 24688 Ojmjakon (Russian_Federation_(In_Asia)), elev 740.0 m — Beck2018=Dfd, code-today=Dfd/oroboreal, RM≈oroboreal [Tp=449]
	name: "Ojmjakon", lat: 63.25,
	tmean:  [12]float64{-45.7, -42.2, -30.2, -12.7, 3.6, 12.7, 15.3, 10.8, 2.5, -13.8, -34.4, -44.9},
	tmin:   [12]float64{-49.3, -47.6, -39.2, -22.9, -3.4, 4.4, 7, 3.2, -3.3, -19.2, -38.8, -48.2},
	tmax:   [12]float64{-42.1, -35.8, -19.8, -3, 9.8, 20, 23, 18.6, 9.1, -8.3, -30.2, -41.9},
	precip: [12]float64{6.3, 6.7, 5.3, 5.4, 15, 38.5, 46.5, 39.2, 24.5, 13.4, 12.6, 7.1},
	wantKoppen: "Dfd", wantMacro: "boreal", wantThermotypeRM: "oroboreal",
},
{ // WMO 24966 UstMajya (Russian_Federation_(In_Asia)), elev 169.0 m — Beck2018=Dfd, code-today=Dfd/oroboreal, RM≈mesoboreal [Tp=648]
	name: "UstMajya", lat: 60.383,
	tmean:  [12]float64{-38.3, -33.7, -19, -3.1, 8.1, 16.3, 18.9, 15, 6.5, -6.5, -25.9, -38.2},
	tmin:   [12]float64{-41.8, -39.3, -27.9, -10.4, 1.8, 9.2, 12.5, 8.9, 1.8, -10.2, -29.7, -41.2},
	tmax:   [12]float64{-34.3, -26.6, -9.9, 3.9, 14.8, 23.5, 25.8, 22.1, 12.6, -2.2, -21.4, -34.9},
	precip: [12]float64{11.8, 8.4, 8.1, 10.9, 28.4, 34.5, 54.1, 50.8, 44.5, 29.4, 19.5, 11.2},
	wantKoppen: "Dfd", wantMacro: "boreal", wantThermotypeRM: "mesoboreal",
},
{ // WMO 24266 Verhojansk (Russian_Federation_(In_Asia)), elev 136.0 m — Beck2018=Dsd, code-today=Dsd/oromediterranean, RM≈supraboreal [Tp=499]
	name: "Verhojansk", lat: 67.55,
	tmean:  [12]float64{-44.7, -42.2, -28.8, -10.8, 4.2, 14.2, 16.7, 12.1, 2.7, -13.6, -33.7, -43.4},
	tmin:   [12]float64{-47.7, -46.4, -37.3, -20.3, -2, 7.5, 10.1, 5.7, -2, -18.1, -37.2, -46.2},
	tmax:   [12]float64{-41.6, -36.9, -18.6, -1.7, 10.4, 21, 23.7, 19.3, 8.7, -8.6, -30, -40.4},
	precip: [12]float64{5.9, 5.3, 4.6, 4, 15.7, 30.1, 34.7, 29.4, 22.1, 12.8, 11.1, 6.1},
	wantKoppen: "Dsd", wantMacro: "boreal", wantThermotypeRM: "supraboreal",
},
{ // WMO 6639 Napf (Switzerland), elev 1404.0 m — Beck2018=Dfc, code-today=Dfb/orotemperate, RM≈orotemperate [Tp=722]
	name: "Napf", lat: 47.005,
	tmean:  [12]float64{-1.3, -1.6, 1, 4.2, 8.3, 11.9, 13.9, 13.9, 10, 6.8, 2.2, -0.4},
	tmin:   [12]float64{-3.8, -4.1, -1.7, 1.2, 5, 8.6, 10.6, 10.9, 7.4, 4.3, -0.1, -2.8},
	tmax:   [12]float64{1.3, 1, 3.8, 7.7, 12.1, 15.9, 18, 17.6, 13.1, 9.7, 4.8, 2.2},
	precip: [12]float64{98.4, 98.3, 114.1, 125.5, 187.8, 178, 188.2, 175.6, 131.1, 118.2, 109.7, 123.9},
	wantKoppen: "Dfc", wantMacro: "temperate", wantThermotypeRM: "orotemperate",
},
{ // WMO 22820 Petrozavodsk (Russian_Federation_(In_Europe)), elev 110.0 m — Beck2018=Dfc, code-today=Dfb/orotemperate, RM≈thermoboreal [Tp=714]
	name: "Petrozavodsk", lat: 61.817,
	tmean:  [12]float64{-8.4, -8.2, -3.5, 2.5, 8.9, 14.1, 17.1, 15, 10, 3.8, -1.9, -5.7},
	tmin:   [12]float64{-11.5, -11.5, -6.8, -1.3, 4, 9.3, 12.8, 11.1, 6.8, 1.5, -4.2, -8.3},
	tmax:   [12]float64{-5.6, -5.2, 0, 6.7, 13.8, 18.8, 21.6, 19.3, 13.6, 6.4, 0.2, -3.4},
	precip: [12]float64{37.9, 28.3, 31.6, 32, 47.7, 61.1, 80.2, 79.5, 59.2, 53.6, 50.1, 43.6},
	wantKoppen: "Dfc", wantMacro: "boreal", wantThermotypeRM: "thermoboreal",
},
{ // WMO 56778 KUNMING (China), elev 1892.0 m — Beck2018=Cwb, code-today=Csb/mesomediterranean, RM≈mesotropical [It/Itc=362]
	name: "KUNMING", lat: 25.02,
	tmean:  [12]float64{9.3, 11.5, 14.8, 17.8, 19.6, 20.7, 20.5, 20.2, 18.8, 16.2, 12.5, 9.5},
	tmin:   [12]float64{4, 5.8, 8.9, 12, 14.9, 17.3, 17.6, 17.2, 15.7, 12.9, 8.2, 4.8},
	tmax:   [12]float64{16.3, 18.5, 21.8, 24.3, 25.2, 25.4, 24.8, 25, 23.5, 21.1, 18.7, 16},
	precip: [12]float64{23.8, 11.9, 19.6, 25.4, 80.1, 173.1, 215.7, 195.9, 119.3, 82.4, 30.1, 13.7},
	wantKoppen: "Cwb", wantMacro: "tropical", wantThermotypeRM: "mesotropical",
},
{ // WMO 76680 Tacubaya (Mexico), elev 2309.0 m — Beck2018=Cwb, code-today=Csb/thermomediterranean, RM≈mesotropical [It/Itc=465]
	name: "Tacubaya", lat: 19.404,
	tmean:  [12]float64{15.1, 16.9, 18.7, 19.9, 19.9, 18.9, 17.8, 18, 17.3, 17, 16.3, 15.6},
	tmin:   [12]float64{8.1, 9.7, 11.2, 13.1, 13.7, 13.9, 13.1, 13.2, 13.3, 12.3, 10, 8.5},
	tmax:   [12]float64{20.8, 23.2, 25.1, 26.6, 26.5, 24.9, 23.7, 23.8, 23, 22.5, 21.8, 21.2},
	precip: [12]float64{4.8, 5.2, 9.6, 16.9, 45.9, 108.9, 133.2, 132.1, 124.4, 57.2, 14.7, 2.3},
	wantKoppen: "Cwb", wantMacro: "tropical", wantThermotypeRM: "mesotropical",
},
{ // WMO 67481 Isoka (Zambia), elev 1360.0 m — Beck2018=Cwa, code-today=Csa/thermomediterranean, RM≈thermotropical [It/Itc=568]
	name: "Isoka", lat: -10.167,
	tmean:  [12]float64{22.3, 22.2, 22.1, 21.3, 19.6, 18.1, 17.7, 19.8, 22.4, 24.4, 24.6, 22.9},
	tmin:   [12]float64{17.1, 17.1, 16.9, 15.8, 12.9, 10.7, 10, 11.6, 13.9, 16.3, 17.5, 17.3},
	tmax:   [12]float64{27.5, 27.3, 27.3, 26.7, 26.3, 25.5, 25.4, 27.9, 30.8, 32.5, 31.6, 28.5},
	precip: [12]float64{231.2, 223.8, 171.4, 71.8, 7.1, 0.2, 0.2, 0.1, 0.1, 8.4, 68.4, 218},
	wantKoppen: "Cwa", wantMacro: "tropical", wantThermotypeRM: "thermotropical",
},
{ // WMO 65473 Tema (Ghana), elev 14.0 m — Beck2018=Aw, code-today=BSh/thermotemperate, RM≈infratropical [It/Itc=781]
	name: "Tema", lat: 5.617,
	tmean:  [12]float64{28.1, 28.9, 29, 28.8, 28.2, 26.9, 25.8, 25.3, 26.2, 27.3, 28.3, 28.5},
	tmin:   [12]float64{25, 26, 26.2, 26, 25.5, 24.5, 23.6, 23, 23.8, 24.5, 25.4, 25.5},
	tmax:   [12]float64{31.2, 31.8, 31.8, 31.6, 31, 29.3, 27.9, 27.5, 28.6, 30.1, 31.3, 31.5},
	precip: [12]float64{8.4, 22, 51.3, 89, 141.3, 183.9, 59.6, 15, 35.5, 62.1, 25.2, 14.7},
	wantKoppen: "Aw", wantMacro: "tropical", wantThermotypeRM: "infratropical",
},
{ // WMO 52055210 Botana__AUT_ (Colombia), elev 2820.0 m — Beck2018=Cfb, code-today=Csb/mesomediterranean, RM≈mesotropical [It/Itc=377]
	name: "Botana__AUT_", lat: 1.16,
	tmean:  [12]float64{12.58, 12.82, 12.96, 13.05, 13.09, 12.53, 12.01, 11.97, 12.42, 12.76, 12.73, 12.64},
	tmin:   [12]float64{8.75, 9.08, 9.41, 9.46, 9.5, 9.38, 8.91, 8.58, 8.37, 8.66, 8.88, 9.01},
	tmax:   [12]float64{17.39, 17.63, 17.66, 17.77, 17.68, 16.88, 16.33, 16.44, 17.34, 17.92, 17.7, 17.39},
	precip: [12]float64{86.31, 73.2, 93.52, 102.58, 90.98, 67.37, 58.28, 39.58, 42.99, 83.49, 119.11, 96.25},
	wantKoppen: "Cfb", wantMacro: "tropical", wantThermotypeRM: "mesotropical",
},
{ // WMO 2836 Sodankyla_Tahtela (Finland), elev 179.0 m — Beck2018=Dfc, code-today=Dfc/oroboreal, RM≈supraboreal [Tp=519]
	name: "Sodankyla_Tahtela", lat: 67.367,
	tmean:  [12]float64{-12.5, -12.1, -7.1, -0.8, 5.6, 11.9, 15, 12.4, 7, 0, -5.8, -9.6},
	tmin:   [12]float64{-17.5, -17.2, -12.9, -6, 0.7, 6.9, 10.1, 7.8, 3.3, -2.8, -9.2, -14},
	tmax:   [12]float64{-8.1, -7.6, -1.8, 4, 10.5, 16.9, 20.2, 17.4, 11.1, 2.9, -2.8, -5.7},
	precip: [12]float64{36.1, 30.8, 30, 31.6, 40.1, 60.8, 75.5, 55.8, 51.7, 47.4, 42.4, 40.9},
	wantKoppen: "Dfc", wantMacro: "boreal", wantThermotypeRM: "supraboreal",
},
{ // WMO 8391 SEVILLASANPABLO (Spain), elev 34.0 m — Beck2018=Csa, code-today=Csa/thermomediterranean, RM≈thermomediterranean [It/Itc=416]
	name: "SEVILLASANPABLO", lat: 37.417,
	tmean:  [12]float64{11.2, 12.6, 15.5, 17.6, 21.3, 25.3, 28, 28.1, 24.7, 20.4, 15.1, 12.1},
	tmin:   [12]float64{6.1, 7, 9.5, 11.5, 14.7, 18.2, 20.4, 20.6, 18.2, 14.7, 10, 7.4},
	tmax:   [12]float64{16.2, 18.2, 21.5, 23.7, 27.8, 32.4, 35.7, 35.6, 31.1, 26, 20.1, 16.8},
	precip: [12]float64{56.3, 46.2, 46.9, 54, 33.9, 5.8, 0.6, 2.5, 33.1, 75.4, 72.2, 77.2},
	wantKoppen: "Csa", wantMacro: "mediterranean", wantThermotypeRM: "thermomediterranean",
},
{ // WMO 16239 RomaCiampino (Italy), elev 129.0 m — Beck2018=Csa, code-today=Csa/mesomediterranean, RM≈mesomediterranean [It/Itc=312]
	name: "RomaCiampino", lat: 41.808,
	tmean:  [12]float64{7.47, 8, 10.7, 13.6, 17.96, 22.48, 25.11, 25.43, 21.02, 16.97, 12.42, 8.52},
	tmin:   [12]float64{3.42, 3.37, 5.86, 8.63, 12.56, 16.74, 19.26, 19.77, 15.99, 12.4, 8.47, 4.67},
	tmax:   [12]float64{12, 12.95, 15.75, 18.81, 23.33, 28.1, 31.02, 31.55, 26.69, 22.16, 16.87, 12.73},
	precip: [12]float64{65.62, 62.76, 58.58, 68.63, 56.85, 30.08, 19.79, 30.2, 64.86, 88.11, 108.24, 98.31},
	wantKoppen: "Csa", wantMacro: "mediterranean", wantThermotypeRM: "mesomediterranean",
},
{ // WMO 94610 PerthAirport (Australia), elev 15.0 m — Beck2018=Csa, code-today=Csa/thermomediterranean, RM≈inframediterranean [It/Itc=455]
	name: "PerthAirport", lat: -31.927,
	tmean:  [12]float64{24.9, 25, 23.1, 19.8, 16.4, 14.2, 13.3, 13.7, 14.8, 17.2, 20.2, 22.9},
	tmin:   [12]float64{17.4, 17.6, 16, 13.3, 10.4, 8.9, 8.1, 8.1, 8.9, 10.6, 13.3, 15.5},
	tmax:   [12]float64{32.3, 32.4, 30.1, 26.4, 22.5, 19.6, 18.6, 19.2, 20.7, 23.8, 27.2, 30.2},
	precip: [12]float64{15.8, 16.6, 17.9, 30.7, 81.4, 123.9, 139.6, 122.1, 78.9, 33.6, 28.7, 9.6},
	wantKoppen: "Csa", wantMacro: "mediterranean", wantThermotypeRM: "inframediterranean",
},
{ // WMO 8487 ALMERIAAEROPUERTO (Spain), elev 21.0 m — Beck2018=BSh, code-today=BSh/thermotemperate, RM≈thermomediterranean [It/Itc=448]
	name: "ALMERIAAEROPUERTO", lat: 36.846,
	tmean:  [12]float64{12.8, 13.3, 15.1, 17, 19.9, 23.4, 26.1, 26.9, 24, 20.4, 16.2, 13.8},
	tmin:   [12]float64{8.7, 9.2, 10.8, 12.8, 15.5, 18.9, 21.8, 22.6, 19.8, 16.3, 12.2, 9.7},
	tmax:   [12]float64{17, 17.5, 19.4, 21.3, 24.3, 27.8, 30.5, 31.2, 28.2, 24.5, 20.3, 17.9},
	precip: [12]float64{20.8, 23.3, 20.7, 15.2, 10.9, 5.5, 0.6, 2.3, 16.1, 25.2, 25.1, 31.8},
	wantKoppen: "BSh", wantMacro: "mediterranean", wantThermotypeRM: "thermomediterranean",
},
{ // WMO 8222 MADRIDRETIRO (Spain), elev 667.0 m — Beck2018=BSk, code-today=BSk/mesotemperate, RM≈mesomediterranean [It/Itc=296]
	name: "MADRIDRETIRO", lat: 40.412,
	tmean:  [12]float64{6.6, 8, 11.4, 13.7, 17.7, 23, 26.4, 25.9, 21.2, 15.5, 10, 7.1},
	tmin:   [12]float64{3, 3.7, 6.3, 8.2, 11.9, 16.5, 19.5, 19.3, 15.5, 11.1, 6.4, 3.7},
	tmax:   [12]float64{10.2, 12.3, 16.4, 19.1, 23.5, 29.4, 33.3, 32.6, 26.9, 20, 13.7, 10.4},
	precip: [12]float64{32, 33.5, 35.3, 46.1, 48.3, 19.6, 9.2, 10.4, 23.6, 64.2, 52.2, 42.4},
	wantKoppen: "BSk", wantMacro: "mediterranean", wantThermotypeRM: "mesomediterranean",
},
{ // WMO 70274 PALMER_MUNI_AP (United_States), elev 70.0 m — Beck2018=Dsc, code-today=Dsc/oromediterranean, RM≈mesoboreal [Tp=663]
	name: "PALMER_MUNI_AP", lat: 61.596,
	tmean:  [12]float64{-9.2, -6.2, -3.6, 3.8, 9.4, 13.5, 15.1, 13.5, 9.1, 1.9, -5.4, -7.6},
	tmin:   [12]float64{-13.2, -10.4, -8.2, -1.2, 3.8, 8.3, 10.4, 9, 4.9, -1.9, -9.1, -11.4},
	tmax:   [12]float64{-5.3, -2, 1.1, 8.8, 15.1, 18.7, 19.8, 18.1, 13.2, 5.7, -1.7, -3.8},
	precip: [12]float64{16.4, 17.7, 12, 7.5, 15.8, 25.3, 40.6, 58.9, 52.8, 31.8, 16.8, 24.3},
	wantKoppen: "Dsc", wantMacro: "boreal", wantThermotypeRM: "mesoboreal",
},
{ // WMO 10379 Potsdam (Germany), elev 80.9 m — Beck2018=Cfb, code-today=Cfb/supratemperate, RM≈supratemperate [Tp=1175]
	name: "Potsdam", lat: 52.381,
	tmean:  [12]float64{0.7, 1.6, 4.7, 9.9, 14.2, 17.4, 19.4, 18.9, 14.6, 9.6, 4.8, 1.7},
	tmin:   [12]float64{-1.9, -1.4, 0.9, 4.7, 8.8, 12.1, 14.4, 14.1, 10.4, 6.2, 2.2, -0.7},
	tmax:   [12]float64{3.2, 4.9, 9.2, 15.7, 20, 23, 25.1, 24.9, 19.9, 13.7, 7.6, 3.9},
	precip: [12]float64{45.3, 36.1, 39.3, 29.2, 53.3, 60.8, 76.2, 59.2, 47.1, 42.8, 42.3, 46.1},
	wantKoppen: "Cfb", wantMacro: "temperate", wantThermotypeRM: "supratemperate",
},
{ // WMO 11035 Wien_Hohe_Warte (Austria), elev 198.0 m — Beck2018=Cfb, code-today=Cfb/supratemperate, RM≈supratemperate [It/Itc=151]
	name: "Wien_Hohe_Warte", lat: 48.249,
	tmean:  [12]float64{1.1, 2.8, 6.9, 11.9, 16.3, 20, 21.9, 21.6, 16.6, 11.2, 6.2, 1.8},
	tmin:   [12]float64{-1.3, -0.5, 2.6, 6.7, 10.7, 14.7, 15.9, 15.6, 12, 7.3, 3.7, -0.4},
	tmax:   [12]float64{3.5, 6.5, 10.7, 17.2, 20.7, 25.1, 26.4, 26.1, 21.1, 14.3, 8.8, 4},
	precip: [12]float64{42.1, 38.1, 51.6, 41.8, 78.9, 70, 77.7, 69.1, 64.1, 46.9, 46, 46.8},
	wantKoppen: "Cfb", wantMacro: "temperate", wantThermotypeRM: "supratemperate",
},
{ // WMO 2978 HelsinkiKaisaniemi (Finland), elev 3.0 m — Beck2018=Dfb, code-today=Dfb/supratemperate, RM≈supratemperate [Tp=860]
	name: "HelsinkiKaisaniemi", lat: 60.175,
	tmean:  [12]float64{-3.1, -3.8, -0.7, 4.4, 10.4, 14.9, 18.1, 16.9, 12.3, 6.6, 2.4, -0.7},
	tmin:   [12]float64{-5.6, -6.3, -3.6, 1.1, 6.4, 11.2, 14.5, 13.5, 9.3, 4.2, 0.4, -2.9},
	tmax:   [12]float64{-0.7, -1.3, 2.3, 8.1, 14.6, 18.8, 21.9, 20.5, 15.4, 9.2, 4.4, 1.4},
	precip: [12]float64{52.8, 38.3, 34.4, 34.1, 37.9, 60.1, 57.1, 81.4, 55.7, 73.3, 69.3, 58.3},
	wantKoppen: "Dfb", wantMacro: "temperate", wantThermotypeRM: "supratemperate",
},
{ // WMO 94970 Hobart (Australia), elev 51.0 m — Beck2018=Cfb, code-today=Cfb/mesotemperate, RM≈thermotemperate [It/Itc=314]
	name: "Hobart", lat: -42.89,
	tmean:  [12]float64{17.9, 17.6, 16.2, 13.8, 11.6, 9.2, 9, 9.8, 11.4, 13.1, 14.7, 16.4},
	tmin:   [12]float64{12.9, 12.8, 11.6, 9.4, 7.6, 5.5, 5.2, 5.6, 7, 8.4, 10, 11.6},
	tmax:   [12]float64{22.8, 22.3, 20.8, 18.2, 15.6, 13, 12.8, 13.9, 15.9, 17.8, 19.4, 21.2},
	precip: [12]float64{43.7, 37.8, 37, 42.6, 39.2, 46, 44.5, 63, 55.6, 52.8, 49.4, 53.7},
	wantKoppen: "Cfb", wantMacro: "temperate", wantThermotypeRM: "thermotemperate",
},
```

**Recommended test assertions** (high confidence; each is far from any threshold except the ones deliberately chosen as boundaries):
- `KoppenGeiger`: every row above.
  - Boundary rows that pin the deviations: Mt Washington (Thot = 10.0 → ET), Napf and Petrozavodsk (Tmon10 with Sep = 10.0 → Dfc), Kunming, Tacubaya and Isoka (s/w precedence → Cw), Tema (half-year → Aw), Lihue and Fortaleza (→ Aw, no As).
  - Verkhoyansk (Dsd) and Ust-Maya (Tcold −38.3) are knife-edge cases. Keep them as regression pins only.
- Macrobioclimate: Säntis/Zugspitze → temperate; Resolute → polar; Sodankylä/Petrozavodsk/Palmer/Verkhoyansk → boreal; Sevilla/Roma/Perth/Almería/Madrid → mediterranean; Kunming/Tacubaya/Botana/Isoka/Tema/Darwin/Bangkok/Miami/Cairns → tropical; Potsdam/Wien/Helsinki/Hobart → temperate.
- Thermotype: South Pole → gelid; Resolute → suprapolar; Sodankylä → supraboreal; Darwin/Bangkok → infratropical; Sevilla → thermomediterranean; Roma → mesomediterranean; Potsdam → supratemperate; Hobart → thermotemperate.

---

## Deviations found (code vs. source)

| # | Where | Source quote and URL | Code | Effect / evidence |
|---|---|---|---|---|
| D1 | `koppen.go` `case thot < 10` | Beck18 Table 2: E "Not (B) & Thot≤10"; C and D "Thot>10" (PMC6207062 XML, Table 2) | `thot < 10` | Thot = 10.0 → C/D instead of E. Mt Washington ET→**Dfc**; Nagqu ET→**Dwc**. (Peel07 Table 1 has "Thot <10", which leaves Thot = 10 in no class; Beck18 fixed this.) |
| D2 | `koppenA` As branch | Beck18 Table 2 lists only "f", "m", "w" under A; w: "Not (Af) & Pdry<100-MAP/25". As exists only in Kottek06 Table 1 (p. 260: "As … Pmin < 60 mm in summer"). | Returns "As" when argmin P is in the summer half | 40/684 A stations become As instead of Aw (Lihue, Fortaleza, Natal, Acapulco, Sri Lanka …). The code's own As rule also differs from Kottek06's. |
| D3 | `seasonLetter` precedence | Beck18 Methods: "made mutually exclusive by assigning s when more precipitation falls in winter than in summer and assigning w otherwise." Kottek06 Table 1: Cs needs "Psmin < Pwmin"; Cw needs "Pwmin < Psmin". | s tested first unconditionally | 13 stations: Kunming, Tacubaya, Dali, Mpika Cwb→Csb; Isoka, Kabwe, Lundazi, Mindat, Morelia, Guanajuato, Ciudad Guzmán, Mackay Cwa→Csa; Ust-Barguzin Dwc→Dsc. These then become RM "mediterranean". |
| D4 | `tmon10` counting | Beck18 footnote: "Tmon10 = the number of months with air temperature >10 °C". Peel07: "number of months where the temperature is above 10". | `if t >= 10` (this is Kottek06's "Tmon ≥ +10 °C") | 10 stations Dfc→Dfb: Napf, Ulrichen, La Brévine (MeteoSwiss), Petrozavodsk, Kuopio-Maaninka, Seinäjoki, Edsbyn, Östmark, Fjærland, Leusi. Petrozavodsk then becomes RM "temperate" instead of boreal. |
| D5 | `heatLetter` d first | Beck18 and Peel07: d "Not (a or b) & Tcold<-38"; Kottek06 Table 2 (p. 262): "d … like (c) but Tmin ≤ −38 °C", where c is "not (b)". | `if allowD && tcold < -38 { return "d" }` before a/b | Synthetic only: Tcold < −38 with Thot ≥ 22 or Tmon10 ≥ 4 gives d; the source gives a or b. No such station among the WMO 1991–2020 normals (max Thot 18.9, Tmon10 3 among Tcold < −38). |
| D6 | `halfYears(lat)` | Beck18 footnote: "Summer (winter) is the six-month period that is warmer (colder) between April-September and October-March." Peel07 Table 1 footnote says the same. | Hemisphere by sign of latitude (as in Kottek06: "half-years on the hemisphere considered") | 10 stations, all near-equatorial: Tema, Lomé Aw→**BSh** (via Pthreshold); Colombian Andes Cfb↔Csb. It also affects As/Aw and Pthreshold for every station where the two definitions differ. |
| D7 | Pthreshold `>=` | Beck18 footnote: "if **>70%** of precipitation falls in winter … if >70% … in summer" | `pWinter >= 0.7*mapp`, `pSummer >= 0.7*mapp` | Only at exactly 70.0 %. Kottek06 uses "at least 2/3" instead, a different number. |
| D8 | `macrobioclimateSuffix` (whole approach) | WBCS08 key 5.1 (ch. 5) and ch. 3 definitions. Macrobioclimate depends on latitude band (Tropical "entre los paralelos 23º N y S"; Polar "51º a 90º N y 53º a 90ºS"; Boreal "43º a 71º N"), altitude reduction to 200 m, Ic/T/Tp thresholds, and summer drought "Ios2 > 2 … Ios3 … Ios4". | Derived from the Köppen letters | 1,197/4,339 stations (27.6 %) disagree. Clear counter-examples: Säntis and Zugspitze (code polar; RM temperate); Almería and Madrid (code temperate; RM mediterranean); Tamanrasset (code temperate; RM tropical); Kunming, Tacubaya, Isoka (code mediterranean; RM tropical); Verkhoyansk and Palmer AK (code mediterranean; RM boreal); Petrozavodsk with Beck-correct Dfc (code would say boreal, RM agrees) vs Helsinki Dfb (temperate both). |
| D9 | `thermotypeHorizon` single ladder | WBCS08 Fig. 19 gives separate ladders. Temperate thermo 2000–2350 / meso 1400–2000 / supra 800–1400 / oro 380–800 / cryoro 1–380 matches the code. Mediterranean is 2100/1500/900/450; tropical 2300/1700/950/450; boreal 680/580/480/380; polar 280/100/1. Source: "los termoclimáticos (It, Itc, Tp) difieren algo en la mayoría de los macrobioclimas". | Temperate Tp ladder for all | Boreal sites always "oro-" (Sodankylä: RM supraboreal; Ust-Maya: RM mesoboreal). Polar names invalid ("cryoropolar", "oropolar"; RM polar has only "termo-, meso-, supra- y gélido"). |
| D10 | Tp always used | WBCS08 ch. 3: "cuando el índice de termicidad (It) es inferior a 120 o cuando el índice de continentalidad (Ic) es igual o superior a 21, para calcular el termotipo se utiliza … Tp". Otherwise It/Itc applies (Fig. 19 It column, Fig. 7 Itc compensation). | Tp only; `ThermicityIndex` exists but is not used for thermotypes | Hobart It 314 → RM thermotemperate, code mesotemperate. Perth It 455 → RM inframediterranean, code thermo. |
| D11 | Missing infra- and gelid | WBCS08 Fig. 19: infratropical > 2900 (It > 710), inframediterranean > 2400, infratemperate > 2350 (It > 410); "Gélido … 0" in every macrobioclimate | Tp > 2000 → "thermo"; Tp ≤ 380 (including 0) → "cryoro" | Darwin, Bangkok, Fortaleza → RM infratropical; South Pole Tp = 0 → RM gélido, code "cryoropolar". |
| D12 | `thermotype.go` source comment | Cites "Rivas-Martínez 2011; boundaries as tabulated in 'Bioclimate of Italy'" | — | **Unverified**: I could not fetch the 2011 Global Geobotany text or the Italy paper (bot challenges). WBCS08 matches the code's temperate numbers. |

Matches confirmed (no deviation):
- A: Tcold ≥ 18; Af Pdry ≥ 60; Am Pdry ≥ 100 − MAP/25
- B precedence over E (Beck18)
- BW < 5·Pth; h MAT ≥ 18
- C/D at 0 °C (`> 0` vs `≤ 0`)
- s criterion "Psdry<40 & Psdry<Pwwet/3"; w criterion "Pwdry<Pswet/10"
- a Thot ≥ 22; b Tmon10 ≥ 4 (count threshold); d `< -38` (strict)
- ET/EF at Thot > 0
- Tp = 10·Σ(T > 0)
- It = (T + M + m)·10 (WBCS08 ch. 2: "It : índice de termicidad (T + M + m) 10")

## Unverified / not obtainable

- RM2011 (*Global Geobotany* 1:1–634) itself. All RM thresholds above come from the 2008 online WBCS edition (Wayback 2013 snapshots). The 2011 print edition may differ in detail.
- Any worldwide claim that no Dxa/Dxb station with Tcold < −38 exists. I checked only the 4,540 complete WMO 1991–2020 stations.
- Beck-map pixel values for the stations. I did not sample the Beck GeoTIFF; all expected classes are **computed** from station normals with the Beck18 criteria. The Beck map (1980–2016, 1 km gridded) may differ at borderline stations.
- RM classes are from my implementation of key 5.1. It simplifies Iosc (no Fig. 6 Io-dependent table), skips step 9 (Pcm3 < Pcm1 > Pcm2), and uses It instead of Itc in step 4. Clear-cut cases are listed above; borderline RM results (e.g. Perth inframediterranean at It 455 vs 450) should not be asserted.
- The South Pole series is my 1991–2020 average of BAS READER monthly means, not an official WMO normal. It has no precipitation.
- Possible data-quality issues in WMO9120 for some stations (e.g. Calabar, Ikeja). Do not use them.
