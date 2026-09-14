// SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
// SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
//
// SPDX-License-Identifier: GPL-3.0-only

package dat

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestWriteValidatesUint32BodyLengthsAndMetadataOffsetsBeforeWriting(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry
		want    string
	}{
		{
			name:    "body length",
			entries: []Entry{{Path: "large.bin", Size: uint64(math.MaxUint32) + 1}},
			want:    "body length",
		},
		{
			name: "metadata offset",
			entries: []Entry{
				{Path: "large.bin", Size: math.MaxUint32},
				{Path: "next.bin", Size: 0},
			},
			want: "metadata offset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			err := Write(&output, tt.entries, nil)

			if err == nil {
				t.Fatal("Write() error = nil, want DAT limit error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want %q context", err, tt.want)
			}
			if output.Len() != 0 {
				t.Errorf("wrote %d bytes before validation, want 0", output.Len())
			}
		})
	}
}
