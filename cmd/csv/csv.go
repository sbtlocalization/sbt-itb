// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package csv

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "csv",
		Short: "Work with localization CSV files",
		Long:  "Combine and transform localization entries stored in CSV files.",
	}
	cmd.AddCommand(NewMergeCommand())
	return cmd
}
