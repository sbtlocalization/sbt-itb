// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	datarchive "github.com/sbtlocalization/sbt-itb/dat"
)

const (
	sourceTreeFolderName = "ukr"
	resourcesSubfolder   = "resources"
	archiveFileName      = "resource.dat"
)

func main() {
	baseDir, err := resolveBaseDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sourceTreePath, archivePath := platformPaths(baseDir)
	fmt.Fprintf(os.Stderr, "Base directory: %s\n", baseDir)
	fmt.Fprintf(os.Stderr, "Archive source tree: %s\n", sourceTreePath)
	fmt.Fprintf(os.Stderr, "Target DAT archive: %s\n", archivePath)

	if sourceTreePath == "" || archivePath == "" {
		fmt.Fprintf(os.Stderr, "no hardcoded archive paths are defined for %s yet\n", runtime.GOOS)
		os.Exit(1)
	}

	if err := datarchive.Patch(sourceTreePath, archivePath, true, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveBaseDir() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve binary location: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("failed to resolve binary location %s: %w", executable, err)
	}
	return filepath.Dir(resolved), nil
}

func platformPaths(baseDir string) (sourceTreePath, archivePath string) {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(baseDir, sourceTreeFolderName), filepath.Join(baseDir, archiveFileName)
	case "windows", "linux":
		resourcesDir := filepath.Join(baseDir, resourcesSubfolder)
		return filepath.Join(resourcesDir, sourceTreeFolderName), filepath.Join(resourcesDir, archiveFileName)
	default:
		return "", ""
	}
}
