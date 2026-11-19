# OM file format (version 3) — what tempus reads, and how we know

This decoder is a clean-room implementation. Its sources are limited to:

- **Spec** — the format description in the README of
  `open-meteo/om-file-format` (prose and tables only, no code).
- **Oracle** — `omfiles` (Python) used as a black box: it wrote `.om` files from
  inputs we chose and read files back. Its source, and the source of every other
  OM implementation (C, Swift, Rust, TypeScript, Go ports) and of TurboPFor, was
  never consulted. The experiments are reproducible with
  `scripts/om-conformance/fixtures.py` and the scratch probes they grew out of.
- **Real files** — bytes of files in the public bucket `s3://openmeteo`,
  checked against values the oracle read from the same files
  (`testdata/s3/`, `scripts/om-conformance/s3_fixture_cases.py`).

Byte order is little-endian unless stated otherwise.

## 1. File layout (Spec, confirmed on real files)

```
"OM" 0x03 | compressed chunks … | LUT | metadata | trailer (24 bytes)
```

Trailer: `"OM"`, version `0x03`, five reserved bytes, then **offset** (u64) and
**size** (u64) of the root variable. The Spec's table lists size before offset;
every oracle file and every real file has offset first (Oracle, Real files).

## 2. Array variable (Spec, confirmed)

| Bytes | Field |
|---|---|
| 1 | data type — 20 for a float32 array (Oracle) |
| 1 | compression — 0 for `pfor_delta_2d_int16` (Oracle) |
| 2 | name length |
| 4 | number of children |
| 8 | LUT size in bytes |
| 8 | LUT offset |
| 8 | number of dimensions *n* |
| 4 | scale factor (f32) |
| 4 | add offset (f32) |
| 16·children | child sizes and offsets |
| 8·n | dimension lengths |
| 8·n | chunk lengths |
| name | |

Chunks are numbered in C order over the chunk grid (Oracle: multi-chunk files;
Real files: series reads). Consequence used for efficiency: all chunks along the
last dimension for one location are adjacent in the file.

## 3. Lookup table (Oracle experiments 6, Real files)

The LUT holds `chunks+1` absolute offsets (start of each chunk and the end of the
last one). It is cut into blocks of 64 entries; every block occupies the same
number of bytes, `LUT size / number of blocks`, padded with zeros. A block is
a prefix varint (§4.1) with its first offset, followed by one PFor block (§4.3,
horizontal packing, no zigzag) with the differences to the following entries.

## 4. Integer coding

### 4.1 Prefix varint (Oracle experiments 1–2, Real files)

The number of leading one bits *k* of the first byte gives the number of extra
bytes; the remaining bits of the first byte are the most significant ones.

| First byte | Value |
|---|---|
| `0xxxxxxx` | 7 bits |
| `10xxxxxx` b1 | `x<<8 | b1` |
| `110xxxxx` b1 b2 | `x<<16 | LE16(b1,b2)` |
| `1110xxxx` b1 b2 b3 | `x<<24 | b1<<16 | b2<<8 | b3` (**big-endian**, Real files: ECMWF IFS chunk files; little-endian gave offsets that miss the chunk, big-endian matches the oracle's encoding of the same values) |
| `11110xxx` b1…b4 | `x<<32 | LE32(b1…b4)` (Real files: offsets near the middle of 4 GB and 32 GB files) |

Longer forms were never observed (offsets stay below 32 GiB) and are rejected.

### 4.2 Bit packing (Oracle experiments 1, 2, 4)

- **Horizontal**: values of width *b* are written one after another into an
  LSB-first bit stream; `ceil(n·b/8)` bytes. Unused high bits of the last byte
  are not guaranteed to be zero (Oracle vs Real files differ there).
- **Lanes** (full blocks of 128 values in chunks): 8 lanes of 16-bit words.
  Value *i* belongs to lane `i % 8` at position `i / 8`; each lane is an
  LSB-first bit stream over its own words; word *k* of lane *l* is stored at byte
  `(k·8 + l)·2`. A block takes `16·b` bytes.

### 4.3 PFor block (Oracle experiments 1–5)

Header byte: low six bits = base width *b*, high two bits = mode.

| Mode | Layout |
|---|---|
| `00` | header, base values (*b* bits each) |
| `11` | header, one value in `ceil(b/8)` bytes; all *n* values equal it |
| `10` | header, exception width *bx*, bitmap `ceil(n/8)` bytes (bit *i* = value *i* has an exception), exceptions packed horizontally with *bx* bits, base values; value = base \| exception << b |
| `01` | header, exception count *c*, base values, *c* exception varints (§4.4), *c* position bytes; value = base \| exception << b |

### 4.4 Exception varint (Oracle experiment 7, Real files)

| First byte | Value |
|---|---|
| `< 0xb1` | the byte |
| `0xb1…0xf0` b1 | `(first − 0xb1)·256 + b1 + 0xb1` (up to 16 560) |
| `0xf1…0xf8` b1 b2 | `(first − 0xf1)·65536 + LE16(b1,b2) + 16 561` (Real files: IFS cloud cover; oracle writes the same bytes for those values in lane blocks) |
| `0xff` b1 b2 | `LE16(b1,b2)` (Oracle: scalar blocks) |

Other first bytes were never observed and are rejected.

## 5. `pfor_delta_2d_int16` chunk (Oracle experiments 1–4)

Encoding, as observed; the decoder runs it backwards:

1. Quantise: `round(x·scale)` to int16; NaN and +∞ become 32767, −∞ −32768.
2. Treat the chunk as rows × columns, columns = the (edge-clipped) length of the
   last dimension, rows = the product of the others. Every row except the first
   is replaced by its difference to the row above (int16, wrapping).
3. Flatten; replace every value but the first by its difference to the previous
   one (int16, wrapping), zigzag-encode (`(v<<1) ^ (v>>15)`).
4. Store the first value as a prefix varint of its uint16 bits, then the rest
   in PFor blocks: full blocks of 128 with lane packing, a final shorter block
   with horizontal packing.

Decoding to float: 32767 → NaN, otherwise `float32(v) / scale + add_offset`
in float32 (division, not multiplication by the reciprocal: the oracle
fixtures fail with the latter).

Edge chunks use their clipped shape in step 2; partial chunks along the last
dimension shorten the rows.
