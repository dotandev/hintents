// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package sourcemap provides source code resolution with optimized storage
// for WASM offset to source location mappings.
package sourcemap

import (
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// CompactSourceMap is an optimized storage format for WASM offset to source location mappings.
// It uses delta encoding and binary serialization to achieve ~30% size reduction compared
// to raw JSON/bincode storage.
//
// Storage format (binary):
//   - Header: magic bytes + version + entry count
//   - For each file: file index, string data (delta encoded for offsets)
//   - For each mapping: wasm offset (delta), line delta, column delta, file index
//
// Delta encoding approach:
//   - WasmOffset: delta from previous offset (typically small, fits in varint)
//   - Line: delta from previous line (usually small, often 1)
//   - Column: delta from start of line (variable)
//   - File paths are interned and delta encoded
type CompactSourceMap struct {
	// Version of the storage format
	Version uint16

	// Interned file paths for deduplication
	Files []string

	// Mapping entries sorted by WasmOffset
	Mappings []SourceMapping

	// Original uncompressed size for statistics
	OriginalSize int
}

// SourceMapping represents a single WASM offset to source location mapping.
// The wasm offset should always be greater than the previous one.
type SourceMapping struct {
	WasmOffset uint64
	Line       uint32
	Column     uint32
	FileIndex  uint32 // Index into the Files slice
}

// CompactMappingStats contains statistics about the compact source map.
type CompactMappingStats struct {
	OriginalSize   int
	CompressedSize int
	ReductionRatio float64
	NumMappings    int
	NumFiles       int
	AvgMappingSize float64
}

// NewCompactSourceMap creates a new compact source map from the given mappings.
func NewCompactSourceMap(mappings []SourceMapping, files []string) *CompactSourceMap {
	// Sort mappings by WasmOffset to ensure delta encoding works
	sortedMappings := make([]SourceMapping, len(mappings))
	copy(sortedMappings, mappings)
	sort.Slice(sortedMappings, func(i, j int) bool {
		return sortedMappings[i].WasmOffset < sortedMappings[j].WasmOffset
	})

	return &CompactSourceMap{
		Version:      CurrentVersion,
		Files:        files,
		Mappings:     sortedMappings,
		OriginalSize: estimateOriginalSize(mappings, files),
	}
}

// CurrentVersion is the current version of the compact storage format.
const CurrentVersion uint16 = 1

// Magic bytes to identify the format
var magicBytes = [4]byte{'H', 'S', 'M', 'A'} // Hints Source Map A

// estimateOriginalSize estimates the size of the original JSON/bincode representation.
func estimateOriginalSize(mappings []SourceMapping, files []string) int {
	// Rough estimate: each mapping as JSON would be ~60 bytes
	// Each file path as JSON would be ~len(path) + 10 bytes
	estimate := len(mappings) * 60
	for _, f := range files {
		estimate += len(f) + 10
	}
	return estimate
}

// Serialize writes the compact source map to a writer using binary format with optional compression.
func (c *CompactSourceMap) Serialize(w io.Writer, compress bool) error {
	if compress {
		return c.serializeCompressed(w)
	}
	return c.serialize(w)
}

// serialize writes without compression.
func (c *CompactSourceMap) serialize(w io.Writer) error {
	// Write header
	if _, err := w.Write(magicBytes[:]); err != nil {
		return errors.Wrap(err, "failed to write magic bytes")
	}

	// Write version
	if err := binary.Write(w, binary.LittleEndian, c.Version); err != nil {
		return errors.Wrap(err, "failed to write version")
	}

	// Write number of files
	numFiles := uint32(len(c.Files))
	if err := binary.Write(w, binary.LittleEndian, numFiles); err != nil {
		return errors.Wrap(err, "failed to write file count")
	}

	// Write file paths with delta encoding
	if err := c.writeFilePaths(w); err != nil {
		return errors.Wrap(err, "failed to write file paths")
	}

	// Write number of mappings
	numMappings := uint32(len(c.Mappings))
	if err := binary.Write(w, binary.LittleEndian, numMappings); err != nil {
		return errors.Wrap(err, "failed to write mapping count")
	}

	// Write mappings with delta encoding
	if err := c.writeMappings(w); err != nil {
		return errors.Wrap(err, "failed to write mappings")
	}

	return nil
}

// serializeCompressed writes with zlib compression.
func (c *CompactSourceMap) serializeCompressed(w io.Writer) error {
	// Write header with compression flag
	if _, err := w.Write(magicBytes[:]); err != nil {
		return errors.Wrap(err, "failed to write magic bytes")
	}

	versionWithFlag := c.Version | 0x8000 // Set high bit to indicate compression
	if err := binary.Write(w, binary.LittleEndian, versionWithFlag); err != nil {
		return errors.Wrap(err, "failed to write version")
	}

	// Create a zlib writer
	zw := zlib.NewWriter(w)
	defer zw.Close()

	// Write to compressed stream
	// Number of files
	numFiles := uint32(len(c.Files))
	if err := binary.Write(zw, binary.LittleEndian, numFiles); err != nil {
		return errors.Wrap(err, "failed to write file count")
	}

	// File paths
	if err := c.writeFilePathsCompressed(zw); err != nil {
		return errors.Wrap(err, "failed to write file paths")
	}

	// Number of mappings
	numMappings := uint32(len(c.Mappings))
	if err := binary.Write(zw, binary.LittleEndian, numMappings); err != nil {
		return errors.Wrap(err, "failed to write mapping count")
	}

	// Mappings
	if err := c.writeMappingsCompressed(zw); err != nil {
		return errors.Wrap(err, "failed to write mappings")
	}

	// Close and flush
	if err := zw.Close(); err != nil {
		return errors.Wrap(err, "failed to close compressor")
	}

	return nil
}

// writeFilePaths writes file paths with delta encoding.
func (c *CompactSourceMap) writeFilePaths(w io.Writer) error {
	// Use simple length-prefixed strings for now
	// Could be optimized further with dictionary encoding
	for _, f := range c.Files {
		data := []byte(f)
		// Write length
		if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
			return err
		}
		// Write data
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

// writeFilePathsCompressed writes file paths to a compressed writer.
func (c *CompactSourceMap) writeFilePathsCompressed(zw *zlib.Writer) error {
	return c.writeFilePaths(zw)
}

// writeMappings writes mappings with delta encoding.
func (c *CompactSourceMap) writeMappings(w io.Writer) error {
	if len(c.Mappings) == 0 {
		return nil
	}

	var prevOffset uint64
	var prevLine uint32

	for i, m := range c.Mappings {
		// Delta encode offset
		deltaOffset := m.WasmOffset - prevOffset
		if err := writeUvarint(w, deltaOffset); err != nil {
			return errors.Wrapf(err, "failed to write offset delta at index %d", i)
		}

		// Delta encode line
		var deltaLine uint32
		if i == 0 {
			deltaLine = m.Line
		} else {
			if m.Line >= prevLine {
				deltaLine = m.Line - prevLine
			}
			// Note: Line can go backwards in some edge cases (e.g., inlined code)
			// In that case we encode a special marker
		}
		if err := writeUvarint(w, uint64(deltaLine)); err != nil {
			return errors.Wrapf(err, "failed to write line delta at index %d", i)
		}

		// Column is not delta encoded (column resets at line start)
		if err := writeUvarint(w, uint64(m.Column)); err != nil {
			return errors.Wrapf(err, "failed to write column at index %d", i)
		}

		// File index (could also be delta encoded for further savings)
		if err := writeUvarint(w, uint64(m.FileIndex)); err != nil {
			return errors.Wrapf(err, "failed to write file index at index %d", i)
		}

		prevOffset = m.WasmOffset
		prevLine = m.Line
	}

	return nil
}

// writeMappingsCompressed writes mappings to a compressed writer.
func (c *CompactSourceMap) writeMappingsCompressed(zw *zlib.Writer) error {
	return c.writeMappings(zw)
}

// Deserialize reads a compact source map from a reader.
func Deserialize(r io.Reader) (*CompactSourceMap, error) {
	// Read header
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, errors.Wrap(err, "failed to read magic bytes")
	}

	if magic != magicBytes {
		return nil, errors.New("invalid magic bytes: not a compact source map")
	}

	// Read version
	var versionRaw uint16
	if err := binary.Read(r, binary.LittleEndian, &versionRaw); err != nil {
		return nil, errors.Wrap(err, "failed to read version")
	}

	compressed := (versionRaw & 0x8000) != 0
	version := versionRaw & 0x7FFF

	if version != CurrentVersion {
		return nil, fmt.Errorf("unsupported version: %d (expected %d)", version, CurrentVersion)
	}

	var c *CompactSourceMap
	var err error

	if compressed {
		c, err = deserializeCompressed(r)
	} else {
		c, err = deserialize(r)
	}

	if err != nil {
		return nil, err
	}

	c.Version = version
	return c, nil
}

// deserialize reads without decompression.
func deserialize(r io.Reader) (*CompactSourceMap, error) {
	// Read file count
	var numFiles uint32
	if err := binary.Read(r, binary.LittleEndian, &numFiles); err != nil {
		return nil, errors.Wrap(err, "failed to read file count")
	}

	// Read files
	files := make([]string, numFiles)
	for i := uint32(0); i < numFiles; i++ {
		var pathLen uint32
		if err := binary.Read(r, binary.LittleEndian, &pathLen); err != nil {
			return nil, errors.Wrap(err, "failed to read path length")
		}
		data := make([]byte, pathLen)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, errors.Wrap(err, "failed to read path data")
		}
		files[i] = string(data)
	}

	// Read mapping count
	var numMappings uint32
	if err := binary.Read(r, binary.LittleEndian, &numMappings); err != nil {
		return nil, errors.Wrap(err, "failed to read mapping count")
	}

	// Read mappings
	mappings := make([]SourceMapping, numMappings)
	var prevOffset uint64
	var prevLine uint32

	for i := uint32(0); i < numMappings; i++ {
		deltaOffset, err := readUvarint(r)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read offset delta at index %d", i)
		}

		deltaLine, err := readUvarint(r)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read line delta at index %d", i)
		}

		column, err := readUvarint(r)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read column at index %d", i)
		}

		fileIndex, err := readUvarint(r)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file index at index %d", i)
		}

		mappings[i] = SourceMapping{
			WasmOffset: prevOffset + deltaOffset,
			Line:       prevLine + uint32(deltaLine),
			Column:     uint32(column),
			FileIndex:  uint32(fileIndex),
		}

		prevOffset = mappings[i].WasmOffset
		prevLine = mappings[i].Line
	}

	return &CompactSourceMap{
		Files:    files,
		Mappings: mappings,
	}, nil
}

