// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package csv

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMergeCommandContract(t *testing.T) {
	group := NewCommand()
	if group.Use != "csv" {
		t.Errorf("group Use = %q, want csv", group.Use)
	}

	var merge *cobra.Command
	for _, child := range group.Commands() {
		if child.Name() == "merge" {
			merge = child
			break
		}
	}
	if merge == nil {
		t.Fatal("csv merge command is missing")
	}

	tests := []struct {
		name       string
		shorthand  string
		defaultVal string
		directory  bool
	}{
		{name: "orig", shorthand: "g", directory: true},
		{name: "input", shorthand: "i", directory: true},
		{name: "output", shorthand: "o", directory: true},
		{name: "lang", shorthand: "l", defaultVal: "uk"},
		{name: "column", shorthand: "c", defaultVal: "Ukrainian"},
		{name: "target-column", shorthand: "t", defaultVal: "English"},
		{name: "verbose", shorthand: "v", defaultVal: "false"},
	}
	for _, tt := range tests {
		flag := merge.Flags().Lookup(tt.name)
		if flag == nil {
			t.Errorf("%s flag is missing", tt.name)
			continue
		}
		if flag.Shorthand != tt.shorthand {
			t.Errorf("%s shorthand = %q, want %q", tt.name, flag.Shorthand, tt.shorthand)
		}
		if flag.DefValue != tt.defaultVal {
			t.Errorf("%s default = %q, want %q", tt.name, flag.DefValue, tt.defaultVal)
		}
		if tt.directory {
			if _, ok := flag.Annotations[cobra.BashCompSubdirsInDir]; !ok {
				t.Errorf("%s annotations = %v, want directory completion", tt.name, flag.Annotations)
			}
		}
	}
}

func TestMergeMergesByNormalizedIDInSourceOrderWithFallbackWarnings(t *testing.T) {
	orig, input, output := mergeDirectories(t)
	source := append([]byte{0xef, 0xbb, 0xbf}, []byte("ID,English,Context\n id-2 ,Second,B\nid-1,First,A\nid-3,Third,C\n")...)
	writeTestFile(t, filepath.Join(orig, "Foo.csv"), source)
	translated := append([]byte{0xef, 0xbb, 0xbf}, []byte("ID,Ukrainian\r\nid-1,Перший\r\n id-2 ,Другий\r\nid-3,   \r\nextra,Зайвий\r\n")...)
	writeTestFile(t, filepath.Join(input, "uk_Foo.csv"), translated)

	stderr, err := executeMerge(t, orig, input, output)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := append([]byte{0xef, 0xbb, 0xbf}, []byte("ID,English,Context\r\n id-2 ,Другий,B\r\nid-1,Перший,A\r\nid-3,Third,C\r\n")...)
	assertTestFile(t, filepath.Join(output, "Foo.csv"), want)
	wantWarnings := "Warning: Foo.csv: empty translation for ID \"id-3\"; retained source English\n" +
		"Warning: Foo.csv: translation ID \"extra\" has no source entry; ignored\n"
	if stderr != wantWarnings {
		t.Errorf("stderr = %q, want %q", stderr, wantWarnings)
	}
}

func TestMergeReplacesConfiguredTargetColumn(t *testing.T) {
	orig, input, output := mergeDirectories(t)
	writeTestFile(t, filepath.Join(orig, "Foo.csv"), []byte("ID,Dialogue,English\n1,Old dialogue,Keep English\n2,Fallback dialogue,Other English\n"))
	writeTestFile(t, filepath.Join(input, "uk_Foo.csv"), []byte("ID,Ukrainian\n1,Новий діалог\n"))

	stderr, err := executeMerge(t, orig, input, output, "--target-column", "Dialogue")

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	wantWarning := "Warning: Foo.csv: no translation for ID \"2\"; retained source \"Dialogue\"\n"
	if stderr != wantWarning {
		t.Errorf("stderr = %q, want %q", stderr, wantWarning)
	}
	want := append([]byte{0xef, 0xbb, 0xbf}, []byte("ID,Dialogue,English\r\n1,Новий діалог,Keep English\r\n2,Fallback dialogue,Other English\r\n")...)
	assertTestFile(t, filepath.Join(output, "Foo.csv"), want)
}

