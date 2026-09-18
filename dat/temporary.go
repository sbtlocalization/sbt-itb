// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func CreateArchiveTemporary(archivePath string) (*os.File, error) {
	directory := filepath.Dir(archivePath)
	base := filepath.Base(archivePath)
	for range 10 {
		var randomBytes [8]byte
		if _, err := rand.Read(randomBytes[:]); err != nil {
			return nil, fmt.Errorf("failed to generate temporary DAT archive name: %w", err)
		}
		name := filepath.Join(directory, "."+base+"."+hex.EncodeToString(randomBytes[:])+".tmp")
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("failed to create temporary DAT archive beside %s: %w", archivePath, err)
		}
	}
	return nil, fmt.Errorf("failed to create unique temporary DAT archive beside %s", archivePath)
}
