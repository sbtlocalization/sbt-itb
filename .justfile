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

test:
	go test -v ./...
