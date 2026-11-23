# Changelog

## [0.27.1](https://github.com/jobrunner/tempus/compare/v0.27.0...v0.27.1) (2026-11-23)


### Bug Fixes

* **reanalysis:** cancel left-behind lookups and reads when a read returns ([bd10b85](https://github.com/jobrunner/tempus/commit/bd10b85b355cc0313ddd258e81bd06649d30535a))
* **reanalysis:** collect round 1 in priority order and never publish a superseded listing ([dbf1640](https://github.com/jobrunner/tempus/commit/dbf164095029f84be1b729ea5ed34c49391b7b70))
* **reanalysis:** plan from cached months, fail only on needed models, stop background with Run ([88659d0](https://github.com/jobrunner/tempus/commit/88659d069a96c73c0f4ef85d02665808c2898100))
* **reanalysis:** wait only for needed coverage answers, cancel refresh listings, copy the root variable ([4336e9f](https://github.com/jobrunner/tempus/commit/4336e9f5c08568e5ca849a54be98e9d0900c4069))


### Performance Improvements

* **reanalysis:** read every model and variable in one round and keep bucket metadata warm ([027523d](https://github.com/jobrunner/tempus/commit/027523d3b923eabd6afc16b3af618d11cf16aa7c))

## [0.27.0](https://github.com/jobrunner/tempus/compare/v0.26.0...v0.27.0) (2026-11-22)


### Features

* **cache:** cap the cache in bytes and evict least recently used entries; new `TEMPUS_CACHE_MAX_BYTES`, default 256 MiB ([#104](https://github.com/jobrunner/tempus/pull/104))


### Bug Fixes

* **astronomy:** return sun and moon events for the local day of the instant, not its UTC calendar day ([#106](https://github.com/jobrunner/tempus/pull/106)), closes [#102](https://github.com/jobrunner/tempus/issues/102)
* **cache:** cache archive reads per model cell and key results on the exact point; nearby points no longer get each other's feature ([#105](https://github.com/jobrunner/tempus/pull/105))
* **deps:** patch x/net and the runtime image's Alpine packages ([#108](https://github.com/jobrunner/tempus/pull/108)), closes [#99](https://github.com/jobrunner/tempus/issues/99)

## [0.26.0](https://github.com/jobrunner/tempus/compare/v0.25.1...v0.26.0) (2026-11-21)


### ⚠ BREAKING CHANGES

* elevation (metres above sea level) is now required on GET /api/v1/query and on every batch point; requests without it get 400. Config keys providers.openmeteo.* are replaced by providers.reanalysis.*.

### Features

* **omfile:** add a clean-room decoder for Open-Meteo OM files ([c0f1a56](https://github.com/jobrunner/tempus/commit/c0f1a5677669c8a84c8ff30ff38ac902834e0f10))
* read weather from the reanalysis open data and require elevation ([1163857](https://github.com/jobrunner/tempus/commit/1163857c0390c42256f0778f145f9d9173a213c3))


### Bug Fixes

* **reanalysis:** address review findings on keys, rounding and cancellation ([e983155](https://github.com/jobrunner/tempus/commit/e983155a6d8a22a58f668d0f8b8eaf664c743739))
* **reanalysis:** build the regular-grid window in a fixed array ([cc475bc](https://github.com/jobrunner/tempus/commit/cc475bc31d26118a35fcb68ca05a68cbe401811d))
* **reanalysis:** reject NaN coordinates and detach shared bucket work ([6b201a1](https://github.com/jobrunner/tempus/commit/6b201a1f94b4c7e7cd3c6032e0d5c51b05bcfe41))

## [0.25.1](https://github.com/jobrunner/tempus/compare/v0.25.0...v0.25.1) (2026-11-14)


### Bug Fixes

* **bioclim:** bump the cache version so roundoff-corrected Köppen classes are served ([41f6c02](https://github.com/jobrunner/tempus/commit/41f6c02b5b58a799b9205975540b0a26efe405f3))
* **domain:** compare Köppen thresholds on computed values beyond float roundoff ([c442f1b](https://github.com/jobrunner/tempus/commit/c442f1b43b9ec89899310ec3142f42909d41cce7))

## [0.25.0](https://github.com/jobrunner/tempus/compare/v0.24.3...v0.25.0) (2026-11-14)


### Features

* **app:** hand the TracerProvider to the Open-Meteo clients ([2f47984](https://github.com/jobrunner/tempus/commit/2f479843c8cf53649eb8ae2980985811a20130de))
* **bioclim:** expose the Rivas-Martínez macrobioclimate and feed the grid elevation into its key ([ac45d7d](https://github.com/jobrunner/tempus/commit/ac45d7d6e2ae791bab8af3ee8caa725d09119dcc))
* **omhttp:** trace each Open-Meteo attempt as a client span ([cee51af](https://github.com/jobrunner/tempus/commit/cee51af3e287d68610f803b6ad4421eca18d98ca))
* **telemetry:** trace outbound Open-Meteo calls via otelhttp ([f6982d6](https://github.com/jobrunner/tempus/commit/f6982d6df67fe43eddcd7dee5c4a739fc5da786e))


### Bug Fixes

* **bioclim:** bump the cache version so corrected Köppen classes are served ([9728651](https://github.com/jobrunner/tempus/commit/972865181b2981d7158423b62fc76d99158b1743))
* **domain:** classify Thot = 10 °C as polar (Köppen E) ([8548c78](https://github.com/jobrunner/tempus/commit/8548c782f6d863eae55c30e765b6936794385230))
* **domain:** compare Rivas-Martínez indices with the bounds beyond float roundoff ([537d880](https://github.com/jobrunner/tempus/commit/537d880f907bd181249070c0c42796d840a75aeb))
* **domain:** count only months above 10 °C for Köppen Tmon10 ([a9392c5](https://github.com/jobrunner/tempus/commit/a9392c56febc968e216b025f02680ec4ac452a87))
* **domain:** derive the Rivas-Martínez macrobioclimate and thermotype per source ([61f032e](https://github.com/jobrunner/tempus/commit/61f032e827e3dc2df746e493d4cddf6da1de0b52))
* **domain:** drop the non-Beck "As" Köppen class ([b65121a](https://github.com/jobrunner/tempus/commit/b65121a0eb11d41df8f6a6a7e67ebd129fe2f235))
* **domain:** keep an exact 70 % seasonal share at the even Köppen threshold ([a47fd36](https://github.com/jobrunner/tempus/commit/a47fd364233a16870aafec613629ff64cd215fd7))
* **domain:** let Köppen a/b take precedence over d ([8acf6f3](https://github.com/jobrunner/tempus/commit/8acf6f3ca335d971f11f512d4a411609f1a909af))
* **domain:** pick the Köppen summer half-year by temperature ([a553ad0](https://github.com/jobrunner/tempus/commit/a553ad02f023e1cdb7e3ca2fa65ee236a6c423a6))
* **domain:** require more than 70 % seasonal rain for the Köppen B threshold ([3ca365d](https://github.com/jobrunner/tempus/commit/3ca365d1681c4e95362eddee13bd1ca5916dafbb))
* **domain:** resolve Köppen s/w overlap by the wetter half-year ([2448ef1](https://github.com/jobrunner/tempus/commit/2448ef13df4253f23463de4e9e327c9f1c795c96))
* **harness:** require an explicit version on every golangci-lint action step ([b7ef2ed](https://github.com/jobrunner/tempus/commit/b7ef2ed572f134324069cfffa62adaf6dc0af0f1))
* **http:** pin Swagger UI to swagger-ui-dist 5.35.0 with SRI ([0c84c86](https://github.com/jobrunner/tempus/commit/0c84c86e03ac72eb429b105305547f144f98377e))
* **http:** pin Swagger UI to the two-week-old 5.34.1 instead of 5.35.0 ([471898b](https://github.com/jobrunner/tempus/commit/471898bfc233ec0a421ddffa6f53109a737e24b6))
* **logging:** log request-scoped events with the request context ([688f022](https://github.com/jobrunner/tempus/commit/688f022f2d1d3bf27e6af0aad88a89543b7179e5))
* **omhttp:** reject Retry-After values that overflow a Duration ([b599ea8](https://github.com/jobrunner/tempus/commit/b599ea854f2dd0dcf818bb785df1f17fe0c556a2))

## [0.24.3](https://github.com/jobrunner/tempus/compare/v0.24.2...v0.24.3) (2026-11-14)


### Bug Fixes

* **deps:** close the reachable CVEs govulncheck reports on main ([#84](https://github.com/jobrunner/tempus/issues/84)) ([4b87bb2](https://github.com/jobrunner/tempus/commit/4b87bb2847de6365113615e6dd796ed6380a4a86))

## [0.24.2](https://github.com/jobrunner/tempus/compare/v0.24.1...v0.24.2) (2026-09-14)


### Bug Fixes

* reject features with an incomplete licence at the port boundary ([#76](https://github.com/jobrunner/tempus/issues/76)) ([5721de1](https://github.com/jobrunner/tempus/commit/5721de1c695680fc15f71b6048bbdfe79ee9847d))

## [0.24.1](https://github.com/jobrunner/tempus/compare/v0.24.0...v0.24.1) (2026-09-14)


### Bug Fixes

* **cors:** reject origin patterns that widen the rule or never match ([#65](https://github.com/jobrunner/tempus/issues/65)) ([4595139](https://github.com/jobrunner/tempus/commit/4595139b5184a752b45908d03500eb7369b2cdf8))

## [0.24.0](https://github.com/jobrunner/tempus/compare/v0.23.0...v0.24.0) (2026-09-13)


### Features

* **http:** add optional per-client-IP rate limiting ([#63](https://github.com/jobrunner/tempus/issues/63)) ([e15e0e8](https://github.com/jobrunner/tempus/commit/e15e0e85c2863480f0bd9c009051f20ac0a7a184))

## [0.23.0](https://github.com/jobrunner/tempus/compare/v0.22.1...v0.23.0) (2026-09-13)


### Features

* **http:** add configurable CORS support ([#61](https://github.com/jobrunner/tempus/issues/61)) ([bc374a6](https://github.com/jobrunner/tempus/commit/bc374a64922723a953190fda9999fc108c725c01))

## [0.22.1](https://github.com/jobrunner/tempus/compare/v0.22.0...v0.22.1) (2026-09-07)


### Bug Fixes

* **frontend:** legible disabled buttons and grouped batch parse errors ([#58](https://github.com/jobrunner/tempus/issues/58)) ([2aaf84d](https://github.com/jobrunner/tempus/commit/2aaf84d24c99ca29fa3cf82b79623ff3aa25c247))

## [0.22.0](https://github.com/jobrunner/tempus/compare/v0.21.0...v0.22.0) (2026-09-07)


### Features

* **frontend:** provider multiselect and CSV upload for the batch tab ([#56](https://github.com/jobrunner/tempus/issues/56)) ([95cb9df](https://github.com/jobrunner/tempus/commit/95cb9df414154de54b4d22d0b83cc6beac14fbc5))

## [0.21.0](https://github.com/jobrunner/tempus/compare/v0.20.0...v0.21.0) (2026-09-07)


### Features

* batch query endpoint with self-throttling Open-Meteo access ([b04fe53](https://github.com/jobrunner/tempus/commit/b04fe53e67a46a2d501935063ab0edb9b2f6c624))

## [0.20.0](https://github.com/jobrunner/tempus/compare/v0.19.0...v0.20.0) (2026-08-21)


### Features

* **quality:** add the CodeCharta map and a complexity ratchet ([#47](https://github.com/jobrunner/tempus/issues/47)) ([9975a45](https://github.com/jobrunner/tempus/commit/9975a455eb9666bcf3fdec266e0073d145448ad4))

## [0.19.0](https://github.com/jobrunner/tempus/compare/v0.18.0...v0.19.0) (2026-08-21)


### Features

* **frontend:** link source citations; ci: gate breaking OpenAPI changes ([#45](https://github.com/jobrunner/tempus/issues/45)) ([1f65dcb](https://github.com/jobrunner/tempus/commit/1f65dcb8a9d16bbc0e7429b6488a95e4fc88b75d))

## [0.18.0](https://github.com/jobrunner/tempus/compare/v0.17.0...v0.18.0) (2026-08-20)


### Features

* **api:** wire the OpenAPI feature schemas and expose source URLs ([#43](https://github.com/jobrunner/tempus/issues/43)) ([442c38b](https://github.com/jobrunner/tempus/commit/442c38b1ca9184052898a661a88b8b3e94a9159e))

## [0.17.0](https://github.com/jobrunner/tempus/compare/v0.16.0...v0.17.0) (2026-08-20)


### Features

* **weather:** derive the Beaufort wind force from windSpeed10m ([#41](https://github.com/jobrunner/tempus/issues/41)) ([e43f5ed](https://github.com/jobrunner/tempus/commit/e43f5eda46bd1f40c5a24eda0a1d7d5bb281f8ed))

## [0.16.0](https://github.com/jobrunner/tempus/compare/v0.15.0...v0.16.0) (2026-08-02)


### Features

* **aggregate:** GDD base 5+10 always + Arrhenius thermal-time index; fix(frontend): datetime width on iOS ([#39](https://github.com/jobrunner/tempus/issues/39)) ([50fa0e4](https://github.com/jobrunner/tempus/commit/50fa0e4cd28340e681fdc17b87cf8893a5905288))

## [0.15.0](https://github.com/jobrunner/tempus/compare/v0.14.2...v0.15.0) (2026-08-02)


### Features

* **bioclim:** Köppen boundary flag; fix(frontend): datetime icon on iOS ([#37](https://github.com/jobrunner/tempus/issues/37)) ([cfcc038](https://github.com/jobrunner/tempus/commit/cfcc038c21fac7ed64f5614b4bd5d1f9f02c2cb8))

## [0.14.2](https://github.com/jobrunner/tempus/compare/v0.14.1...v0.14.2) (2026-08-02)


### Bug Fixes

* **frontend:** mobile horizontal overflow (cut-off stat boxes + datetime) ([#35](https://github.com/jobrunner/tempus/issues/35)) ([1634b4f](https://github.com/jobrunner/tempus/commit/1634b4fe7f027c9311313ba93fdb5851b857c996))

## [0.14.1](https://github.com/jobrunner/tempus/compare/v0.14.0...v0.14.1) (2026-08-02)


### Bug Fixes

* **frontend:** datetime icon on iOS + double-escaped umlauts in aggregate labels ([#33](https://github.com/jobrunner/tempus/issues/33)) ([d74c591](https://github.com/jobrunner/tempus/commit/d74c5914476f041e5fd1872bebac1436efddcf33))

## [0.14.0](https://github.com/jobrunner/tempus/compare/v0.13.0...v0.14.0) (2026-08-02)


### Features

* climate-derived altitudinal belt (Höhenstufe) + Rivas-Martínez thermotype ([#31](https://github.com/jobrunner/tempus/issues/31)) ([1514009](https://github.com/jobrunner/tempus/commit/1514009e2db96dc9d9c3801a4d6d95f18ea0cd5c))

## [0.13.0](https://github.com/jobrunner/tempus/compare/v0.12.0...v0.13.0) (2026-08-02)


### Features

* add bioclim provider (19 WorldClim BIO variables + Köppen-Geiger) ([#29](https://github.com/jobrunner/tempus/issues/29)) ([dad2587](https://github.com/jobrunner/tempus/commit/dad258708f87d00668b343ed0fdf777030eee1c0))

## [0.12.0](https://github.com/jobrunner/tempus/compare/v0.11.0...v0.12.0) (2026-08-02)


### Features

* add weather-aggregate provider (antecedent precip, day extrema, GDD) ([#27](https://github.com/jobrunner/tempus/issues/27)) ([509b99d](https://github.com/jobrunner/tempus/commit/509b99dc08849bad2e7a8fc4c3e0ac9d3861c4d4))

## [0.11.0](https://github.com/jobrunner/tempus/compare/v0.10.2...v0.11.0) (2026-07-23)


### Features

* add sun and moon astronomy features ([#25](https://github.com/jobrunner/tempus/issues/25)) ([7b798f2](https://github.com/jobrunner/tempus/commit/7b798f22ee13ada66962a954f27f231e768afcb9))

## [0.10.2](https://github.com/jobrunner/tempus/compare/v0.10.1...v0.10.2) (2026-07-22)


### Bug Fixes

* compute day/night from solar position (Open-Meteo archive is_day is unreliable) ([#23](https://github.com/jobrunner/tempus/issues/23)) ([2e393f0](https://github.com/jobrunner/tempus/commit/2e393f07da014e0fa45419723288690063830baa))

## [0.10.1](https://github.com/jobrunner/tempus/compare/v0.10.0...v0.10.1) (2026-07-22)


### Bug Fixes

* allow typing the year in the datetime field ([#21](https://github.com/jobrunner/tempus/issues/21)) ([3bcc561](https://github.com/jobrunner/tempus/commit/3bcc561865a4907f4d0a41e5ff9c9152f74f17ac))

## [0.10.0](https://github.com/jobrunner/tempus/compare/v0.9.0...v0.10.0) (2026-07-22)


### Features

* WMO-4677 weather-code descriptions and dew-point comfort (DE/EN) with sources ([#19](https://github.com/jobrunner/tempus/issues/19)) ([e0f0622](https://github.com/jobrunner/tempus/commit/e0f06220ba9a7f67e66343297e3a3bbfb6a07997))

## [0.9.0](https://github.com/jobrunner/tempus/compare/v0.8.0...v0.9.0) (2026-07-22)


### Features

* computed dew point (Taupunkt) as its own attributed result + frontend ([#17](https://github.com/jobrunner/tempus/issues/17)) ([cd0fc68](https://github.com/jobrunner/tempus/commit/cd0fc683ecb6304ff738a063860634e786aed751))

## [0.8.0](https://github.com/jobrunner/tempus/compare/v0.7.0...v0.8.0) (2026-07-22)


### Features

* named timezone dropdown, current-hour default, footer version ([#15](https://github.com/jobrunner/tempus/issues/15)) ([83755e4](https://github.com/jobrunner/tempus/commit/83755e481ea72408e9f26374da2b092ef153f60d))

## [0.7.0](https://github.com/jobrunner/tempus/compare/v0.6.0...v0.7.0) (2026-07-22)


### Features

* auto-detect timezone from coordinate (offline), keep manual override ([#13](https://github.com/jobrunner/tempus/issues/13)) ([0cacddb](https://github.com/jobrunner/tempus/commit/0cacddbdef91e4d8daaa98c9a2bf706c0d31f96f))

## [0.6.0](https://github.com/jobrunner/tempus/compare/v0.5.0...v0.6.0) (2026-07-22)


### ⚠ BREAKING CHANGES

* The `timezone` query parameter is no longer accepted by GET /api/v1/query. The `localTime` and `timezone` fields are removed from the QueryEcho response and from feature properties. Callers must now convert local time to UTC before calling the API.

### Features

* local-time entry with offset dropdown; remove API timezone param ([#11](https://github.com/jobrunner/tempus/issues/11)) ([c28c20d](https://github.com/jobrunner/tempus/commit/c28c20d71367c3820c6f079286ba1b19d15eebe6))

## [0.5.0](https://github.com/jobrunner/tempus/compare/v0.4.0...v0.5.0) (2026-07-22)


### Features

* image ships app-owned /data so a mounted volume needs no init/chown ([#9](https://github.com/jobrunner/tempus/issues/9)) ([44fb901](https://github.com/jobrunner/tempus/commit/44fb90156c860ee1e97db594e50b7be9cbcdfec5))

## [0.4.0](https://github.com/jobrunner/tempus/compare/v0.3.0...v0.4.0) (2026-07-21)


### Features

* add web frontend at / with geolocation and full response + attribution ([#6](https://github.com/jobrunner/tempus/issues/6)) ([2628235](https://github.com/jobrunner/tempus/commit/26282356fc804747e9269d92f2f4b4d1f4607487))


### Bug Fixes

* harden frontend link scheme, error messages, and content-type test ([#8](https://github.com/jobrunner/tempus/issues/8)) ([61b46c5](https://github.com/jobrunner/tempus/commit/61b46c54cb554d2e106939f810c15111cfbbddc4))

## [0.3.0](https://github.com/jobrunner/tempus/compare/v0.2.0...v0.3.0) (2026-07-21)


### Features

* multi-arch (amd64/arm64) Alpine image, digest-pinned deps ([#4](https://github.com/jobrunner/tempus/issues/4)) ([fda3b46](https://github.com/jobrunner/tempus/commit/fda3b46987ca39ef5b2471df977a4d889fc9b43f))

## [0.2.0](https://github.com/jobrunner/tempus/compare/v0.1.0...v0.2.0) (2026-07-21)


### Features

* tempus coordinate+time feature-query service (weather via Open-Meteo) ([#1](https://github.com/jobrunner/tempus/issues/1)) ([2203d9d](https://github.com/jobrunner/tempus/commit/2203d9d81f312771ecfc91f77f85505a65e2dba6))
