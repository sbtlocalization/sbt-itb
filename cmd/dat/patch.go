// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	datarchive "github.com/sbtlocalization/sbt-itb/dat"
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

func runPatch(cmd *cobra.Command, args []string) error {
	inputPath, _ := cmd.Flags().GetString("input")
	outputPath, _ := cmd.Flags().GetString("output")
	verbose, _ := cmd.Flags().GetBool("verbose")
	return datarchive.Patch(inputPath, outputPath, verbose, cmd.ErrOrStderr())
}
