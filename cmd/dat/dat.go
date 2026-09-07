// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dat",
		Short: "Work with DAT archives",
		Long:  "Inspect and extract entries stored in DAT archives.",
	}
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewExtractCommand())
	return cmd
}
