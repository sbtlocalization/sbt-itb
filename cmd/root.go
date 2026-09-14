// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package cmd

import (
	"os"

	"github.com/sbtlocalization/sbt-itb/cmd/csv"
	"github.com/sbtlocalization/sbt-itb/cmd/dat"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sbt-itb",
	Short: "A set of tools for Into the Breach game",
	Long: `SBT ITB Tools is a collection of utilities designed to assist with the localization and 
modification of the game Into the Breach.`,
	Version: "1",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(csv.NewCommand())
	rootCmd.AddCommand(dat.NewCommand())
}
