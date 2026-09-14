// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package csv

import (
	"bytes"
	stdcsv "encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

var languageCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type mergeOptions struct {
	orig         string
	input        string
	output       string
	lang         string
	column       string
	targetColumn string
	verbose      bool
}

type localizationRow struct {
	fields []string
	id     string
}

func NewMergeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge --orig <directory> --input <directory> --output <directory>",
		Short: "Merge translations into source localization entries",
		Long: `Combines canonical source localization entries with their available
language-specific translations while retaining source entries as fallbacks.`,
		Args: cobra.NoArgs,
		RunE: runMerge,
	}
	cmd.Flags().StringP("orig", "g", "", "source CSV `directory`")
	cmd.Flags().StringP("input", "i", "", "translation CSV `directory`")
	cmd.Flags().StringP("output", "o", "", "output CSV `directory`")
	cmd.Flags().StringP("lang", "l", "uk", "translation language `code`")
	cmd.Flags().StringP("column", "c", "Ukrainian", "translation column `name`")
	cmd.Flags().StringP("target-column", "t", "English", "source column `name` to replace")
	cmd.Flags().BoolP("verbose", "v", false, "report each source file")
	cmd.MarkFlagRequired("orig")
	cmd.MarkFlagRequired("input")
	cmd.MarkFlagRequired("output")
	cmd.MarkFlagDirname("orig")
	cmd.MarkFlagDirname("input")
	cmd.MarkFlagDirname("output")
	return cmd
}

func runMerge(cmd *cobra.Command, args []string) error {
	options := readMergeOptions(cmd)
	files, err := prepareMerge(options)
	if err != nil {
		return err
	}

	failed := 0
	for i, name := range files {
		if options.verbose {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "[%d/%d] %s\n", i+1, len(files), name); err != nil {
				return fmt.Errorf("failed to write progress for %s: %w", name, err)
			}
		}
		if err := processSourceFile(cmd, options, name); err != nil {
			failed++
			if _, writeErr := fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s: %v\n", name, err); writeErr != nil {
				return fmt.Errorf("failed to write error for %s: %w", name, writeErr)
			}
		}
	}

	if failed == 1 {
		return errors.New("1 source file failed")
	}
	if failed > 1 {
		return fmt.Errorf("%d source files failed", failed)
	}
	return nil
}

func readMergeOptions(cmd *cobra.Command) mergeOptions {
	orig, _ := cmd.Flags().GetString("orig")
	input, _ := cmd.Flags().GetString("input")
	output, _ := cmd.Flags().GetString("output")
	lang, _ := cmd.Flags().GetString("lang")
	column, _ := cmd.Flags().GetString("column")
	targetColumn, _ := cmd.Flags().GetString("target-column")
	verbose, _ := cmd.Flags().GetBool("verbose")
	return mergeOptions{
		orig:         orig,
		input:        input,
		output:       output,
		lang:         lang,
		column:       column,
		targetColumn: targetColumn,
		verbose:      verbose,
	}
}

func prepareMerge(options mergeOptions) ([]string, error) {
	if options.column == "" {
		return nil, errors.New("column name must not be empty")
	}
	if options.targetColumn == "" {
		return nil, errors.New("target-column name must not be empty")
	}
	if !languageCodePattern.MatchString(options.lang) {
		return nil, errors.New("language code must contain only letters, digits, underscores, or hyphens")
	}
	if err := requireDirectory(options.orig, "source"); err != nil {
		return nil, err
	}
	if err := requireDirectory(options.input, "translation"); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(options.output, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", options.output, err)
	}
	if err := requireDirectory(options.output, "output"); err != nil {
		return nil, err
	}
	if err := requireDistinctDirectories(options.orig, options.input, options.output); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(options.orig)
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory %s: %w", options.orig, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".csv") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("source directory %s contains no eligible CSV files", options.orig)
	}
	return files, nil
}

func requireDirectory(path string, kind string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to access %s directory %s: %w", kind, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s path %s is not a directory", kind, path)
	}
	return nil
}

func requireDistinctDirectories(paths ...string) error {
	type resolvedDirectory struct {
		path string
		info os.FileInfo
	}

	resolved := make([]resolvedDirectory, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("failed to resolve directory %s: %w", path, err)
		}
		for _, previous := range resolved {
			if os.SameFile(previous.info, info) {
				return fmt.Errorf("directory paths must be distinct: %s and %s resolve to the same location", previous.path, path)
			}
		}
		resolved = append(resolved, resolvedDirectory{path: path, info: info})
	}
	return nil
}

