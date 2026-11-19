#!/usr/bin/env bash
# Fetch the model surface heights (HSURF) of the three reanalysis grids from the
# Open-Meteo open-data bucket into internal/adapters/reanalysis/data, where they
# are embedded into the binary, and record their checksums in MANIFEST.
# Re-run only when Open-Meteo publishes new static files; commit the result.
set -euo pipefail
base="https://openmeteo.s3.amazonaws.com/data"
dir="$(cd "$(dirname "$0")/.." && pwd)/internal/adapters/reanalysis/data"
mkdir -p "$dir"
: > "$dir/MANIFEST"
for domain in copernicus_era5 copernicus_era5_land ecmwf_ifs; do
  curl -fsS -o "$dir/$domain.om" "$base/$domain/static/HSURF.om"
  etag=$(curl -fsSI "$base/$domain/static/HSURF.om" | tr -d '\r' | awk -F': ' 'tolower($1)=="etag"{print $2}')
  sum=$(shasum -a 256 "$dir/$domain.om" | cut -d' ' -f1)
  echo "$domain.om sha256=$sum etag=$etag source=$base/$domain/static/HSURF.om" >> "$dir/MANIFEST"
done
cat "$dir/MANIFEST"
