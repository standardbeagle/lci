package testing

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/standardbeagle/lci/internal/types"
)

// BinarySnapshot represents a binary serialization of index state
type BinarySnapshot struct {
	Checksum [32]byte
	Data     []byte
}

// TrigramSnapshot captures TrigramIndex state for comparison
type TrigramSnapshot struct {
	TrigramCount int
	FileCount    int
	Trigrams     []TrigramEntrySnapshot
}

// TrigramEntrySnapshot represents a single trigram entry
type TrigramEntrySnapshot struct {
	Hash      uint64
	Locations []LocationSnapshot
}

// LocationSnapshot represents a file location
type LocationSnapshot struct {
	FileID types.FileID
	Offset uint32
}

// SnapshotTrigramIndexData creates a binary snapshot from trigram data
func SnapshotTrigramIndexData(fileCount int, trigramData map[uint64][]LocationSnapshot) (*BinarySnapshot, error) {
	var buf bytes.Buffer

	// Write version header
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		return nil, fmt.Errorf("failed to write version: %w", err)
	}

	// Write file count
	if err := binary.Write(&buf, binary.LittleEndian, uint32(fileCount)); err != nil {
		return nil, fmt.Errorf("failed to write file count: %w", err)
	}

	// Write trigram count
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(trigramData))); err != nil {
		return nil, fmt.Errorf("failed to write trigram count: %w", err)
	}

	// Write trigrams in sorted order for deterministic output
	var hashes []uint64
	for hash := range trigramData {
		hashes = append(hashes, hash)
	}
	sort.Slice(hashes, func(i, j int) bool { return hashes[i] < hashes[j] })

	for _, hash := range hashes {
		locations := trigramData[hash]

		// Write trigram hash
		if err := binary.Write(&buf, binary.LittleEndian, hash); err != nil {
			return nil, fmt.Errorf("failed to write trigram hash: %w", err)
		}

		// Write location count
		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(locations))); err != nil {
			return nil, fmt.Errorf("failed to write location count: %w", err)
		}

		// Write locations
		for _, loc := range locations {
			if err := binary.Write(&buf, binary.LittleEndian, uint32(loc.FileID)); err != nil {
				return nil, fmt.Errorf("failed to write file ID: %w", err)
			}
			if err := binary.Write(&buf, binary.LittleEndian, loc.Offset); err != nil {
				return nil, fmt.Errorf("failed to write offset: %w", err)
			}
		}
	}

	// Calculate checksum
	data := buf.Bytes()
	checksum := sha256.Sum256(data)

	return &BinarySnapshot{
		Checksum: checksum,
		Data:     data,
	}, nil
}

// CompareBinarySnapshots compares two binary snapshots for equality
func CompareBinarySnapshots(snap1, snap2 *BinarySnapshot) bool {
	if snap1 == nil || snap2 == nil {
		return snap1 == snap2
	}
	return snap1.Checksum == snap2.Checksum
}

// SnapshotDiff returns differences between two snapshots
func SnapshotDiff(snap1, snap2 *BinarySnapshot) (*SnapshotDifference, error) {
	if snap1 == nil || snap2 == nil {
		return &SnapshotDifference{
			ChecksumMatch: false,
			SizeMatch:     false,
			Message:       "One or both snapshots are nil",
		}, nil
	}

	diff := &SnapshotDifference{
		ChecksumMatch: snap1.Checksum == snap2.Checksum,
		SizeMatch:     len(snap1.Data) == len(snap2.Data),
	}

	if diff.ChecksumMatch {
		diff.Message = "Snapshots are identical"
		return diff, nil
	}

	// Analyze differences
	if !diff.SizeMatch {
		diff.Message = fmt.Sprintf("Size difference: %d vs %d bytes",
			len(snap1.Data), len(snap2.Data))
	} else {
		diff.Message = "Same size but different content"

		// Find first differing byte
		for i := 0; i < len(snap1.Data) && i < len(snap2.Data); i++ {
			if snap1.Data[i] != snap2.Data[i] {
				diff.FirstDifferenceOffset = i
				diff.Message += fmt.Sprintf(" (first diff at byte %d)", i)
				break
			}
		}
	}

	return diff, nil
}