func processSourceFile(cmd *cobra.Command, options mergeOptions, name string) error {
	sourcePath := filepath.Join(options.orig, name)
	translationName := options.lang + "_" + name
	translationPath := filepath.Join(options.input, translationName)
	destination := filepath.Join(options.output, name)

	_, err := os.Stat(translationPath)
	if os.IsNotExist(err) {
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read source file: %w", err)
		}
		if err := atomicWrite(destination, data); err != nil {
			return fmt.Errorf("failed to copy source file: %w", err)
		}
		if err := writeWarning(cmd, "%s: translation file %s is missing; copied source unchanged", name, translationName); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to access translation file %s: %w", translationName, err)
	}

	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}
	translationData, err := os.ReadFile(translationPath)
	if err != nil {
		return fmt.Errorf("failed to read translation file %s: %w", translationName, err)
	}
	merged, err := mergeCSV(sourceData, translationData, options.column, options.targetColumn, func(format string, values ...any) error {
		return writeWarning(cmd, name+": "+format, values...)
	})
	if err != nil {
		return err
	}
	if err := atomicWrite(destination, merged); err != nil {
		return fmt.Errorf("failed to write merged file: %w", err)
	}
	return nil
}

func mergeCSV(sourceData []byte, translationData []byte, column string, targetColumn string, warn func(string, ...any) error) ([]byte, error) {
	sourceRecords, err := parseCSV(sourceData, "source")
	if err != nil {
		return nil, err
	}
	translationRecords, err := parseCSV(translationData, "translation")
	if err != nil {
		return nil, err
	}

	targetIndex, err := requiredHeader(sourceRecords[0], targetColumn, "source")
	if err != nil {
		return nil, err
	}
	translationIndex, err := requiredHeader(translationRecords[0], column, "translation")
	if err != nil {
		return nil, err
	}

	sourceRows, err := indexedRows(sourceRecords[1:], "source")
	if err != nil {
		return nil, err
	}
	translationRows, err := indexedRows(translationRecords[1:], "translation")
	if err != nil {
		return nil, err
	}
	translations := make(map[string]localizationRow, len(translationRows))
	for _, row := range translationRows {
		translations[row.id] = row
	}

	fallback := "source English"
	if targetColumn != "English" {
		fallback = fmt.Sprintf("source %q", targetColumn)
	}

	sourceIDs := make(map[string]struct{}, len(sourceRows))
	output := make([][]string, 1, len(sourceRows)+1)
	output[0] = sourceRecords[0]
	for _, source := range sourceRows {
		sourceIDs[source.id] = struct{}{}
		translated, exists := translations[source.id]
		if !exists {
			if err := warn("no translation for ID %q; retained %s", source.id, fallback); err != nil {
				return nil, err
			}
		} else if strings.TrimSpace(translated.fields[translationIndex]) == "" {
			if err := warn("empty translation for ID %q; retained %s", source.id, fallback); err != nil {
				return nil, err
			}
		} else {
			source.fields[targetIndex] = translated.fields[translationIndex]
		}
		output = append(output, source.fields)
	}
	for _, translated := range translationRows {
		if _, exists := sourceIDs[translated.id]; !exists {
			if err := warn("translation ID %q has no source entry; ignored", translated.id); err != nil {
				return nil, err
			}
		}
	}
	return encodeCSV(output), nil
}

func parseCSV(data []byte, kind string) ([][]string, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s CSV is not valid UTF-8", kind)
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	reader := stdcsv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s CSV: %w", kind, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s CSV has no header", kind)
	}
	if len(records[0]) == 0 || records[0][0] != "ID" {
		return nil, fmt.Errorf("first %s header must be exactly ID", kind)
	}
	return records, nil
}

func requiredHeader(header []string, name string, kind string) (int, error) {
	index := -1
	for i, value := range header {
		if value == name {
			if index >= 0 {
				return -1, fmt.Errorf("%s CSV contains duplicate %q headers", kind, name)
			}
			index = i
		}
	}
	if index < 0 {
		return -1, fmt.Errorf("%s CSV is missing required %q header", kind, name)
	}
	return index, nil
}

func indexedRows(records [][]string, kind string) ([]localizationRow, error) {
	rows := make([]localizationRow, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if emptyRecord(record) {
			continue
		}
		id := strings.TrimSpace(record[0])
		if id == "" {
			return nil, fmt.Errorf("%s CSV contains a record with an empty ID", kind)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("%s CSV contains duplicate ID %q", kind, id)
		}
		seen[id] = struct{}{}
		rows = append(rows, localizationRow{fields: record, id: id})
	}
	return rows, nil
}

func emptyRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

func encodeCSV(records [][]string) []byte {
	var output bytes.Buffer
	output.Write([]byte{0xef, 0xbb, 0xbf})
	for _, record := range records {
		for i, field := range record {
			if i > 0 {
				output.WriteByte(',')
			}
			writeCSVField(&output, field)
		}
		output.WriteString("\r\n")
	}
	return output.Bytes()
}

func writeCSVField(writer *bytes.Buffer, field string) {
	if !strings.ContainsAny(field, ",\"\r\n") {
		writer.WriteString(field)
		return
	}
	writer.WriteByte('"')
	writer.WriteString(strings.ReplaceAll(field, "\"", "\"\""))
	writer.WriteByte('"')
}

func writeWarning(cmd *cobra.Command, format string, values ...any) error {
	message := fmt.Sprintf(format, values...)
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", message); err != nil {
		return fmt.Errorf("failed to write warning: %w", err)
	}
	return nil
}

func atomicWrite(destination string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+"-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	return nil
}