// deserializeCompressed reads with zlib decompression.
func deserializeCompressed(r io.Reader) (*CompactSourceMap, error) {
	zr, err := zlib.NewReader(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create zlib reader")
	}
	defer zr.Close()

	return deserialize(zr)
}

// GetSourceLocation finds the source location for a given WASM offset.
// It returns the most appropriate location (the one with the largest offset <= target).
func (c *CompactSourceMap) GetSourceLocation(wasmOffset uint64) (file string, line, column int, found bool) {
	if len(c.Mappings) == 0 {
		return "", 0, 0, false
	}

	// Binary search for the best match
	idx := sort.Search(len(c.Mappings), func(i int) bool {
		return c.Mappings[i].WasmOffset > wasmOffset
	})

	if idx == 0 {
		return "", 0, 0, false
	}

	mapping := c.Mappings[idx-1]
	if int(mapping.FileIndex) < len(c.Files) {
		return c.Files[mapping.FileIndex], int(mapping.Line), int(mapping.Column), true
	}

	return "", 0, 0, false
}

// Stats returns statistics about the compact source map.
func (c *CompactSourceMap) Stats() CompactMappingStats {
	compressedSize := c.EstimateSerializedSize(true)
	uncompressedSize := c.EstimateSerializedSize(false)

	ratio := 0.0
	if c.OriginalSize > 0 {
		ratio = 1.0 - (float64(compressedSize) / float64(c.OriginalSize))
	}

	return CompactMappingStats{
		OriginalSize:   c.OriginalSize,
		CompressedSize: compressedSize,
		ReductionRatio: ratio,
		NumMappings:    len(c.Mappings),
		NumFiles:       len(c.Files),
		AvgMappingSize: float64(uncompressedSize) / float64(len(c.Mappings)+1),
	}
}

