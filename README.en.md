<!--
SPDX-FileCopyrightText: © 2026 SBT Localization https://sbt.localization.com.ua
SPDX-FileContributor: Serhii Olendarenko <sergey.olendarenko@gmail.com>
SPDX-License-Identifier: GPL-3.0-only 
-->

# SBT ITB

[☑️ Українська](./README.md) | **✅ English**

This repository contains our helper tools for working with the resources of the game Into the Breach.

## Command list

The main executable `sbt-itb` is a CLI program with a number of subcommands listed below.

### Working with resources

The game's core resources, such as fonts and images, are packed into the `resource.dat` file.

- `dat list` – browse resources. Supports JSONL output (with the `--json` flag), which pairs nicely with Nushell or `jq`.
- `dat extract` – extract resources to disk.
- `dat pack` – build a new `dat` file from a folder on disk.
- `dat patch` – update individual files in an existing `dat` file: replaces existing files or adds new ones.

#### `sbt-itb-mini`

There is also the option to build a separate executable, `sbt-itb-mini`, which is essentially a trimmed-down version of `sbt-itb dat patch` with the input parameters hardcoded depending on the operating system. We ship this version of the tool with our Ukrainian localization to patch the game's resources in-place.

You can build it via
```
go build ./mini
```

### Working with CSV

Translations are stored in CSV files, and we built a separate `csv merge` command for this purpose, which in the end turned out not to be used, although it is still present in the program.

# License

Almost all of the code is available under the [GPL-3.0](./LICENSES/GPL-3.0-only.txt) license. The exception is the description of the [parser](./kaitai/ftl_dat.ksy) for `dat` files for Kaitai Struct, available under the [CC0-1.0](./LICENSES/CC0-1.0.txt) license.