// SnapshotDifference describes differences between snapshots
type SnapshotDifference struct {
	ChecksumMatch         bool
	SizeMatch             bool
	FirstDifferenceOffset int
	Message               string
}

// WriteBinarySnapshot writes a snapshot to a writer
func WriteBinarySnapshot(w io.Writer, snap *BinarySnapshot) error {
	if snap == nil {
		return errors.New("snapshot is nil")
	}

	// Write checksum
	if _, err := w.Write(snap.Checksum[:]); err != nil {
		return fmt.Errorf("failed to write checksum: %w", err)
	}

	// Write data length
	if err := binary.Write(w, binary.LittleEndian, uint32(len(snap.Data))); err != nil {
		return fmt.Errorf("failed to write data length: %w", err)
	}

	// Write data
	if _, err := w.Write(snap.Data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}

// ReadBinarySnapshot reads a snapshot from a reader
func ReadBinarySnapshot(r io.Reader) (*BinarySnapshot, error) {
	snap := &BinarySnapshot{}

	// Read checksum
	if _, err := io.ReadFull(r, snap.Checksum[:]); err != nil {
		return nil, fmt.Errorf("failed to read checksum: %w", err)
	}

	// Read data length
	var dataLen uint32
	if err := binary.Read(r, binary.LittleEndian, &dataLen); err != nil {
		return nil, fmt.Errorf("failed to read data length: %w", err)
	}

	// Read data
	snap.Data = make([]byte, dataLen)
	if _, err := io.ReadFull(r, snap.Data); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	// Verify checksum
	actualChecksum := sha256.Sum256(snap.Data)
	if actualChecksum != snap.Checksum {
		return nil, fmt.Errorf("checksum mismatch: expected %x, got %x",
			snap.Checksum, actualChecksum)
	}

	return snap, nil
}

// CreateTrigramTestSnapshot creates a snapshot from test data for validation
func CreateTrigramTestSnapshot(trigrams map[uint64][]LocationSnapshot) *TrigramSnapshot {
	snapshot := &TrigramSnapshot{
		TrigramCount: len(trigrams),
		Trigrams:     make([]TrigramEntrySnapshot, 0, len(trigrams)),
	}

	// Track unique files
	fileSet := make(map[types.FileID]bool)

	// Sort trigrams by hash for deterministic output
	var hashes []uint64
	for hash := range trigrams {
		hashes = append(hashes, hash)
	}
	sort.Slice(hashes, func(i, j int) bool { return hashes[i] < hashes[j] })

	// Build snapshot entries
	for _, hash := range hashes {
		locations := trigrams[hash]

		// Sort locations for deterministic output
		sortedLocs := make([]LocationSnapshot, len(locations))
		copy(sortedLocs, locations)
		sort.Slice(sortedLocs, func(i, j int) bool {
			if sortedLocs[i].FileID != sortedLocs[j].FileID {
				return sortedLocs[i].FileID < sortedLocs[j].FileID
			}
			return sortedLocs[i].Offset < sortedLocs[j].Offset
		})

		snapshot.Trigrams = append(snapshot.Trigrams, TrigramEntrySnapshot{
			Hash:      hash,
			Locations: sortedLocs,
		})

		// Track files
		for _, loc := range locations {
			fileSet[loc.FileID] = true
		}
	}

	snapshot.FileCount = len(fileSet)
	return snapshot
}

// ValidateSnapshotIntegrity performs comprehensive validation of snapshot data
func ValidateSnapshotIntegrity(snap *BinarySnapshot) error {
	if snap == nil {
		return errors.New("snapshot is nil")
	}

	if len(snap.Data) == 0 {
		return errors.New("snapshot data is empty")
	}

	// Verify checksum
	actualChecksum := sha256.Sum256(snap.Data)
	if actualChecksum != snap.Checksum {
		return errors.New("checksum validation failed")
	}

	// Validate data structure
	buf := bytes.NewReader(snap.Data)

	// Check version
	var version uint32
	if err := binary.Read(buf, binary.LittleEndian, &version); err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}

	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	// Check file count
	var fileCount uint32
	if err := binary.Read(buf, binary.LittleEndian, &fileCount); err != nil {
		return fmt.Errorf("failed to read file count: %w", err)
	}

	// More validation can be added as the format is extended

	return nil
}
