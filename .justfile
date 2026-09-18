# SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
# SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
# 
# SPDX-License-Identifier: GPL-3.0-only

@default:
	just --list

update-parser:
	kaitai-struct-compiler --target go --go-package parser --outdir . kaitai/*.ksy

build:
	go build .

build-win:
	GOOS=windows GOARCH=amd64 go build .

build-mini-win:
	GOOS=windows GOARCH=amd64 go build -o ../ukr-into-the-breach/mini/windows/sbt-itb-mini.exe ./mini

build-mini-linux:
	GOOS=linux GOARCH=amd64 go build -o ../ukr-into-the-breach/mini/linux/sbt-itb-mini ./mini

build-mini-mac:
	GOOS=darwin GOARCH=arm64 go build -o ../ukr-into-the-breach/mini/macos/sbt-itb-mini-arm64 ./mini
	GOOS=darwin GOARCH=amd64 go build -o ../ukr-into-the-breach/mini/macos/sbt-itb-mini-amd64 ./mini
	lipo -create -output ../ukr-into-the-breach/mini/macos/sbt-itb-mini ../ukr-into-the-breach/mini/macos/sbt-itb-mini-arm64 ../ukr-into-the-breach/mini/macos/sbt-itb-mini-amd64
	rm ../ukr-into-the-breach/mini/macos/sbt-itb-mini-arm64 ../ukr-into-the-breach/mini/macos/sbt-itb-mini-amd64

build-mini-all: build-mini-win build-mini-linux build-mini-mac

test:
	go test -v ./...
