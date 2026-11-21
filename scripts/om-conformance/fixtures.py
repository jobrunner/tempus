"""Generate oracle fixtures for internal/adapters/omfile.

omfiles (GPL-2.0) is used strictly as a black box: it writes .om files from
chosen inputs and reads them back. Neither its source nor any other OM library
source is consulted (clean-room protocol, see internal/adapters/omfile/FORMAT.md).

Output per case: <name>.om plus <name>.f32 (the array read back by omfiles,
little-endian float32, C order).
Usage: python fixtures.py <outdir>
"""
import sys, pathlib
import numpy as np
from omfiles import OmFileReader, OmFileWriter

OUT = pathlib.Path(sys.argv[1])
OUT.mkdir(parents=True, exist_ok=True)
rng = np.random.default_rng(20261120)


def from_zz(zz, first=0):
    d = [(z >> 1) ^ -(z & 1) for z in zz]
    return np.cumsum([first] + d).astype(np.float32)


def case(name, data, chunks, scale=1.0):
    path = OUT / f"{name}.om"
    w = OmFileWriter.at_path(str(path))
    v = w.write_array(np.asarray(data, np.float32), chunks=chunks, scale_factor=scale, compression="pfor_delta_2d_int16", name="data")
    w.close(v)
    back = np.asarray(OmFileReader.from_path(str(path))[...], np.float32)
    back.astype("<f4").tofile(OUT / f"{name}.f32")


# Hand-picked encodings (FORMAT.md experiment numbers in brackets).
case("single_zero", [[0]], [1, 1])
case("varint_forms", [[0, 100, 1000, 16383, 16384, 20000, -1, -20000]], [1, 1])
case("constant_delta", [[0, 1, 2, 3]], [1, 4])
case("horizontal_b4", [[0, 5, 0, 5]], [1, 4])
case("two_rows", [[1, 2, 3, 4], [2, 3, 4, 5]], [2, 4])
case("lanes_129", from_zz([i % 8 for i in range(128)]).reshape(1, -1), [1, 129])
case("lanes_300", from_zz([i % 4 for i in range(299)]).reshape(1, -1), [1, 300])
z = [1] * 20; z[5] = 1000
case("exc_varint", from_zz(z).reshape(1, -1), [1, 21])
z = [1] * 20; z[5] = 1000; z[9] = 1000
case("exc_bitmap", from_zz(z).reshape(1, -1), [1, 21])
z = [1] * 128
for p in range(0, 128, 4): z[p] = 1000 + p
case("exc_bitmap_lanes", from_zz(z).reshape(1, -1), [1, 129])
z = [1] * 128; z[5] = z[9] = z[77] = 1000
case("exc_varint_lanes", from_zz(z).reshape(1, -1), [1, 129])
for v in [176, 177, 434, 16560, 16561, 65535]:
    z = [0] * 40; z[3] = v
    case(f"exc_value_{v}", from_zz(z).reshape(1, -1), [1, 41])
case("nan_inf", [[1, np.nan, 2, np.inf, -np.inf]], [1, 5])
case("scale_10", [[0.04, 0.05, 0.06, -0.05, 0.15, 0.25, -0.25, 1e9, -1e9]], [1, 9], scale=10.0)
case("cube", (np.arange(24, dtype=np.float32).reshape(2, 3, 4) ** 2), [2, 3, 4])
case("many_chunks", rng.integers(0, 50, size=(1, 3 * 300)).astype(np.float32), [1, 3])

# Realistic shapes: temperature-like fields with edge chunks in every dimension.
t = np.linspace(0, 40 * np.pi, 1100)
field = (15 + 10 * np.sin(t)[None, None, :] + rng.normal(0, 2, size=(2, 8, 1100))).astype(np.float32)
field[1, 4, 100:130] = np.nan
case("weather_like_20", field, [1, 6, 504], scale=20.0)
precip = np.where(rng.random((3, 7, 700)) < 0.8, 0, rng.gamma(0.6, 3, size=(3, 7, 700))).astype(np.float32)
case("precip_like_10", precip, [1, 6, 504], scale=10.0)
wide = rng.normal(0, 3000, size=(2, 9, 600)).astype(np.float32)
case("wide_range", wide, [1, 6, 128], scale=1.0)
