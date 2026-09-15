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
)

func TestPatchRejectsDuplicateOriginalPathsForEmptySourceTree(t *testing.T) {
	outputPath := writeArchive(t, []archiveEntry{
		{path: "duplicate.txt", body: []byte("first")},
		{path: "duplicate.txt", body: []byte("second")},
	})
	original, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := t.TempDir()
	cmd := NewPatchCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", inputPath, "--output", outputPath})

	err = cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want duplicate archive path error")
	}
	if !strings.Contains(err.Error(), `duplicate archive path "duplicate.txt"`) {
		t.Errorf("error = %q, want duplicate archive path context", err)
	}
	assertFileBytes(t, outputPath, original)
}

func TestPatchLeavesValidatedArchiveByteForByteUnchangedForEmptySourceTree(t *testing.T) {
	outputPath := writeArchive(t, []archiveEntry{{path: "entry.txt", body: []byte("body")}})
	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("unparsed trailing bytes")); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewPatchCommand()
	cmd.SetArgs([]string{"--input", t.TempDir(), "--output", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertFileBytes(t, outputPath, original)
}

func TestPatchCleanlyRebuildsWithoutReplacedOrTrailingBytes(t *testing.T) {
	oldBody := []byte("obsolete-body-marker")
	trailing := []byte("trailing-data-marker")
	outputPath := writeArchive(t, []archiveEntry{{path: "entry.txt", body: oldBody}})
	file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(trailing); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	inputPath := t.TempDir()
	writeSourceFile(t, inputPath, "entry.txt", []byte("new body"))
	cmd := NewPatchCommand()
	cmd.SetArgs([]string{"--input", inputPath, "--output", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, oldBody) {
		t.Errorf("rebuilt archive retains replaced body %q", oldBody)
	}
	if bytes.Contains(data, trailing) {
		t.Errorf("rebuilt archive retains trailing bytes %q", trailing)
	}
	assertArchiveEntries(t, readArchive(t, outputPath), []archiveEntry{{path: "entry.txt", body: []byte("new body")}})
}

func TestPatchPreservesInitialArchivePermissions(t *testing.T) {
	outputPath := writeArchive(t, []archiveEntry{{path: "entry.txt", body: []byte("old")}})
	if err := os.Chmod(outputPath, 0o751); err != nil {
		t.Fatal(err)
	}
	inputPath := t.TempDir()
	writeSourceFile(t, inputPath, "entry.txt", []byte("new"))
	cmd := NewPatchCommand()
	cmd.SetArgs([]string{"-i", inputPath, "-o", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o751 {
		t.Errorf("output permissions = %o, want 751", info.Mode().Perm())
	}
}

func TestPatchReportsCopiedReplacedAndAddedEntriesInArchiveOrder(t *testing.T) {
	outputPath := writeArchive(t, []archiveEntry{
		{path: "keep.txt", body: []byte("kept")},
		{empty: true},
		{path: "same.txt", body: []byte("same body")},
	})
	inputPath := t.TempDir()
	writeSourceFile(t, inputPath, "same.txt", []byte("same body"))
	writeSourceFile(t, inputPath, "new.txt", []byte("new body"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewPatchCommand()
	cmd.SetArgs([]string{"--input", inputPath, "--output", outputPath, "--verbose"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	want := "[1/3] Copied keep.txt\n[2/3] Replaced same.txt\n[3/3] Added new.txt\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestPatchPreservesInitialArchiveWhenProgressFailsBeforeRename(t *testing.T) {
	outputPath := writeArchive(t, []archiveEntry{{path: "entry.txt", body: []byte("old body")}})
	original, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := t.TempDir()
	writeSourceFile(t, inputPath, "entry.txt", []byte("new body"))
	cmd := NewPatchCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", inputPath, "--output", outputPath, "--verbose"})
	cmd.SetErr(failingProgressWriter{})

	err = cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want progress write error")
	}
	if !strings.Contains(err.Error(), "failed to write patching progress") {
		t.Errorf("error = %q, want progress context", err)
	}
	assertFileBytes(t, outputPath, original)
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(outputPath), ".archive-without-dat-extension.*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("temporary archives = %v, want none", matches)
	}
}

func TestPatchRequiresAnExistingReadableRegularDATArchive(t *testing.T) {
	tests := []struct {
		name       string
		outputPath func(t *testing.T) string
		want       string
	}{
		{
			name: "missing",
			outputPath: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing", "archive.dat")
			},
			want: "failed to open existing DAT archive",
		},
		{
			name: "directory",
			outputPath: func(t *testing.T) string {
				return t.TempDir()
			},
			want: "is not a regular file",
		},
		{
			name: "malformed",
			outputPath: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "malformed.dat")
				if err := os.WriteFile(path, []byte{1, 0}, 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			},
			want: "failed to parse existing DAT archive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath := tt.outputPath(t)
			cmd := NewPatchCommand()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--input", t.TempDir(), "--output", outputPath})

			err := cmd.Execute()

			if err == nil {
				t.Fatal("Execute() error = nil, want archive validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want %q context", err, tt.want)
			}
			if tt.name == "missing" {
				if _, statErr := os.Stat(filepath.Dir(outputPath)); statErr != nil {
					t.Errorf("output parent stat error = %v, want created directory", statErr)
				}
			}
		})
	}
}

func TestPatchReplacesExactMatchesPreservesOriginalSlotsAppendsLexicallyAndIsQuiet(t *testing.T) {
	outputPath := writeArchive(t, []archiveEntry{
		{path: "keep.txt", body: []byte("keep body")},
		{empty: true},
		{path: "Case.txt", body: []byte("uppercase body")},
		{path: "replace.txt", body: []byte("old body")},
	})
	inputPath := t.TempDir()
	writeSourceFile(t, inputPath, "replace.txt", []byte("replacement body"))
	writeSourceFile(t, inputPath, "case.txt", []byte("lowercase body"))
	writeSourceFile(t, inputPath, "z-new.txt", []byte("last new body"))
	writeSourceFile(t, inputPath, "a-new.txt", []byte("first new body"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewPatchCommand()
	cmd.SetArgs([]string{"--input", inputPath, "--output", outputPath})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := []archiveEntry{
		{path: "keep.txt", body: []byte("keep body")},
		{empty: true},
		{path: "Case.txt", body: []byte("uppercase body")},
		{path: "replace.txt", body: []byte("replacement body")},
		{path: "a-new.txt", body: []byte("first new body")},
		{path: "case.txt", body: []byte("lowercase body")},
		{path: "z-new.txt", body: []byte("last new body")},
	}
	assertArchiveEntries(t, readArchive(t, outputPath), want)
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}
