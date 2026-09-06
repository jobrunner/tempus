# Batch-Query für tempus — Design

**Datum:** 2026-09-06
**Status:** Entwurf zur Review

## Kontext und Ziel

tempus beantwortet heute ausschließlich Einzelabfragen (`GET /api/v1/query`).
Poweruser wollen gelegentlich größere Bestände (Größenordnung 1000
Koordinaten/Datum-Paare) interaktiv im tempus-Frontend anreichern, mit
Fortschrittsanzeige. tempus hängt dabei am freien Open-Meteo-Tier
(≈10.000 gewichtete Calls/Tag, 5.000/h, 600/min); eine Einzelabfrage kostet
bis zu 4 Upstream-Calls (weather 1, aggregate 2, bioclim 1). Der Batch-Betrieb
muss sich also selbst drosseln, statt in Limits zu laufen.

ortus besitzt bereits einen Batch-Endpoint (`POST /api/v1/query/batch`,
synchron + NDJSON-Streaming, clientseitiger Fortschritt). Der tempus-Batch
soll **formkonform zu diesem Vertrag** sein — gleiche Pfad-Konvention, gleiche
Envelope-, Fehler- und Kappungs-Muster. ortus ist dabei nur die
Konventions-Referenz, kein Konsument und keine Abhängigkeit.

Randbedingungen aus der Klärung:

- Nutzungsprofil: interaktiv im tempus-Frontend, selten, Zeit egal,
  Fortschrittsanzeige nötig. Daneben eine iOS-App mit Einzelabfragen, die
  nicht hinter Batches verhungern darf.
- Open-Meteo: freier Tier, kein API-Key, kein Self-Hosting.
- Keine Durabilität: ein Batch überlebt keinen Neustart. Der Feature-Cache
  ist das implizite Checkpointing — ein erneuter Submit holt bereits
  Geholtes aus dem Cache und kostet fast nichts.

## Nicht-Ziele

- Kein asynchrones Job-Modell (keine Job-IDs, kein Status-Endpoint, kein
  Polling) — ortus macht das auch nicht.
- Kein Open-Meteo-API-Key-Support, kein Self-Hosted Open-Meteo.
- Kein Multi-Koordinaten-Packing in Open-Meteo-Requests (Limits zählen dort
  pro Location, nicht pro HTTP-Request; das Packing spart nur Round-Trips
  und verkompliziert Cache und Fehlerbehandlung). Später als Optimierung
  möglich.
- Kein Redis, keine neue Persistenz.
- Keine Authentifizierung/serverseitige Mandantentrennung (wie bisher).

## Teil 1: API-Vertrag `POST /api/v1/query/batch`

### Request

```json
{
  "providers": ["weather", "aggregate"],
  "gddBase": 7,
  "refPeriod": "1991-2020",
  "points": [
    { "id": "fund-17", "lat": 49.79, "lon": 9.95, "datetime": "2025-06-03T14:00:00Z" },
    { "lat": 47.42, "lon": 10.98, "datetime": "2024-08-14T09:30:00+02:00" }
  ]
}
```

- Gemeinsame Optionen (`providers`, `gddBase`, `refPeriod`) gelten für alle
  Punkte und heißen wie die Query-Parameter von `GET /api/v1/query`; alle
  optional mit denselben Defaults.
- GDD-Semantik wie im Einzelrequest: der aggregate-Provider liefert **immer
  Basis 5 und Basis 10** aus derselben Tagesserie; `gddBase` ergänzt nur
  optional einen zusätzlichen `custom`-Wert. Batch-Items enthalten also
  stets GDD5 und GDD10, ohne dass der Client etwas anfordern muss.
- Pro Punkt: `lat`, `lon`, `datetime` (RFC 3339) — Parsing über die
  bestehende `domain.ParseQueryRequest`-Logik, damit Einzel- und
  Batch-Validierung nicht auseinanderlaufen.
- `id` ist ein opakes Echo-Feld (ortus-Muster): fehlt es, wird der 0-basierte
  Eingabeindex als String verwendet.

### Validierung und Kappungen (ortus-Muster)

- Leeres `points` → 400.
- Mehr als `query.batch.max_points` (Default 10.000) → 400.
- Mehr als `query.batch.max_sync_points` (Default 1.000) im Sync-Modus →
  413 mit Hinweis, es mit `Accept: application/x-ndjson` erneut zu versuchen.
- Body-Größe via `http.MaxBytesReader` gekappt
  (`max_points × 512 B + 64 KiB`) → 413.
