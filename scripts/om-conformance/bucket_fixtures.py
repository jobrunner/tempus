"""Generate a miniature bucket for internal/adapters/omstore tests.

Files follow the real layout (year_YYYY.om, chunk_N.om with N = epoch hours /
504) and hold values the Go test can recompute: q = epochHour % 1000 + 7·loc,
stored with scale 20, so the decoded value is float32(q) / 20.
omfiles is used only as a black-box writer.
Usage: python bucket_fixtures.py <outdir>
"""
import sys, pathlib, datetime as dt
import numpy as np
from omfiles import OmFileWriter

OUT = pathlib.Path(sys.argv[1])
EPOCH = dt.datetime(1970, 1, 1)


def hours(t):
    return int((t - EPOCH).total_seconds() // 3600)


def write(path, h0, n, locs=3):
    path.parent.mkdir(parents=True, exist_ok=True)
    h = np.arange(h0, h0 + n)
    data = np.stack([((h % 1000) + 7 * loc) / 20 for loc in range(locs)]).astype(np.float32)[None]
    w = OmFileWriter.at_path(str(path))
    v = w.write_array(data, chunks=[1, 2, 504], scale_factor=20.0, compression="pfor_delta_2d_int16", name="data")
    w.close(v)


t = OUT / "data/testdom/temperature_2m"
y0, y1 = hours(dt.datetime(2001, 1, 1)), hours(dt.datetime(2002, 1, 1))
write(t / "year_2001.om", y0, y1 - y0)
for n in range(y1 // 504, y1 // 504 + 3):  # chunks after the year file, the first overlapping it
    write(t / f"chunk_{n}.om", n * 504, 504)
stale = y0 // 504 - 1  # predates the first year file: must be ignored
write(t / f"chunk_{stale}.om", stale * 504, 504)
c = OUT / "data/testdom/cloud_cover"
for n in range(y0 // 504 - 2, y0 // 504 + 1):  # only chunk files
    write(c / f"chunk_{n}.om", n * 504, 504)
