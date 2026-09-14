// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package cmd

import "testing"

func TestRootContainsCSVMergeCommand(t *testing.T) {
	csvCommand, _, err := rootCmd.Find([]string{"csv"})
	if err != nil || csvCommand == rootCmd || csvCommand.Name() != "csv" {
		t.Fatalf("root csv command = (%v, %v), want csv group", csvCommand, err)
	}
	mergeCommand, _, err := rootCmd.Find([]string{"csv", "merge"})
	if err != nil || mergeCommand == csvCommand || mergeCommand.Name() != "merge" {
		t.Fatalf("root csv merge command = (%v, %v), want merge leaf", mergeCommand, err)
	}
}
