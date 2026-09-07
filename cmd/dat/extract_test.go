// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExtractCreatesArchiveHierarchyWithExactBytesAndIsQuiet(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{path: "img/ui/flask.png", body: []byte{0, 1, 2, 255}},
		{path: "scripts/init.lua", body: []byte("return true\n")},
	})
	outputPath := filepath.Join(t.TempDir(), "missing", "output")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewExtractCommand()
	cmd.SetArgs([]string{"--input", archivePath, "--output", outputPath})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertFileBytes(t, filepath.Join(outputPath, "img", "ui", "flask.png"), []byte{0, 1, 2, 255})
	assertFileBytes(t, filepath.Join(outputPath, "scripts", "init.lua"), []byte("return true\n"))
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestExtractReportsVerboseProgressAndWarnsAboutEmptySlots(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{path: "first.txt", body: []byte{1}},
		{empty: true},
		{path: "nested/second.txt", body: []byte{2}},
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewExtractCommand()
	cmd.SetArgs([]string{"--input", archivePath, "--output", t.TempDir(), "--verbose"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	want := "[1/2] Extracted first.txt\nWarning: empty archive slot 2\n[2/2] Extracted nested/second.txt\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestExtractWarnsAboutEmptySlotsWithoutVerboseOutput(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{empty: true},
		{path: "only.txt", body: []byte{1}},
	})
	var stderr bytes.Buffer
	cmd := NewExtractCommand()
	cmd.SetArgs([]string{"--input", archivePath, "--output", t.TempDir()})
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stderr.String() != "Warning: empty archive slot 1\n" {
		t.Errorf("stderr = %q, want empty slot warning", stderr.String())
	}
}

func TestExtractRequiresFlagsAndRejectsPositionalArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing flags", args: nil},
		{name: "missing input", args: []string{"--output", "out"}},
		{name: "missing output", args: []string{"--input", "archive.dat"}},
		{name: "positional argument", args: []string{"--input", "archive.dat", "--output", "out", "entry.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewExtractCommand()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err == nil {
				t.Fatal("Execute() error = nil, want flag or argument error")
			}
		})
	}
}

func TestExtractUsesExAliasAndCompletionMetadata(t *testing.T) {
	cmd := NewExtractCommand()

	if len(cmd.Aliases) != 1 || cmd.Aliases[0] != "ex" {
		t.Errorf("Aliases = %v, want [ex]", cmd.Aliases)
	}
	input := cmd.Flags().Lookup("input")
	if input == nil {
		t.Fatal("input flag is missing")
	}
	if input.Shorthand != "i" {
		t.Errorf("input shorthand = %q, want i", input.Shorthand)
	}
	extensions := input.Annotations[cobra.BashCompFilenameExt]
	if len(extensions) != 1 || extensions[0] != "dat" {
		t.Errorf("input filename extensions = %v, want [dat]", extensions)
	}
	output := cmd.Flags().Lookup("output")
	if output == nil {
		t.Fatal("output flag is missing")
	}
	if output.Shorthand != "o" {
		t.Errorf("output shorthand = %q, want o", output.Shorthand)
	}
	if _, ok := output.Annotations[cobra.BashCompSubdirsInDir]; !ok {
		t.Errorf("output annotations = %v, want directory completion", output.Annotations)
	}
	verbose := cmd.Flags().Lookup("verbose")
	if verbose == nil || verbose.Shorthand != "v" {
		t.Errorf("verbose flag = %v, want -v shorthand", verbose)
	}
}

func TestExtractKeepsEarlierFilesWhenLaterEntryFailsToParse(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{path: "first.txt", body: []byte{1}},
		{path: "broken.txt", body: []byte{2, 3}},
	})
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, data[:len(data)-1], 0o600); err != nil {
		t.Fatal(err)
	}
	outputPath := t.TempDir()
	cmd := NewExtractCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", archivePath, "--output", outputPath})

	err = cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want parser error")
	}
	want := "failed to read archive entry 2 metadata from DAT archive " + archivePath
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want context %q", err, want)
	}
	assertFileBytes(t, filepath.Join(outputPath, "first.txt"), []byte{1})
	if _, statErr := os.Stat(filepath.Join(outputPath, "broken.txt")); !os.IsNotExist(statErr) {
		t.Errorf("broken entry stat error = %v, want not exist", statErr)
	}
}

func TestExtractKeepsEarlierFilesWhenLaterDestinationFails(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{path: "first.txt", body: []byte{1}},
		{path: "blocked/second.txt", body: []byte{2}},
	})
	outputPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputPath, "blocked"), []byte("conflict"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewExtractCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", archivePath, "--output", outputPath})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want filesystem error")
	}
	if !strings.Contains(err.Error(), "failed to create parent directory for archive entry \"blocked/second.txt\"") {
		t.Errorf("error = %q, want archive entry context", err)
	}
	assertFileBytes(t, filepath.Join(outputPath, "first.txt"), []byte{1})
	assertFileBytes(t, filepath.Join(outputPath, "blocked"), []byte("conflict"))
}

func TestExtractOverwritesExistingRegularFile(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{{path: "scripts/init.lua", body: []byte("new content")}})
	outputPath := t.TempDir()
	destination := filepath.Join(outputPath, "scripts", "init.lua")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old content that is longer"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewExtractCommand()
	cmd.SetArgs([]string{"--input", archivePath, "--output", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertFileBytes(t, destination, []byte("new content"))
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ReadFile(%q) = %v, want %v", path, got, want)
	}
}