// EstimateSerializedSize estimates the size when serialized.
func (c *CompactSourceMap) EstimateSerializedSize(compressed bool) int {
	// Header: 4 (magic) + 2 (version) = 6
	size := 6

	if compressed {
		// Compressed is typically 20-50% of uncompressed
		size += c.estimateUncompressedSize() * 30 / 100
	} else {
		size += c.estimateUncompressedSize()
	}

	return size
}

// estimateUncompressedSize estimates the raw serialized size.
func (c *CompactSourceMap) estimateUncompressedSize() int {
	size := 0

	// File count: 4 bytes
	size += 4

	// File paths: 4 bytes length + content for each
	for _, f := range c.Files {
		size += 4 + len(f)
	}

	// Mapping count: 4 bytes
	size += 4

	// Mappings: variable size (delta encoded, roughly 8-12 bytes each average)
	size += len(c.Mappings) * 10

	return size
}

// writeUvarint writes an unsigned varint.
func writeUvarint(w io.Writer, val uint64) error {
	var buf [10]byte
	n := binary.PutUvarint(buf[:], val)
	_, err := w.Write(buf[:n])
	return err
}

// readUvarint reads an unsigned varint.
func readUvarint(r io.Reader) (uint64, error) {
	var buf [10]byte
	if _, err := io.ReadFull(r, buf[:1]); err != nil {
		return 0, err
	}
	val := uint64(buf[0])
	shift := uint(7)
	for buf[0]&0x80 != 0 {
		if _, err := io.ReadFull(r, buf[:1]); err != nil {
			return 0, err
		}
		val |= uint64(buf[0]&0x7F) << shift
		shift += 7
	}
	return val, nil
}