func TestMergePairsExactFilenamesCopiesMissingTranslationAndReportsLexicalProgress(t *testing.T) {
	orig, input, output := mergeDirectories(t)
	copyBytes := []byte{0xff, 'n', 'o', 't', '-', 'c', 's', 'v'}
	writeTestFile(t, filepath.Join(orig, "A.csv"), copyBytes)
	writeTestFile(t, filepath.Join(orig, "b.csv"), []byte("ID,English\n1,Old\n"))
	writeTestFile(t, filepath.Join(orig, "ignored.CSV"), []byte("ignored"))
	writeTestFile(t, filepath.Join(orig, "nested", "ignored.csv"), []byte("ignored"))
	writeTestFile(t, filepath.Join(input, "pl_b.csv"), []byte("ID,Polish\n1,Nowy\n"))
	writeTestFile(t, filepath.Join(input, "pl_unrelated.csv"), []byte("not,csv"))

	stderr, err := executeMerge(t, orig, input, output, "--lang", "pl", "--column", "Polish", "--verbose")

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertTestFile(t, filepath.Join(output, "A.csv"), copyBytes)
	wantMerged := append([]byte{0xef, 0xbb, 0xbf}, []byte("ID,English\r\n1,Nowy\r\n")...)
	assertTestFile(t, filepath.Join(output, "b.csv"), wantMerged)
	wantStderr := "[1/2] A.csv\n" +
		"Warning: A.csv: translation file pl_A.csv is missing; copied source unchanged\n" +
		"[2/2] b.csv\n"
	if stderr != wantStderr {
		t.Errorf("stderr = %q, want %q", stderr, wantStderr)
	}
	if _, err := os.Stat(filepath.Join(output, "ignored.CSV")); !os.IsNotExist(err) {
		t.Errorf("ignored.CSV stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(output, "nested", "ignored.csv")); !os.IsNotExist(err) {
		t.Errorf("nested ignored.csv stat error = %v, want not exist", err)
	}
}

func TestMergeValidatesOptionsAndDirectories(t *testing.T) {
	for _, flag := range []string{"column", "target-column"} {
		t.Run("empty "+flag, func(t *testing.T) {
			orig, input, output := mergeDirectories(t)
			writeTestFile(t, filepath.Join(orig, "one.csv"), []byte("ID,English\n"))
			_, err := executeMerge(t, orig, input, output, "--"+flag+"=")
			if err == nil || !strings.Contains(err.Error(), flag+" name must not be empty") {
				t.Fatalf("Execute() error = %v, want empty %s error", err, flag)
			}
		})
	}

	for _, language := range []string{"", "uk/evil", "uk space", "."} {
		t.Run("language "+language, func(t *testing.T) {
			orig, input, output := mergeDirectories(t)
			writeTestFile(t, filepath.Join(orig, "one.csv"), []byte("ID,English\n"))
			_, err := executeMerge(t, orig, input, output, "--lang="+language)
			if err == nil || !strings.Contains(err.Error(), "language code") {
				t.Fatalf("Execute() error = %v, want language code error", err)
			}
		})
	}

	t.Run("same resolved location", func(t *testing.T) {
		orig, _, output := mergeDirectories(t)
		writeTestFile(t, filepath.Join(orig, "one.csv"), []byte("ID,English\n"))
		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(orig, alias); err != nil {
			t.Fatal(err)
		}
		_, err := executeMerge(t, orig, alias, output)
		if err == nil || !strings.Contains(err.Error(), "must be distinct") {
			t.Fatalf("Execute() error = %v, want distinct directory error", err)
		}
	})

	t.Run("source and translation must exist as directories", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "file")
		writeTestFile(t, file, []byte("data"))
		for _, paths := range [][2]string{{filepath.Join(root, "missing"), root}, {root, file}} {
			_, err := executeMerge(t, paths[0], paths[1], filepath.Join(root, "output"))
			if err == nil {
				t.Fatal("Execute() error = nil, want directory error")
			}
		}
	})

	t.Run("output must be a directory", func(t *testing.T) {
		orig, input, output := mergeDirectories(t)
		writeTestFile(t, filepath.Join(orig, "one.csv"), []byte("ID,English\n"))
		writeTestFile(t, output, []byte("blocked"))
		_, err := executeMerge(t, orig, input, output)
		if err == nil || !strings.Contains(err.Error(), "failed to create output directory") {
			t.Fatalf("Execute() error = %v, want output directory error", err)
		}
	})
}

