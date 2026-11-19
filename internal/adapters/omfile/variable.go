// Package omfile decodes the parts of Open-Meteo's OM file format (version 3)
// that tempus reads: the trailer, an array variable, its chunk lookup table and
// pfor_delta_2d_int16 chunks. It performs no I/O; callers fetch byte ranges and
// hand them in. The format knowledge, and where each piece of it comes from, is
// recorded in FORMAT.md — this is a clean-room implementation.
package omfile

import (
	"encoding/binary"
	"errors"
	"math"
)

var (
	errTruncated     = errors.New("omfile: truncated data")
	errCorrupt       = errors.New("omfile: corrupt data")
	errUnknownVarint = errors.New("omfile: unknown exception varint prefix")
	// ErrUnsupported reports a file feature this decoder does not implement.
	ErrUnsupported = errors.New("omfile: unsupported format feature")
)

const (
	// TrailerSize is the length of the trailer at the very end of a file.
	TrailerSize = 24
	version3    = 3

	// DTypeFloatArray is the data type of a float32 array variable.
	DTypeFloatArray = 20
	// CompressionPforDelta2DInt16 is the only codec tempus needs.
	CompressionPforDelta2DInt16 = 0

	// variableFixedSize is the fixed part of an array variable message.
	variableFixedSize = 40
	// LUTBlockEntries is the number of offsets per lookup-table block.
	LUTBlockEntries = 64
)

// ParseTrailer returns the offset and size of the root variable.
func ParseTrailer(b []byte) (offset, size uint64, err error) {
	if len(b) < TrailerSize {
		return 0, 0, errTruncated
	}
	t := b[len(b)-TrailerSize:]
	if t[0] != 'O' || t[1] != 'M' {
		return 0, 0, errCorrupt
	}
	if t[2] != version3 {
		return 0, 0, ErrUnsupported
	}
	return binary.LittleEndian.Uint64(t[8:]), binary.LittleEndian.Uint64(t[16:]), nil
}

// Array is a decoded array variable message.
type Array struct {
	Name        string
	DType       uint8
	Compression uint8
	Dims        []uint64
	Chunks      []uint64
	Scale       float32
	AddOffset   float32
	LUTOffset   uint64
	LUTSize     uint64
}

// ParseArray decodes the root variable message. Only float32 arrays compressed
// with pfor_delta_2d_int16 are accepted.
func ParseArray(b []byte) (Array, error) {
	if len(b) < variableFixedSize {
		return Array{}, errTruncated
	}
	a := Array{DType: b[0], Compression: b[1]}
	if a.DType != DTypeFloatArray || a.Compression != CompressionPforDelta2DInt16 {
		return Array{}, ErrUnsupported
	}
	nameLen := int(binary.LittleEndian.Uint16(b[2:]))
	children := int(binary.LittleEndian.Uint32(b[4:]))
	a.LUTSize = binary.LittleEndian.Uint64(b[8:])
	a.LUTOffset = binary.LittleEndian.Uint64(b[16:])
	nDims := binary.LittleEndian.Uint64(b[24:])
	a.Scale = math.Float32frombits(binary.LittleEndian.Uint32(b[32:]))
	a.AddOffset = math.Float32frombits(binary.LittleEndian.Uint32(b[36:]))
	if nDims == 0 || nDims > 8 || a.Scale == 0 {
		return Array{}, errCorrupt
	}
	pos := variableFixedSize + 16*children
	need := pos + 16*int(nDims) + nameLen
	if len(b) < need {
		return Array{}, errTruncated
	}
	a.Dims = make([]uint64, nDims)
	a.Chunks = make([]uint64, nDims)
	for i := range a.Dims {
		a.Dims[i] = binary.LittleEndian.Uint64(b[pos+8*i:])
		a.Chunks[i] = binary.LittleEndian.Uint64(b[pos+8*(int(nDims)+i):])
		if a.Dims[i] == 0 || a.Chunks[i] == 0 {
			return Array{}, errCorrupt
		}
	}
	pos += 16 * int(nDims)
	a.Name = string(b[pos : pos+nameLen])
	return a, nil
}

// Float converts a quantised int16 bit pattern to float32 (NaN for the marker).
func (a Array) Float(v uint16) float32 {
	if v == nanMarker {
		return float32(math.NaN())
	}
	x := float32(v)
	if v >= 1<<15 {
		x -= 1 << 16
	}
	return x/a.Scale + a.AddOffset
}

// chunkGrid is the number of chunks along each dimension.
func (a Array) chunkGrid() []uint64 {
	g := make([]uint64, len(a.Dims))
	for i := range g {
		g[i] = (a.Dims[i] + a.Chunks[i] - 1) / a.Chunks[i]
	}
	return g
}

// NumChunks is the total number of chunks.
func (a Array) NumChunks() uint64 {
	n := uint64(1)
	for _, c := range a.chunkGrid() {
		n *= c
	}
	return n
}

// LUTBlocks is the number of lookup-table blocks; LUTStride their fixed size.
func (a Array) LUTBlocks() uint64 { return (a.NumChunks() + LUTBlockEntries) / LUTBlockEntries }

// LUTStride is the byte size reserved for every lookup-table block.
func (a Array) LUTStride() uint64 { return a.LUTSize / a.LUTBlocks() }
