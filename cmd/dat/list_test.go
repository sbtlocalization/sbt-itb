// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type archiveEntry struct {
	path  string
	body  []byte
	empty bool
}

type failAfterFirstWrite struct {
	bytes.Buffer
	writes int
}

func (w *failAfterFirstWrite) Write(p []byte) (int, error) {
	if w.writes > 0 {
		return 0, errors.New("write failed")
	}
	w.writes++
	return w.Buffer.Write(p)
}

func TestListPrintsArchiveEntriesAsTable(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{path: "img/ui/flask.png", body: []byte{1, 2, 3}},
		{path: "scripts/init.lua", body: []byte("ten bytes!")},
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewListCommand()
	cmd.SetArgs([]string{"--input", archivePath})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "Size  Path\n3     img/ui/flask.png\n10    scripts/init.lua\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestListStreamsArchiveEntriesAsJSONL(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{path: "img/ui/flask.png", body: []byte{1, 2, 3}},
		{path: "scripts/init.lua", body: []byte("ten bytes!")},
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewListCommand()
	cmd.SetArgs([]string{"--input", archivePath, "--json"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "{\"path\":\"img/ui/flask.png\",\"size\":3}\n{\"path\":\"scripts/init.lua\",\"size\":10}\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestListStreamsJSONLWithShortFlag(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{{path: "scripts/init.lua", body: []byte{1, 2}}})
	var stdout bytes.Buffer
	cmd := NewListCommand()
	cmd.SetArgs([]string{"--input", archivePath, "-j"})
	cmd.SetOut(&stdout)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout.String() != "{\"path\":\"scripts/init.lua\",\"size\":2}\n" {
		t.Errorf("stdout = %q, want one JSONL record", stdout.String())
	}
}

func TestListPrintsHeadingsForEmptyArchive(t *testing.T) {
	archivePath := writeArchive(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewListCommand()
	cmd.SetArgs([]string{"-i", archivePath})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout.String() != "Size  Path\n" {
		t.Errorf("stdout = %q, want headings only", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestListEmitsNoJSONLRecordsForEmptyArchive(t *testing.T) {
	archivePath := writeArchive(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewListCommand()
	cmd.SetArgs([]string{"--input", archivePath, "--json"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestListWarnsAndOmitsEmptyArchiveSlot(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{path: "first.txt", body: []byte{1}},
		{empty: true},
		{path: "second.txt", body: []byte{2, 3}},
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewListCommand()
	cmd.SetArgs([]string{"--input", archivePath})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	wantOutput := "Size  Path\n1     first.txt\n2     second.txt\n"
	if stdout.String() != wantOutput {
		t.Errorf("stdout = %q, want %q", stdout.String(), wantOutput)
	}
	if stderr.String() != "Warning: empty archive slot 2\n" {
		t.Errorf("stderr = %q, want empty slot warning", stderr.String())
	}
}

func TestListWarnsAndEmitsNoJSONLRecordForEmptyArchiveSlot(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{path: "first.txt", body: []byte{1}},
		{empty: true},
		{path: "second.txt", body: []byte{2, 3}},
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewListCommand()
	cmd.SetArgs([]string{"--input", archivePath, "--json"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	wantOutput := "{\"path\":\"first.txt\",\"size\":1}\n{\"path\":\"second.txt\",\"size\":2}\n"
	if stdout.String() != wantOutput {
		t.Errorf("stdout = %q, want %q", stdout.String(), wantOutput)
	}
	if stderr.String() != "Warning: empty archive slot 2\n" {
		t.Errorf("stderr = %q, want empty slot warning", stderr.String())
	}
}

func TestListReturnsContextForArchiveEntryParserError(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{{path: "broken.txt", body: []byte{1, 2}}})
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, data[:len(data)-1], 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewListCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", archivePath})

	err = cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want parser error")
	}
	want := "failed to read archive entry 1 metadata from DAT archive " + archivePath
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want context %q", err, want)
	}
}

func TestListKeepsEarlierJSONLRecordsWhenLaterEntryFailsToParse(t *testing.T) {
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
	var stdout bytes.Buffer
	cmd := NewListCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", archivePath, "--json"})
	cmd.SetOut(&stdout)

	err = cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want parser error")
	}
	wantError := "failed to read archive entry 2 metadata from DAT archive " + archivePath
	if !strings.Contains(err.Error(), wantError) {
		t.Errorf("error = %q, want context %q", err, wantError)
	}
	if stdout.String() != "{\"path\":\"first.txt\",\"size\":1}\n" {
		t.Errorf("stdout = %q, want first JSONL record", stdout.String())
	}
}

func TestListKeepsEarlierJSONLRecordsWhenLaterEntryFailsToEncode(t *testing.T) {
	archivePath := writeArchive(t, []archiveEntry{
		{path: "first.txt", body: []byte{1}},
		{path: "second.txt", body: []byte{2, 3}},
	})
	stdout := &failAfterFirstWrite{}
	cmd := NewListCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", archivePath, "--json"})
	cmd.SetOut(stdout)

	err := cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want encoding error")
	}
	if !strings.Contains(err.Error(), "failed to encode archive entry 2") {
		t.Errorf("error = %q, want archive entry context", err)
	}
	if stdout.String() != "{\"path\":\"first.txt\",\"size\":1}\n" {
		t.Errorf("stdout = %q, want first JSONL record", stdout.String())
	}
}

func TestListRequiresInputFlagAndRejectsPositionalArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing input", args: nil},
		{name: "positional input", args: []string{"archive.dat"}},
		{name: "extra argument", args: []string{"--input", "one.dat", "two.dat"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewListCommand()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err == nil {
				t.Fatal("Execute() error = nil, want argument error")
			}
		})
	}
}

func TestListUsesLsAlias(t *testing.T) {
	cmd := NewListCommand()

	if len(cmd.Aliases) != 1 || cmd.Aliases[0] != "ls" {
		t.Errorf("Aliases = %v, want [ls]", cmd.Aliases)
	}
}

func TestListInputFlagCompletesDatFilenames(t *testing.T) {
	cmd := NewListCommand()
	flag := cmd.Flags().Lookup("input")

	if flag == nil {
		t.Fatal("input flag is missing")
	}
	if flag.Shorthand != "i" {
		t.Errorf("input shorthand = %q, want %q", flag.Shorthand, "i")
	}
	extensions := flag.Annotations[cobra.BashCompFilenameExt]
	if len(extensions) != 1 || extensions[0] != "dat" {
		t.Errorf("input filename extensions = %v, want [dat]", extensions)
	}
}

func TestListReturnsContextForInputError(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "missing")
	cmd := NewListCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", archivePath})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want input error")
	}
	want := "failed to open DAT archive " + archivePath
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want context %q", err, want)
	}
}

func TestListReturnsContextForArchiveParserError(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "truncated")
	if err := os.WriteFile(archivePath, []byte{1, 0}, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewListCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--input", archivePath})

	err := cmd.Execute()

	if err == nil {
		t.Fatal("Execute() error = nil, want parser error")
	}
	want := "failed to parse DAT archive " + archivePath
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want context %q", err, want)
	}
}

func writeArchive(t *testing.T, entries []archiveEntry) string {
	t.Helper()

	var archive bytes.Buffer
	if err := binary.Write(&archive, binary.LittleEndian, uint32(len(entries))); err != nil {
		t.Fatal(err)
	}

	offset := uint32(4 + 4*len(entries))
	for _, entry := range entries {
		entryOffset := offset
		if entry.empty {
			entryOffset = 0
		} else {
			offset += uint32(8 + len(entry.path) + len(entry.body))
		}
		if err := binary.Write(&archive, binary.LittleEndian, entryOffset); err != nil {
			t.Fatal(err)
		}
	}

	for _, entry := range entries {
		if entry.empty {
			continue
		}
		if err := binary.Write(&archive, binary.LittleEndian, uint32(len(entry.body))); err != nil {
			t.Fatal(err)
		}
		if err := binary.Write(&archive, binary.LittleEndian, uint32(len(entry.path))); err != nil {
			t.Fatal(err)
		}
		archive.WriteString(entry.path)
		archive.Write(entry.body)
	}

	path := filepath.Join(t.TempDir(), "archive-without-dat-extension")
	if err := os.WriteFile(path, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
