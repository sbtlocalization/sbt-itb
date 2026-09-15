// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	"github.com/sbtlocalization/sbt-itb/parser"
)

func TestPackRecursesInDeterministicOrderWithExactBytesAndIsQuiet(t *testing.T) {
	inputPath := t.TempDir()
	writeSourceFile(t, inputPath, "z-last.txt", []byte("last"))
	writeSourceFile(t, inputPath, "nested/binary.dat", []byte{0, 1, 255, 2})
	writeSourceFile(t, inputPath, ".hidden", []byte("hidden"))
	outputPath := filepath.Join(t.TempDir(), "missing", "archive-without-extension")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewPackCommand()
	cmd.SetArgs([]string{"--input", inputPath, "--output", outputPath})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := readArchive(t, outputPath)
	want := []archiveEntry{
		{path: ".hidden", body: []byte("hidden")},
		{path: "nested/binary.dat", body: []byte{0, 1, 255, 2}},
		{path: "z-last.txt", body: []byte("last")},
	}
	assertArchiveEntries(t, got, want)
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

type failingProgressWriter struct{}

func (failingProgressWriter) Write([]byte) (int, error) {
	return 0, errors.New("progress write failed")
}

func TestPackPreservesExistingOutputWhenProgressWriteFailsBeforeRename(t *testing.T) {
	inputPath := t.TempDir()
	writeSourceFile(t, inputPath, "entry.txt", []byte("new body"))
	outputPath := filepath.Join(t.TempDir(), "archive.dat")
	oldBytes := []byte("old archive bytes")
	if err := os.WriteFile(outputPath, oldBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewPackCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", inputPath, "--output", outputPath, "--verbose"})
	cmd.SetErr(failingProgressWriter{})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want progress write error")
	}
	if !strings.Contains(err.Error(), "failed to write packing progress") {
		t.Errorf("error = %q, want progress context", err)
	}
	assertFileBytes(t, outputPath, oldBytes)
	matches, globErr := filepath.Glob(filepath.Join(filepath.Dir(outputPath), ".archive.dat.*.tmp"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Errorf("temporary archives = %v, want none", matches)
	}
}

func TestPackRejectsInvalidUTF8ArchivePaths(t *testing.T) {
	inputPath := t.TempDir()
	invalidName := string([]byte{'b', 'a', 'd', 0xff})
	invalidPath := filepath.Join(inputPath, invalidName)
	if err := os.WriteFile(invalidPath, []byte("body"), 0o600); err != nil {
		t.Skipf("invalid UTF-8 filenames unavailable: %v", err)
	}
	cmd := NewPackCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", inputPath, "--output", filepath.Join(t.TempDir(), "archive.dat")})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want UTF-8 validation error")
	}
	if !strings.Contains(err.Error(), "is not valid UTF-8") {
		t.Errorf("error = %q, want UTF-8 context", err)
	}
}

func TestPackRejectsOtherNonRegularSourceObjects(t *testing.T) {
	inputPath := t.TempDir()
	socketPath := filepath.Join(inputPath, "service.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("Unix sockets unavailable: %v", err)
	}
	defer listener.Close()
	cmd := NewPackCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", inputPath, "--output", filepath.Join(t.TempDir(), "archive.dat")})

	err = cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want non-regular source error")
	}
	if !strings.Contains(err.Error(), "service.sock is not a regular file") {
		t.Errorf("error = %q, want source path context", err)
	}
}

func TestPackRejectsSymbolicLinks(t *testing.T) {
	inputPath := t.TempDir()
	target := writeSourceFile(t, inputPath, "target.txt", []byte("target"))
	link := filepath.Join(inputPath, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	cmd := NewPackCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", inputPath, "--output", filepath.Join(t.TempDir(), "archive.dat")})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want symbolic link error")
	}
	if !strings.Contains(err.Error(), "link.txt is a symbolic link") {
		t.Errorf("error = %q, want source path context", err)
	}
}

