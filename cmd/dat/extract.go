// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/sbtlocalization/sbt-itb/parser"
	"github.com/spf13/cobra"
)

func NewExtractCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "extract --input <DAT archive> --output <directory>",
		Aliases: []string{"ex"},
		Short:   "Extract archive entries",
		Long: `Extracts every archive entry beneath an output directory while preserving
its archive path.`,
		Args: cobra.NoArgs,
		RunE: runExtract,
	}
	cmd.Flags().StringP("input", "i", "", "input DAT archive `file`")
	cmd.Flags().StringP("output", "o", "", "output `directory`")
	cmd.Flags().BoolP("verbose", "v", false, "report each extracted archive entry")
	cmd.MarkFlagRequired("input")
	cmd.MarkFlagRequired("output")
	cmd.MarkFlagFilename("input", "dat")
	cmd.MarkFlagDirname("output")
	return cmd
}

func runExtract(cmd *cobra.Command, args []string) error {
	archivePath, _ := cmd.Flags().GetString("input")
	outputPath, _ := cmd.Flags().GetString("output")
	verbose, _ := cmd.Flags().GetBool("verbose")

	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open DAT archive %s: %w", archivePath, err)
	}
	defer file.Close()

	archive := parser.NewFtlDat()
	if err := archive.Read(kaitai.NewStream(file), nil, archive); err != nil {
		return fmt.Errorf("failed to parse DAT archive %s: %w", archivePath, err)
	}
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputPath, err)
	}

	total := 0
	for _, entry := range archive.Files {
		if entry.OfsMeta != 0 {
			total++
		}
	}

	current := 0
	for i, entry := range archive.Files {
		if entry.OfsMeta == 0 {
			if err := writeEmptySlotWarning(cmd, i); err != nil {
				return err
			}
			continue
		}

		meta, err := entry.Meta()
		if err != nil {
			return fmt.Errorf("failed to read archive entry %d metadata from DAT archive %s: %w", i+1, archivePath, err)
		}
		destination := filepath.Join(outputPath, filepath.FromSlash(meta.Filename))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			meta.Body = nil
			return fmt.Errorf("failed to create parent directory for archive entry %q: %w", meta.Filename, err)
		}
		err = os.WriteFile(destination, meta.Body, 0o644)
		meta.Body = nil
		if err != nil {
			return fmt.Errorf("failed to write archive entry %q to %s: %w", meta.Filename, destination, err)
		}

		current++
		if verbose {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "[%d/%d] Extracted %s\n", current, total, meta.Filename); err != nil {
				return fmt.Errorf("failed to write extraction progress for archive entry %q: %w", meta.Filename, err)
			}
		}
	}
	return nil
}
