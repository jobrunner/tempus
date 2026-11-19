"""Random sweep cases for the omfile replay test (opt-in robustness check).

Picks random cells across the variables and file kinds tempus reads and writes
expected values read by omfiles (black-box oracle). Run the Go replay test with
TEMPUS_OM_RECORD=1 TEMPUS_OM_CASES=<out> TEMPUS_OM_RANGES=<tmp> against it.
Usage: python s3_sweep_cases.py <out.json> <n> <seed>
"""
import sys, json, base64, random
import numpy as np, s3fs
from omfiles import OmFileReader

FS = s3fs.S3FileSystem(anon=True, client_kwargs={"region_name": "us-west-2"})
DOMVARS = {
    "copernicus_era5": ["temperature_2m", "dew_point_2m", "precipitation", "snowfall_water_equivalent", "wind_u_component_10m", "wind_v_component_10m", "cloud_cover"],
    "copernicus_era5_land": ["temperature_2m", "dew_point_2m"],
    "ecmwf_ifs": ["temperature_2m", "dew_point_2m", "precipitation", "snowfall_water_equivalent", "wind_u_component_10m", "wind_v_component_10m", "cloud_cover"],
}
rnd = random.Random(int(sys.argv[3]))
out = []
while len(out) < int(sys.argv[2]):
    dom = rnd.choice(list(DOMVARS)); var = rnd.choice(DOMVARS[dom])
    files = [p.rsplit("/", 1)[1] for p in FS.ls(f"openmeteo/data/{dom}/{var}/")]
    name = rnd.choice(files)
    path = f"{dom}/{var}/{name}"
    r = OmFileReader.from_fsspec(FS, f"openmeteo/data/{path}")
    coord = [rnd.randrange(d) for d in r.shape[:-1]]
    nt = r.shape[-1]; t0 = rnd.randrange(nt); t1 = rnd.randrange(t0 + 1, nt + 1)
    vals = np.asarray(r[tuple(coord) + (slice(t0, t1),)], np.float32).astype("<f4")
    out.append({"path": path, "coord": coord, "t0": t0, "t1": t1, "want": base64.b64encode(vals.tobytes()).decode()})
    print(len(out), path, coord, t0, t1, flush=True)
json.dump(out, open(sys.argv[1], "w"))
