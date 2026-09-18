// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

func IndexArchiveSourceTree(sourceTreePath, archivePath string) ([]Entry, error) {
	rootInfo, err := os.Lstat(sourceTreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect archive source tree %s: %w", sourceTreePath, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("archive source tree %s is a symbolic link", sourceTreePath)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("archive source tree %s is not a directory", sourceTreePath)
	}
	if err := rejectContainedOutput(sourceTreePath, archivePath); err != nil {
		return nil, err
	}

	entries := make([]Entry, 0)
	err = filepath.WalkDir(sourceTreePath, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("failed to access source path %s: %w", sourcePath, walkErr)
		}
		if entry.IsDir() || filepath.Base(sourcePath) == ".DS_Store" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("failed to inspect source path %s: %w", sourcePath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("source path %s is a symbolic link", sourcePath)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("source path %s is not a regular file (%s)", sourcePath, info.Mode().Type())
		}
		relativePath, err := filepath.Rel(sourceTreePath, sourcePath)
		if err != nil {
			return fmt.Errorf("failed to derive archive path for source path %s: %w", sourcePath, err)
		}
		archiveEntryPath := filepath.ToSlash(relativePath)
		if !utf8.ValidString(archiveEntryPath) {
			return fmt.Errorf("archive path for source path %s is not valid UTF-8", sourcePath)
		}
		if info.Size() < 0 {
			return fmt.Errorf("source path %s has invalid size %d", sourcePath, info.Size())
		}

		pathToOpen := sourcePath
		entries = append(entries, Entry{
			Path: archiveEntryPath,
			Size: uint64(info.Size()),
			OpenBody: func() (io.ReadCloser, error) {
				return os.Open(pathToOpen)
			},
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to index archive source tree %s: %w", sourceTreePath, err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func rejectContainedOutput(sourceTreePath, archivePath string) error {
	inputAbsolute, err := filepath.Abs(sourceTreePath)
	if err != nil {
		return fmt.Errorf("failed to resolve archive source tree %s: %w", sourceTreePath, err)
	}
	outputAbsolute, err := filepath.Abs(archivePath)
	if err != nil {
		return fmt.Errorf("failed to resolve output DAT archive %s: %w", archivePath, err)
	}
	relative, err := filepath.Rel(inputAbsolute, outputAbsolute)
	if err != nil {
		return nil
	}
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return fmt.Errorf("output DAT archive %s is located within archive source tree %s", archivePath, sourceTreePath)
	}
	return nil
}