func TestPackRejectsOutputWithinSourceTreeAndPreservesIt(t *testing.T) {
	inputPath := t.TempDir()
	outputPath := filepath.Join(inputPath, "nested", "archive.dat")
	oldBytes := []byte("existing output")
	writeSourceFile(t, inputPath, "nested/archive.dat", oldBytes)
	cmd := NewPackCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", inputPath, "--output", outputPath})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want containment error")
	}
	if !strings.Contains(err.Error(), "is located within archive source tree") {
		t.Errorf("error = %q, want output containment context", err)
	}
	assertFileBytes(t, outputPath, oldBytes)
	matches, globErr := filepath.Glob(filepath.Join(filepath.Dir(outputPath), ".archive.dat.*.tmp"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Errorf("temporary archives = %v, want none", matches)
	}
}

func TestPackReplacesExistingArchive(t *testing.T) {
	inputPath := t.TempDir()
	writeSourceFile(t, inputPath, "new.txt", []byte("new body"))
	outputPath := filepath.Join(t.TempDir(), "archive.dat")
	if err := os.WriteFile(outputPath, []byte("old archive bytes that must disappear"), 0o600); err != nil {
		t.Fatal(err)
	}
	referencePath := filepath.Join(filepath.Dir(outputPath), "normal-permissions-reference")
	if err := os.WriteFile(referencePath, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	referenceInfo, err := os.Stat(referencePath)
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewPackCommand()
	cmd.SetArgs([]string{"--input", inputPath, "--output", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertArchiveEntries(t, readArchive(t, outputPath), []archiveEntry{{path: "new.txt", body: []byte("new body")}})
	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if outputInfo.Mode().Perm() != referenceInfo.Mode().Perm() {
		t.Errorf("output permissions = %o, want normal permissions %o", outputInfo.Mode().Perm(), referenceInfo.Mode().Perm())
	}
}

func TestPackReportsVerboseProgressInArchiveOrder(t *testing.T) {
	inputPath := t.TempDir()
	writeSourceFile(t, inputPath, "second.txt", []byte("two"))
	writeSourceFile(t, inputPath, "first.txt", []byte("one"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewPackCommand()
	cmd.SetArgs([]string{"-i", inputPath, "-o", filepath.Join(t.TempDir(), "archive.dat"), "-v"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	want := "[1/2] Packed first.txt\n[2/2] Packed second.txt\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestPackCreatesEmptyArchive(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "empty.dat")
	cmd := NewPackCommand()
	cmd.SetArgs([]string{"--input", t.TempDir(), "--output", outputPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := readArchive(t, outputPath); len(got) != 0 {
		t.Errorf("archive entries = %d, want 0", len(got))
	}
}

func writeSourceFile(t *testing.T, root, archivePath string, body []byte) string {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(archivePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readArchive(t *testing.T, path string) []archiveEntry {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	archive := parser.NewFtlDat()
	if err := archive.Read(kaitai.NewStream(file), nil, archive); err != nil {
		t.Fatalf("parse packed archive: %v", err)
	}
	entries := make([]archiveEntry, 0, len(archive.Files))
	for i, file := range archive.Files {
		if file.OfsMeta == 0 {
			entries = append(entries, archiveEntry{empty: true})
			continue
		}
		meta, err := file.Meta()
		if err != nil {
			t.Fatalf("read archive entry %d: %v", i+1, err)
		}
		entries = append(entries, archiveEntry{path: meta.Filename, body: append([]byte(nil), meta.Body...)})
	}
	return entries
}

func assertArchiveEntries(t *testing.T, got, want []archiveEntry) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("archive entries = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].path != want[i].path {
			t.Errorf("entry %d path = %q, want %q", i+1, got[i].path, want[i].path)
		}
		if !bytes.Equal(got[i].body, want[i].body) {
			t.Errorf("entry %d body = %v, want %v", i+1, got[i].body, want[i].body)
		}
	}
}
