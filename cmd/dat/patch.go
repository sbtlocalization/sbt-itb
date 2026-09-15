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
	datarchive "github.com/sbtlocalization/sbt-itb/dat"
	"github.com/sbtlocalization/sbt-itb/parser"
	"github.com/spf13/cobra"
)

func NewPatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch --input <directory> --output <DAT archive>",
		Short: "Patch a DAT archive from an archive source tree",
		Long: `Replaces matching archive entries from an archive source tree, preserves
unmatched entries, and appends new entries.`,
		Args: cobra.NoArgs,
		RunE: runPatch,
	}
	cmd.Flags().StringP("input", "i", "", "archive source tree `directory`")
	cmd.Flags().StringP("output", "o", "", "existing DAT archive `file`")
	cmd.Flags().BoolP("verbose", "v", false, "report each patched archive entry")
	cmd.MarkFlagRequired("input")
	cmd.MarkFlagRequired("output")
	cmd.MarkFlagDirname("input")
	cmd.MarkFlagFilename("output", "dat")
	return cmd
}

type originalArchiveSlot struct {
	path  string
	body  []byte
	empty bool
}

func runPatch(cmd *cobra.Command, args []string) error {
	inputPath, _ := cmd.Flags().GetString("input")
	outputPath, _ := cmd.Flags().GetString("output")
	verbose, _ := cmd.Flags().GetBool("verbose")

	sourceEntries, err := indexArchiveSourceTree(inputPath, outputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory for DAT archive %s: %w", outputPath, err)
	}
	originalSlots, originalMode, err := readOriginalArchive(outputPath)
	if err != nil {
		return err
	}
	if len(sourceEntries) == 0 {
		return nil
	}

	entries, handling := patchEntries(originalSlots, sourceEntries)
	temporary, err := createArchiveTemporary(outputPath)
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
	var progress datarchive.ProgressFunc
	if verbose {
		progress = func(current, total int, archivePath string) error {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "[%d/%d] %s %s\n", current, total, handling[archivePath], archivePath); err != nil {
				return fmt.Errorf("failed to write patching progress for archive entry %q: %w", archivePath, err)
			}
			return nil
		}
	}
	writeErr := datarchive.Write(temporary, entries, progress)
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
	if err := os.Rename(temporaryPath, outputPath); err != nil {
		return fmt.Errorf("failed to replace DAT archive %s: %w", outputPath, err)
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

func patchEntries(original []originalArchiveSlot, source []datarchive.Entry) ([]datarchive.Entry, map[string]string) {
	remaining := make(map[string]datarchive.Entry, len(source))
	for _, entry := range source {
		remaining[entry.Path] = entry
	}
	entries := make([]datarchive.Entry, 0, len(original)+len(source))
	handling := make(map[string]string, len(original)+len(source))
	for _, slot := range original {
		if slot.empty {
			entries = append(entries, datarchive.Entry{Empty: true})
			continue
		}
		if replacement, ok := remaining[slot.path]; ok {
			entries = append(entries, replacement)
			handling[slot.path] = "Replaced"
			delete(remaining, slot.path)
			continue
		}
		body := slot.body
		entries = append(entries, datarchive.Entry{
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