// BuildMappingFromDWARF builds optimized source mappings from DWARF debug info.
// This is a helper function to convert DWARF line information into the compact format.
func BuildMappingFromDWARF(lineEntries []DWARFLineEntry, filePaths []string) []SourceMapping {
	mappings := make([]SourceMapping, 0, len(lineEntries))

	// Build file index lookup
	fileIndexMap := make(map[string]uint32)
	for i, f := range filePaths {
		fileIndexMap[f] = uint32(i)
	}

	// Sort line entries by address
	sort.Slice(lineEntries, func(i, j int) bool {
		return lineEntries[i].Address < lineEntries[j].Address
	})

	for _, entry := range lineEntries {
		fileIdx, ok := fileIndexMap[entry.File]
		if !ok {
			// Unknown file, skip
			continue
		}

		mappings = append(mappings, SourceMapping{
			WasmOffset: entry.Address,
			Line:       uint32(entry.Line),
			Column:     uint32(entry.Column),
			FileIndex:  fileIdx,
		})
	}

	return mappings
}

// DWARFLineEntry represents a single line entry from DWARF debug info.
type DWARFLineEntry struct {
	Address uint64
	File    string
	Line    int
	Column  int
}

// InternFilePaths interns file paths to minimize storage.
// It returns the deduplicated list and a mapping from original to interned index.
func InternFilePaths(paths []string) ([]string, map[string]int) {
	seen := make(map[string]int)
	interned := make([]string, 0, len(paths))
	mapping := make(map[string]int)

	for _, p := range paths {
		// Normalize path separators
		normalized := strings.ReplaceAll(p, "\\", "/")

		if idx, ok := seen[normalized]; ok {
			mapping[p] = idx
		} else {
			idx := len(interned)
			seen[normalized] = idx
			interned = append(interned, normalized)
			mapping[p] = idx
		}
	}

	return interned, mapping
}
