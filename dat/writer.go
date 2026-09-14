// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type Entry struct {
	Path     string
	Size     uint64
	OpenBody func() (io.ReadCloser, error)
}

type ProgressFunc func(current, total int, path string) error

func Write(writer io.Writer, entries []Entry, progress ProgressFunc) error {
	offsets, err := metadataOffsets(entries)
	if err != nil {
		return err
	}
	if err := writeUint32(writer, uint32(len(entries))); err != nil {
		return fmt.Errorf("failed to write archive entry count: %w", err)
	}
	for i, offset := range offsets {
		if err := writeUint32(writer, offset); err != nil {
			return fmt.Errorf("failed to write archive entry %q metadata offset: %w", entries[i].Path, err)
		}
	}
	for i, entry := range entries {
		if err := writeUint32(writer, uint32(entry.Size)); err != nil {
			return fmt.Errorf("failed to write archive entry %q body length: %w", entry.Path, err)
		}
		if err := writeUint32(writer, uint32(len(entry.Path))); err != nil {
			return fmt.Errorf("failed to write archive entry %q path length: %w", entry.Path, err)
		}
		if err := writeString(writer, entry.Path); err != nil {
			return fmt.Errorf("failed to write archive entry %q path: %w", entry.Path, err)
		}
		if err := writeBody(writer, entry); err != nil {
			return err
		}
		if progress != nil {
			if err := progress(i+1, len(entries), entry.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

func metadataOffsets(entries []Entry) ([]uint32, error) {
	if uint64(len(entries)) > math.MaxUint32 {
		return nil, fmt.Errorf("archive entry count %d exceeds DAT uint32 limit", len(entries))
	}
	offset := uint64(4) + uint64(len(entries))*4
	offsets := make([]uint32, len(entries))
	for i, entry := range entries {
		if uint64(len(entry.Path)) > math.MaxUint32 {
			return nil, fmt.Errorf("archive entry %q path length exceeds DAT uint32 limit", entry.Path)
		}
		if entry.Size > math.MaxUint32 {
			return nil, fmt.Errorf("archive entry %q body length %d exceeds DAT uint32 limit", entry.Path, entry.Size)
		}
		if offset > math.MaxUint32 {
			return nil, fmt.Errorf("archive entry %q metadata offset %d exceeds DAT uint32 limit", entry.Path, offset)
		}
		offsets[i] = uint32(offset)
		offset += 8 + uint64(len(entry.Path)) + entry.Size
	}
	return offsets, nil
}

func writeBody(writer io.Writer, entry Entry) error {
	body, err := entry.OpenBody()
	if err != nil {
		return fmt.Errorf("failed to open archive entry %q body: %w", entry.Path, err)
	}
	_, copyErr := io.CopyN(writer, body, int64(entry.Size))
	closeErr := body.Close()
	if copyErr != nil {
		if closeErr != nil {
			return fmt.Errorf("failed to write archive entry %q body: %w; failed to close body: %v", entry.Path, copyErr, closeErr)
		}
		return fmt.Errorf("failed to write archive entry %q body: %w", entry.Path, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close archive entry %q body: %w", entry.Path, closeErr)
	}
	return nil
}

func writeUint32(writer io.Writer, value uint32) error {
	var encoded [4]byte
	binary.LittleEndian.PutUint32(encoded[:], value)
	written, err := writer.Write(encoded[:])
	if err == nil && written != len(encoded) {
		return io.ErrShortWrite
	}
	return err
}

func writeString(writer io.Writer, value string) error {
	written, err := io.WriteString(writer, value)
	if err == nil && written != len(value) {
		return io.ErrShortWrite
	}
	return err
}