func TestMergeCreatesOutputButRejectsSourceWithoutEligibleCSV(t *testing.T) {
	orig, input, output := mergeDirectories(t)
	writeTestFile(t, filepath.Join(orig, "upper.CSV"), []byte("ignored"))
	writeTestFile(t, filepath.Join(orig, "nested", "one.csv"), []byte("ignored"))

	_, err := executeMerge(t, orig, input, output)

	if err == nil || !strings.Contains(err.Error(), "contains no eligible CSV files") {
		t.Fatalf("Execute() error = %v, want no eligible CSV error", err)
	}
	info, statErr := os.Stat(output)
	if statErr != nil || !info.IsDir() {
		t.Errorf("output directory stat = (%v, %v), want created directory", info, statErr)
	}
}

func TestMergeRejectsInvalidCSVAndRequiredHeadersPerFile(t *testing.T) {
	tests := []struct {
		name        string
		source      []byte
		translation []byte
		want        string
	}{
		{name: "source invalid UTF-8", source: []byte("ID,English\n1,\xff\n"), translation: []byte("ID,Ukrainian\n1,Text\n"), want: "source CSV is not valid UTF-8"},
		{name: "translation invalid UTF-8", source: []byte("ID,English\n1,Text\n"), translation: []byte("ID,Ukrainian\n1,\xff\n"), want: "translation CSV is not valid UTF-8"},
		{name: "malformed source quote", source: []byte("ID,English\n1,\"broken\n"), translation: []byte("ID,Ukrainian\n1,Text\n"), want: "failed to parse source CSV"},
		{name: "malformed translation fields", source: []byte("ID,English\n1,Text\n"), translation: []byte("ID,Ukrainian\n1\n"), want: "failed to parse translation CSV"},
		{name: "wrong source first header", source: []byte("Id,English\n1,Text\n"), translation: []byte("ID,Ukrainian\n1,Text\n"), want: "first source header must be exactly ID"},
		{name: "wrong translation first header", source: []byte("ID,English\n1,Text\n"), translation: []byte(" ID,Ukrainian\n1,Text\n"), want: "first translation header must be exactly ID"},
		{name: "missing English", source: []byte("ID,english\n1,Text\n"), translation: []byte("ID,Ukrainian\n1,Text\n"), want: "missing required \"English\" header"},
		{name: "duplicate English", source: []byte("ID,English,English\n1,A,B\n"), translation: []byte("ID,Ukrainian\n1,Text\n"), want: "duplicate \"English\" headers"},
		{name: "missing exact target", source: []byte("ID,English\n1,Text\n"), translation: []byte("ID,ukrainian\n1,Text\n"), want: "missing required \"Ukrainian\" header"},
		{name: "duplicate target", source: []byte("ID,English\n1,Text\n"), translation: []byte("ID,Ukrainian,Ukrainian\n1,A,B\n"), want: "duplicate \"Ukrainian\" headers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig, input, output := mergeDirectories(t)
			writeTestFile(t, filepath.Join(orig, "Foo.csv"), tt.source)
			writeTestFile(t, filepath.Join(input, "uk_Foo.csv"), tt.translation)

			stderr, err := executeMerge(t, orig, input, output)

			if err == nil || err.Error() != "1 source file failed" {
				t.Fatalf("Execute() error = %v, want concise failed-file count", err)
			}
			if !strings.HasPrefix(stderr, "Error: Foo.csv: ") || !strings.Contains(stderr, tt.want) {
				t.Errorf("stderr = %q, want Foo.csv error containing %q", stderr, tt.want)
			}
			if _, statErr := os.Stat(filepath.Join(output, "Foo.csv")); !os.IsNotExist(statErr) {
				t.Errorf("destination stat error = %v, want not exist", statErr)
			}
		})
	}
}

