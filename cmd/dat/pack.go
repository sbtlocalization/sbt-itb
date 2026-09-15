// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	datarchive "github.com/sbtlocalization/sbt-itb/dat"
	"github.com/spf13/cobra"
)

func NewPackCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack --input <directory> --output <DAT archive>",
		Short: "Pack an archive source tree",
		Long: `Creates a DAT archive containing every regular file beneath an archive
source tree, identified by its relative path.`,
		Args: cobra.NoArgs,
		RunE: runPack,
	}
	cmd.Flags().StringP("input", "i", "", "archive source tree `directory`")
	cmd.Flags().StringP("output", "o", "", "output DAT archive `file`")
	cmd.Flags().BoolP("verbose", "v", false, "report each packed archive entry")
	cmd.MarkFlagRequired("input")
	cmd.MarkFlagRequired("output")
	cmd.MarkFlagDirname("input")
	cmd.MarkFlagFilename("output", "dat")
	return cmd
}

func runPack(cmd *cobra.Command, args []string) error {
	inputPath, _ := cmd.Flags().GetString("input")
	outputPath, _ := cmd.Flags().GetString("output")
	verbose, _ := cmd.Flags().GetBool("verbose")

	entries, err := indexArchiveSourceTree(inputPath, outputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory for DAT archive %s: %w", outputPath, err)
	}
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

	var progress datarchive.ProgressFunc
	if verbose {
		progress = func(current, total int, archivePath string) error {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "[%d/%d] Packed %s\n", current, total, archivePath); err != nil {
				return fmt.Errorf("failed to write packing progress for archive entry %q: %w", archivePath, err)
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

func indexArchiveSourceTree(inputPath, outputPath string) ([]datarchive.Entry, error) {
	rootInfo, err := os.Lstat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect archive source tree %s: %w", inputPath, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("archive source tree %s is a symbolic link", inputPath)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("archive source tree %s is not a directory", inputPath)
	}
	if err := rejectContainedOutput(inputPath, outputPath); err != nil {
		return nil, err
	}

	entries := make([]datarchive.Entry, 0)
	err = filepath.WalkDir(inputPath, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
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
		relativePath, err := filepath.Rel(inputPath, sourcePath)
		if err != nil {
			return fmt.Errorf("failed to derive archive path for source path %s: %w", sourcePath, err)
		}
		archivePath := filepath.ToSlash(relativePath)
		if !utf8.ValidString(archivePath) {
			return fmt.Errorf("archive path for source path %s is not valid UTF-8", sourcePath)
		}
		if info.Size() < 0 {
			return fmt.Errorf("source path %s has invalid size %d", sourcePath, info.Size())
		}

		pathToOpen := sourcePath
		entries = append(entries, datarchive.Entry{
			Path: archivePath,
			Size: uint64(info.Size()),
			OpenBody: func() (io.ReadCloser, error) {
				return os.Open(pathToOpen)
			},
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to index archive source tree %s: %w", inputPath, err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func rejectContainedOutput(inputPath, outputPath string) error {
	inputAbsolute, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve archive source tree %s: %w", inputPath, err)
	}
	outputAbsolute, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve output DAT archive %s: %w", outputPath, err)
	}
	relative, err := filepath.Rel(inputAbsolute, outputAbsolute)
	if err != nil {
		return nil
	}
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return fmt.Errorf("output DAT archive %s is located within archive source tree %s", outputPath, inputPath)
	}
	return nil
}

func createArchiveTemporary(outputPath string) (*os.File, error) {
	directory := filepath.Dir(outputPath)
	base := filepath.Base(outputPath)
	for range 10 {
		var randomBytes [8]byte
		if _, err := rand.Read(randomBytes[:]); err != nil {
			return nil, fmt.Errorf("failed to generate temporary DAT archive name: %w", err)
		}
		name := filepath.Join(directory, "."+base+"."+hex.EncodeToString(randomBytes[:])+".tmp")
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("failed to create temporary DAT archive beside %s: %w", outputPath, err)
		}
	}
	return nil, fmt.Errorf("failed to create unique temporary DAT archive beside %s", outputPath)
}
