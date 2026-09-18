// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

	entries, err := datarchive.IndexArchiveSourceTree(inputPath, outputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory for DAT archive %s: %w", outputPath, err)
	}
	temporary, err := datarchive.CreateArchiveTemporary(outputPath)
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
