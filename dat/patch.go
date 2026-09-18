// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/sbtlocalization/sbt-itb/parser"
)

type originalArchiveSlot struct {
	path  string
	body  []byte
	empty bool
}

func Patch(sourceTreePath, archivePath string, reportProgress bool, progressWriter io.Writer) error {
	sourceEntries, err := IndexArchiveSourceTree(sourceTreePath, archivePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory for DAT archive %s: %w", archivePath, err)
	}
	originalSlots, originalMode, err := readOriginalArchive(archivePath)
	if err != nil {
		return err
	}
	if len(sourceEntries) == 0 {
		return nil
	}

	entries, handling := patchEntries(originalSlots, sourceEntries)
	temporary, err := CreateArchiveTemporary(archivePath)
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	replaced := false
	defer func() {
		if !replaced {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(originalMode.Perm()); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("failed to preserve permissions on temporary DAT archive %s: %w", temporaryPath, err)
	}
	var progress ProgressFunc
	if reportProgress {
		progress = func(current, total int, entryPath string) error {
			if _, err := fmt.Fprintf(progressWriter, "[%d/%d] %s %s\n", current, total, handling[entryPath], entryPath); err != nil {
				return fmt.Errorf("failed to write patching progress for archive entry %q: %w", entryPath, err)
			}
			return nil
		}
	}
	writeErr := Write(temporary, entries, progress)
	closeErr := temporary.Close()
	if writeErr != nil {
		if closeErr != nil {
			return errors.Join(writeErr, fmt.Errorf("failed to close temporary DAT archive %s: %w", temporaryPath, closeErr))
		}
		return writeErr
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close temporary DAT archive %s: %w", temporaryPath, closeErr)
	}
	if err := os.Rename(temporaryPath, archivePath); err != nil {
		return fmt.Errorf("failed to replace DAT archive %s: %w", archivePath, err)
	}
	replaced = true
	return nil
}

func readOriginalArchive(path string) ([]originalArchiveSlot, os.FileMode, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open existing DAT archive %s: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to inspect existing DAT archive %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("existing DAT archive %s is not a regular file", path)
	}

	archive := parser.NewFtlDat()
	if err := archive.Read(kaitai.NewStream(file), nil, archive); err != nil {
		return nil, 0, fmt.Errorf("failed to parse existing DAT archive %s: %w", path, err)
	}
	slots := make([]originalArchiveSlot, 0, len(archive.Files))
	paths := make(map[string]int)
	for i, entry := range archive.Files {
		if entry.OfsMeta == 0 {
			slots = append(slots, originalArchiveSlot{empty: true})
			continue
		}
		meta, err := entry.Meta()
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read archive entry %d metadata from existing DAT archive %s: %w", i+1, path, err)
		}
		if earlier, duplicate := paths[meta.Filename]; duplicate {
			return nil, 0, fmt.Errorf("duplicate archive path %q in existing DAT archive %s at slots %d and %d", meta.Filename, path, earlier, i+1)
		}
		paths[meta.Filename] = i + 1
		slots = append(slots, originalArchiveSlot{path: meta.Filename, body: meta.Body})
	}
	return slots, info.Mode(), nil
}

func patchEntries(original []originalArchiveSlot, source []Entry) ([]Entry, map[string]string) {
	remaining := make(map[string]Entry, len(source))
	for _, entry := range source {
		remaining[entry.Path] = entry
	}
	entries := make([]Entry, 0, len(original)+len(source))
	handling := make(map[string]string, len(original)+len(source))
	for _, slot := range original {
		if slot.empty {
			entries = append(entries, Entry{Empty: true})
			continue
		}
		if replacement, ok := remaining[slot.path]; ok {
			entries = append(entries, replacement)
			handling[slot.path] = "Replaced"
			delete(remaining, slot.path)
			continue
		}
		body := slot.body
		entries = append(entries, Entry{
			Path: slot.path,
			Size: uint64(len(body)),
			OpenBody: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(body)), nil
			},
		})
		handling[slot.path] = "Copied"
	}
	for _, entry := range source {
		if _, ok := remaining[entry.Path]; !ok {
			continue
		}
		entries = append(entries, entry)
		handling[entry.Path] = "Added"
	}
	return entries, handling
}