func TestMergeRejectsDuplicateAndInvalidNormalizedIDs(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		translation string
		want        string
	}{
		{name: "duplicate source", source: "ID,English\none,A\n one ,B\n", translation: "ID,Ukrainian\none,T\n", want: "source CSV contains duplicate ID \"one\""},
		{name: "duplicate translation", source: "ID,English\none,A\n", translation: "ID,Ukrainian\none,T\n one ,U\n", want: "translation CSV contains duplicate ID \"one\""},
		{name: "empty source ID", source: "ID,English\n,Text\n", translation: "ID,Ukrainian\none,T\n", want: "source CSV contains a record with an empty ID"},
		{name: "empty translation ID", source: "ID,English\none,A\n", translation: "ID,Ukrainian\n,Text\n", want: "translation CSV contains a record with an empty ID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig, input, output := mergeDirectories(t)
			writeTestFile(t, filepath.Join(orig, "Foo.csv"), []byte(tt.source))
			writeTestFile(t, filepath.Join(input, "uk_Foo.csv"), []byte(tt.translation))

			stderr, err := executeMerge(t, orig, input, output)

			if err == nil {
				t.Fatal("Execute() error = nil, want ID validation error")
			}
			if !strings.Contains(stderr, tt.want) {
				t.Errorf("stderr = %q, want %q", stderr, tt.want)
			}
		})
	}
}

func TestMergeOmitsEmptyRecordsAndWarnsOnceForMissingTranslation(t *testing.T) {
	orig, input, output := mergeDirectories(t)
	writeTestFile(t, filepath.Join(orig, "Foo.csv"), []byte("ID,English,Context\r\n,,\r\n1,One,A\r\n   ,   , \r\n2,Two,B\r\n"))
	writeTestFile(t, filepath.Join(input, "uk_Foo.csv"), []byte("ID,Ukrainian\n,\n1,  Один  \n"))

	stderr, err := executeMerge(t, orig, input, output)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := append([]byte{0xef, 0xbb, 0xbf}, []byte("ID,English,Context\r\n1,  Один  ,A\r\n2,Two,B\r\n")...)
	assertTestFile(t, filepath.Join(output, "Foo.csv"), want)
	wantWarning := "Warning: Foo.csv: no translation for ID \"2\"; retained source English\n"
	if stderr != wantWarning {
		t.Errorf("stderr = %q, want %q", stderr, wantWarning)
	}
}

func TestMergePreservesQuotedFieldsAndCaseSensitiveIDs(t *testing.T) {
	orig, input, output := mergeDirectories(t)
	source := "ID,English,Context\r\n\" id,1 \",\"Old \"\"quoted\"\"\",\"line1\nline2\"\r\nA,Upper,one\r\na,Lower,two\r\n"
	translation := "ID,Ukrainian\n\"id,1\",\"New, \"\"quoted\"\"\"\nA,Велика\na,Мала\n"
	writeTestFile(t, filepath.Join(orig, "Foo.csv"), []byte(source))
	writeTestFile(t, filepath.Join(input, "uk_Foo.csv"), []byte(translation))

	stderr, err := executeMerge(t, orig, input, output)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	wantCSV := "ID,English,Context\r\n\" id,1 \",\"New, \"\"quoted\"\"\",\"line1\nline2\"\r\nA,Велика,one\r\na,Мала,two\r\n"
	want := append([]byte{0xef, 0xbb, 0xbf}, []byte(wantCSV)...)
	assertTestFile(t, filepath.Join(output, "Foo.csv"), want)
}