- Nicht parsebare Punkte (fehlende/ungültige Koordinaten oder Datetime)
  erzeugen ein Per-Item-Fehlerobjekt und erreichen den Query-Pfad nie;
  sie brechen den Batch nicht ab.
- Nach erfolgreicher Validierung hebt der Handler das Write-Deadline des
  Requests auf (ortus-Muster), weil der Batch lange laufen darf.

### Response — synchron (Default)

```json
{
  "results": [ { "id": "fund-17", "query": { "...": "..." }, "features": [], "providers": [] } ],
  "total": 2,
  "processing_time_ms": 8412
}
```

Jedes Item ist der bekannte Einzel-Envelope von `/api/v1/query`
(`query`, `features`, `providers`) plus `id`, in Eingabereihenfolge.
Fehlgeschlagene Punkte (Parse-Fehler) bestehen nur aus
`{ "id": "...", "error": { "message": "..." } }`. Provider-Fehler einzelner
Punkte bleiben wie gehabt im `providers`-Array des Items kodiert
(Status/`retryable`/`retryAfter`), das Item ist dann kein `error`-Item.

### Response — Streaming (`Accept: application/x-ndjson`)

Ein Result-Item pro Zeile, in Eingabereihenfolge, pro Zeile geflusht
(`http.NewResponseController`). Kein Top-Level-Envelope (kein
`total`/`processing_time_ms`) — der Client kennt die Gesamtzahl selbst.
Bricht der Stream serverseitig ab, erkennen Clients das am Vergleich
Zeilenzahl vs. gesendete Punktzahl (ortus dokumentiert dasselbe).

### OpenAPI und Docs

- Neue Pfad-Items/Schemas in `internal/adapters/http/openapi.yaml`
  (embedded, von `TestRoutesMatchOpenAPISpec` erzwungen) und im
  Spiegel-Spec `api/openapi/openapi.yaml`. Schema-Namen analog ortus:
  `BatchQueryPoint`, `BatchQueryRequest`, `BatchQueryResultItem`,
  `BatchQueryResponse`.
- Rein additive Änderung — der oasdiff-Gate (mit `--flatten-allof`) schlägt
  nicht an, kein `api-breaking-ok`-Label nötig.
- Prose-Doku: neuer Abschnitt in der HTTP-API-Referenz plus die neuen
  Config-Keys in der Konfigurations-Referenz (`doc-drift-check` vor dem PR).

## Teil 2: Drosselschicht für Open-Meteo

Heute hat keiner der drei Open-Meteo-Adapter (weather, aggregate, bioclim)
einen Rate-Limiter oder Retries; der `Retry-After`-Hint wird nur im Envelope
nach oben gereicht. Neu:

### Gemeinsamer Outbound-Limiter

- Ein geteilter Token-Bucket (`golang.org/x/time/rate`, neue direkte
  Dependency) vor **allen** Open-Meteo-HTTP-Calls, injiziert als
  gemeinsamer „Doer" (Wrapper um `*http.Client`) in die drei Adapter.
- Rate konfigurierbar: `providers.openmeteo.rate_per_minute`
  (Default 500 — bewusst unter den 600/min des freien Tiers).
- Erwerb via `limiter.Wait(ctx)`: bricht der Request-Kontext ab
  (Client weg, Timeout), wartet nichts weiter.

### Priorisierung der Einzelabfragen

Kein explizites Prioritätssystem (YAGNI). Die Fairness entsteht strukturell:

- Der Batch-Worker-Pool ist klein (`query.batch.concurrency`, Default 4,
  gleicher Config-Key wie bei ortus) — es konkurrieren also höchstens
  ~4 Batch-Punkte gleichzeitig um Tokens.
- Bei 500 Token/min beträgt die Wartezeit pro Token ~120 ms; eine
  Einzelabfrage (≤4 Calls) wartet im schlimmsten Fall unter einer Sekunde.
  Das ist für die iOS-App akzeptabel; sollte es je kippen, ist ein
  Zwei-Klassen-Limiter eine lokale Folgeänderung im Doer.

### Retry mit Retry-After

- Der gemeinsame Doer wiederholt transiente Fehler (429/5xx) selbst:
  max. `providers.openmeteo.retry_attempts` (Default 3) Versuche,
  Wartezeit = `Retry-After`-Header, sonst exponentieller Backoff
  (Start 2 s), immer kontext-abbrechbar. Open-Meteo-GETs sind idempotent.
