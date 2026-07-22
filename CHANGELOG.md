# Changelog

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