func TestMergeContinuesAfterFileErrorAndCommitsOnlySuccessfulFiles(t *testing.T) {
	orig, input, output := mergeDirectories(t)
	writeTestFile(t, filepath.Join(orig, "Broken.csv"), []byte("ID,English\n1,\"broken\n"))
	writeTestFile(t, filepath.Join(input, "uk_Broken.csv"), []byte("ID,Ukrainian\n1,Text\n"))
	writeTestFile(t, filepath.Join(orig, "Good.csv"), []byte("ID,English\n1,Old\n"))
	writeTestFile(t, filepath.Join(input, "uk_Good.csv"), []byte("ID,Ukrainian\n1,New\n"))
	writeTestFile(t, filepath.Join(output, "Broken.csv"), []byte("existing broken destination"))
	writeTestFile(t, filepath.Join(output, "Good.csv"), []byte("existing good destination with longer bytes"))
	writeTestFile(t, filepath.Join(output, "unrelated.txt"), []byte("keep me"))

	stderr, err := executeMerge(t, orig, input, output, "--verbose")

	if err == nil || err.Error() != "1 source file failed" {
		t.Fatalf("Execute() error = %v, want concise failed-file count", err)
	}
	wantPrefix := "[1/2] Broken.csv\nError: Broken.csv: failed to parse source CSV:"
	if !strings.HasPrefix(stderr, wantPrefix) || !strings.Contains(stderr, "\n[2/2] Good.csv\n") {
		t.Errorf("stderr = %q, want grouped deterministic diagnostics", stderr)
	}
	assertTestFile(t, filepath.Join(output, "Broken.csv"), []byte("existing broken destination"))
	wantGood := append([]byte{0xef, 0xbb, 0xbf}, []byte("ID,English\r\n1,New\r\n")...)
	assertTestFile(t, filepath.Join(output, "Good.csv"), wantGood)
	assertTestFile(t, filepath.Join(output, "unrelated.txt"), []byte("keep me"))
	entries, readErr := os.ReadDir(output)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			t.Errorf("temporary output %q was not removed", entry.Name())
		}
	}
}

func TestMergeIsQuietForSuccessfulFilesWithoutVerboseMode(t *testing.T) {
	orig, input, output := mergeDirectories(t)
	writeTestFile(t, filepath.Join(orig, "Foo.csv"), []byte("ID,English\n1,Old\n"))
	writeTestFile(t, filepath.Join(input, "uk_Foo.csv"), []byte("ID,Ukrainian\n1,New\n"))

	stderr, err := executeMerge(t, orig, input, output)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestMergeRequiresDirectoriesAndRejectsArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing all"},
		{name: "missing orig", args: []string{"--input", "in", "--output", "out"}},
		{name: "missing input", args: []string{"--orig", "orig", "--output", "out"}},
		{name: "missing output", args: []string{"--orig", "orig", "--input", "in"}},
		{name: "positional", args: []string{"--orig", "orig", "--input", "in", "--output", "out", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewMergeCommand()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("Execute() error = nil, want validation error")
			}
		})
	}
}

func mergeDirectories(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	orig := filepath.Join(root, "orig")
	input := filepath.Join(root, "input")
	output := filepath.Join(root, "output")
	for _, path := range []string{orig, input} {
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return orig, input, output
}

func executeMerge(t *testing.T, orig, input, output string, extra ...string) (string, error) {
	t.Helper()
	args := []string{"--orig", orig, "--input", input, "--output", output}
	args = append(args, extra...)
	var stderr bytes.Buffer
	cmd := NewMergeCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stderr.String(), err
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertTestFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}
