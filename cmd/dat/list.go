// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/sbtlocalization/sbt-itb/parser"
	"github.com/spf13/cobra"
)

func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list --input <DAT archive>",
		Aliases: []string{"ls"},
		Short:   "List archive entries",
		Long: `Shows which archive entries a DAT archive contains so its contents can be
inspected without extracting them.`,
		Args: cobra.NoArgs,
		RunE: runList,
	}
	cmd.Flags().StringP("input", "i", "", "input DAT archive `file`")
	cmd.Flags().BoolP("json", "j", false, "output archive entries as JSONL")
	cmd.MarkFlagRequired("input")
	cmd.MarkFlagFilename("input", "dat")
	return cmd
}

type listEntry struct {
	Path string `json:"path"`
	Size uint32 `json:"size"`
}

func runList(cmd *cobra.Command, args []string) error {
	archivePath, _ := cmd.Flags().GetString("input")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open DAT archive %s: %w", archivePath, err)
	}
	defer file.Close()

	archive := parser.NewFtlDat()
	if err := archive.Read(kaitai.NewStream(file), nil, archive); err != nil {
		return fmt.Errorf("failed to parse DAT archive %s: %w", archivePath, err)
	}

	if jsonOutput {
		return writeJSONL(cmd, archive, archivePath)
	}
	return writeTable(cmd, archive, archivePath)
}

func writeJSONL(cmd *cobra.Command, archive *parser.FtlDat, archivePath string) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
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
		if err := encoder.Encode(listEntry{Path: meta.Filename, Size: meta.LenBody}); err != nil {
			return fmt.Errorf("failed to encode archive entry %d: %w", i+1, err)
		}
		meta.Body = nil
	}
	return nil
}

func writeTable(cmd *cobra.Command, archive *parser.FtlDat, archivePath string) error {
	output := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(output, "Size\tPath")
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
		fmt.Fprintf(output, "%d\t%s\n", meta.LenBody, meta.Filename)
		meta.Body = nil
	}
	if err := output.Flush(); err != nil {
		return fmt.Errorf("failed to write archive entry table: %w", err)
	}
	return nil
}

func writeEmptySlotWarning(cmd *cobra.Command, index int) error {
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Warning: empty archive slot %d\n", index+1); err != nil {
		return fmt.Errorf("failed to write empty archive slot warning: %w", err)
	}
	return nil
}
