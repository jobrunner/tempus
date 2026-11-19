"""Expected values for internal/adapters/omfile's real-file replay test.

Reads the chosen series from the public Open-Meteo bucket with omfiles (used
only as a black-box oracle) and writes testdata/s3/cases.json. The byte ranges
themselves are recorded by the Go test (TEMPUS_OM_RECORD=1), so the fixture
holds exactly what the Go reader fetches.
Usage: python s3_fixture_cases.py <out.json>
"""
import sys, json, base64
import numpy as np, s3fs
from omfiles import OmFileReader

FS = s3fs.S3FileSystem(anon=True, client_kwargs={"region_name": "us-west-2"})
CASES = [
    ("copernicus_era5/temperature_2m/year_2005.om", [550, 764], 0, 8760),
    ("copernicus_era5/precipitation/year_2005.om", [550, 764], 4000, 4100),
    ("copernicus_era5_land/temperature_2m/year_2005.om", [1376, 1910], 0, 8760),
    ("ecmwf_ifs/temperature_2m/year_2020.om", [0, 1234567], 0, 8784),
    ("ecmwf_ifs/precipitation/chunk_950.om", [0, 1234567], 0, 504),
    ("ecmwf_ifs/cloud_cover/chunk_817.om", [0, 6599679], 400, 504),
    ("copernicus_era5/static/HSURF.om", [550], 700, 800),
    # offsets in [2^21, 2^28) exercise the 4-byte LUT varint (k=3)
    ("copernicus_era5/temperature_2m/year_2005.om", [40, 100], 1000, 3000),
    ("copernicus_era5/temperature_2m/chunk_950.om", [550, 764], 0, 504),
    ("copernicus_era5_land/dew_point_2m/chunk_960.om", [1376, 1910], 100, 504),
    ("ecmwf_ifs/temperature_2m/chunk_989.om", [0, 3000000], 0, 504),
    ("ecmwf_ifs/wind_u_component_10m/year_2017.om", [0, 6000], 0, 8760),
    ("ecmwf_ifs/static/HSURF.om", [0], 3000000, 3000100),
]
out = []
for path, coord, t0, t1 in CASES:
    r = OmFileReader.from_fsspec(FS, f"openmeteo/data/{path}")
    sel = tuple(coord) + (slice(t0, t1),)
    vals = np.asarray(r[sel], np.float32).astype("<f4")
    out.append({"path": path, "coord": coord, "t0": t0, "t1": t1, "want": base64.b64encode(vals.tobytes()).decode()})
    print(path, r.shape, r.chunks, len(vals))
json.dump(out, open(sys.argv[1], "w"), indent=1)