- Erst nach ausgeschöpften Versuchen wird wie bisher ein
  `TransientError` mit `retryAfter` in den Envelope kodiert.
- Die bestehende `retryAfter`-Parsing-Lücke im aggregate-Adapter
  (hartkodierte 30 s statt Header) verschwindet dabei, weil die
  Klassifikation in den gemeinsamen Doer wandert.

### Tagesbudget (gewichtete Calls)

- Open-Meteo gewichtet Calls nach Datenmenge; lange Archiv-Zeiträume zählen
  mehrfach. Statische, konfigurierbare Gewichte pro Call-Typ (Defaults sind
  dokumentierte Schätzungen): weather 1, aggregate-daily nach
  Zeitraumlänge, bioclim (30-Jahres-Serie) deutlich höher (~30).
- Ein In-Memory-Tageszähler (UTC-Tageswechsel) mit Budget
  `providers.openmeteo.daily_budget` (Default 8.000 — Puffer unter 10.000).
- **Nur der Batch-Pfad prüft das Budget**: der Zähler lebt im Doer (er
  zählt jeden Upstream-Call, egal woher), aber nur der `BatchService`
  fragt ihn vor dem Dispatch jedes Punkts ab. Ist das Budget erschöpft,
  bekommen die restlichen Punkte sofort einen Per-Provider-Fehler
  (transient, `retryable: true`, Hinweis „daily budget exhausted, retry
  tomorrow") statt Upstream-Calls zu feuern. Einzelabfragen laufen ungeprüft
  weiter (ihr Volumen ist vernachlässigbar; harte Grenzen setzt notfalls
  Open-Meteo selbst per 429, was der Doer sauber behandelt).
- Erneuter Submit am Folgetag: bereits Geholtes kommt aus dem Cache,
  nur die Fehlpunkte kosten Budget.

### Dedup innerhalb eines Batches

Vor dem Abarbeiten werden Punkte mit identischem effektiven Schlüssel
(Koordinate auf Cache-Präzision gerundet, identisches Datetime, identische
gemeinsame Optionen) gruppiert: ein Fetch, das Ergebnis wird auf alle
zugehörigen `id`s gefächert. Bei geclusterten Funddaten spart das real Calls.

## Teil 3: aggregate cachbar machen

Der aggregate-Provider ist heute ungecacht, weil `gddBase` nicht im
Cache-Key steckt — im Batch der Hotspot (2 der 4 Calls pro Punkt). Änderung:

- Der `CacheKey` des Caching-Decorators erhält eine optionale
  Provider-Parametrisierungs-Komponente (z. B. `gddBase=10` als Teil des
  Hash-Inputs); der aggregate-Provider liefert sie, alle anderen lassen
  sie leer (Keys der bestehenden Provider ändern sich nicht — kein
  Cache-Invalidieren beim Deploy).
- aggregate wird mit dem bestehenden Decorator umwickelt; TTL-Logik
  (reif 365 d / unreif 1 h über `archive_delay`) gilt unverändert.

## Teil 4: Frontend — Batch-Tab

Das eingebettete `index.html` (go:embed, kein CDN) bekommt einen zweiten
Tab „Batch" nach dem ortus-Muster:

- **Eingabe:** Textarea mit CSV-Zeilen `[id,] lat, lon, datetime`
  (optionale führende id-Spalte; fehlt sie, wird der Zeilenindex zur id).
  Dazu die gemeinsamen Optionen als Formularfelder: Provider-Auswahl,
  `gddBase`, `refPeriod`.
- **Submit streamt immer** (`fetch` mit `Accept: application/x-ndjson`) —
  nie der Sync-Modus, damit große Batches nicht an der 413-Grenze scheitern.
- **Fortschritt clientseitig:** determinierter `<progress>`-Balken aus
  „empfangene NDJSON-Zeilen / gesendete Punkte". Kein Polling.
- **Ergebnistabelle** wächst zeilenweise: id, Koordinate, Datum, Status pro
  Provider (ok/Fehler mit Meldung); Fehlerzeilen sichtbar markiert, damit
  Poweruser gezielt erneut einreichen können (z. B. nach Budget-Erschöpfung).
- **JSON-Export** nach Abschluss: der Client rekonstruiert lokal den
  Sync-Envelope `{results, total, processing_time_ms}` (Zeitmessung
  clientseitig) — identisch zum ortus-Frontend.
- **Abbruch-Button** (`AbortController` auf den `fetch`) plus UI-Hinweis,
  dass der Tab während des Laufs offen bleiben muss; ein Neustart ist dank
  Cache billig.

## Architektur-Einordnung (hexagonal)

- **HTTP-Adapter:** neuer Handler `handleQueryBatch` + NDJSON-Rendering in
  `internal/adapters/http/` (analog ortus' `batch.go`/`batch_render.go`
  aufgeteilt, damit keine Datei wuchert).
- **Application:** ein `BatchService` (oder Erweiterung des Input-Ports) mit
  Worker-Pool und Dedup, der pro Punkt den bestehenden
  `FeatureService.Query` aufruft — Cache, Provider-Fan-out,
  Fehler-Envelope kommen geschenkt. Emission in Eingabereihenfolge über
  ein kleines Reorder-Fenster (Pool füllt vor, ausgegeben wird, sobald der
  Kopf fertig ist).
- **Outbound:** der gemeinsame Open-Meteo-Doer (Limiter + Retry + Budget-
  Zähler) lebt als eigenes Paket unter `internal/adapters/openmeteo`-Nähe
  und wird in `internal/app/providers.go` in alle drei Adapter injiziert.
  Die depguard/gomodguard-Grenzen bleiben unberührt (kein neuer
  Domänen-Import in Adapter-Richtung).

## Konfiguration (neu, viper, Prefix `TEMPUS`)

| Key | Default | Zweck |
|---|---|---|
| `query.batch.max_points` | 10000 | harte Kappung (400) |
| `query.batch.max_sync_points` | 1000 | Sync-Grenze (413 + NDJSON-Hinweis) |
| `query.batch.concurrency` | 4 | Worker-Pool-Größe |
| `providers.openmeteo.rate_per_minute` | 500 | Token-Bucket-Rate |
| `providers.openmeteo.retry_attempts` | 3 | Retries im Doer |
| `providers.openmeteo.daily_budget` | 8000 | gewichtetes Tagesbudget (nur Batch-Pfad) |

## Fehlerbehandlung — Zusammenfassung

| Fall | Verhalten |
|---|---|
| Punkt nicht parsebar | Per-Item `{id, error:{message}}`, Batch läuft weiter |
| Provider-Fehler eines Punkts | wie bei `/query`: Status im `providers`-Array des Items |
| 429/5xx upstream | Retry im Doer (Retry-After/Backoff), danach transienter Provider-Fehler |
| Tagesbudget erschöpft | restliche Punkte: transienter Provider-Fehler „retry tomorrow", keine Upstream-Calls |
| Client bricht ab / Stream reißt | Kontext-Abbruch stoppt Worker und Limiter-Wartende; Wiederholung ist dank Cache billig |
| Ganzer Request ungültig | normaler Fehler-Envelope mit 400/413 |

## Teststrategie (TDD, `make verify` muss grün bleiben)

- **Handler:** Sync-Envelope, NDJSON-Streaming (Zeilen, Reihenfolge,
  Flush), 400 (leer, >max_points), 413 (Sync-Grenze mit
  Hinweis, Body-Cap), Per-Item-Fehler.
- **BatchService:** Dedup (ein Fetch, gefächerte ids), Reihenfolge-Garantie
  trotz Pool, Kontext-Abbruch.
- **Doer:** Token-Bucket greift (Fake-Clock), Retry-After wird beachtet,
  Retry-Erschöpfung → TransientError, Budget-Erschöpfung → sofortiger
  Fehler ohne HTTP-Call (httptest-Zähler).
- **aggregate-Caching:** gleicher Punkt, gleiches `gddBase` → Cache-Hit;
  anderes `gddBase` → Miss; bestehende Provider-Keys unverändert
  (Golden-Key-Test).
- **Contract:** `TestRoutesMatchOpenAPISpec` erzwingt die Spec-Einträge;
  oasdiff-Gate bleibt grün (additiv).
- **Frontend:** manueller Smoke-Test über den Batch-Tab (Streaming,
  Progress, Abbruch); kein neues E2E-Gerüst.

## Grobe Laufzeitabschätzung

1000 Punkte × bis zu 4 Calls ≈ 4000 gewichtete Calls; bei 500/min läuft ein
kalter Batch in ~10–15 min durch — gut streambar. Mit Cache-Treffern und
Dedup entsprechend schneller. Enthält der Batch viele frische Koordinaten
mit bioclim, greift das Tagesbudget früher; die Restpunkte kommen als
transiente Fehler zurück und der Folge-Submit am nächsten Tag ist billig.
