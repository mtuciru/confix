# confix

Small, zero-dependency configuration helper for Go that reads and writes JSON, YAML, and TOML files.

It focuses on two things:
- Make it trivial to load config from a file (or a directory of well-known file names) into your struct.
- Make it easy to write the effective config back to disk in a stable, pretty format — atomically.

No code generation. No global singletons. No reflection tricks beyond what standard encoders already do.

## Features

- Load into your own struct type using the standard `encoding/json`, `gopkg.in/yaml.v3`, and `github.com/BurntSushi/toml` decoders.
- Supported file formats: `.json`, `.yaml`, `.yml`, `.toml`.
- Config discovery via environment variables or sane defaults:
  - `CONFIG_FILE_PATH` — load exactly this file; create it if missing.
  - `CONFIG_DIR_PATH` — look for `config.json`, `config.toml`, `config.yml`, `config.yaml` in that directory.
  - If neither is set — look for the same file names in the current working directory (both `./` and absolute executable dir path are checked).
- Write-back helpers:
  - `WithWritingConfigToFile(path)` — write the effective config to a file.
  - `WithSyncingConfigToFiles()` — write to all discovered config paths at once.
  - Atomic writes: temp file + rename.
- Optional validation hook: `WithValidation(func(*T) error)`.

Note: The library does not map environment variables into struct fields. Environment is used only to locate config files.

## Installation

```bash
go get github.com/mtuciru/confix
```

Requires Go 1.20+ (the project’s `go` directive is 1.25).

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/mtuciru/confix"
)

type Config struct {
    A string `json:"a" yaml:"a" toml:"a"`
    B int    `json:"b" yaml:"b" toml:"b"`
}

func main() {
    // Point to a directory and let confix find config.json|.toml|.yml|.yaml there.
    _ = confix.SetConfigDir(os.TempDir())

    // Seed defaults – any fields not present in files will keep these values.
    cfg := &Config{A: "default", B: 1}

    if err := confix.New(cfg, confix.WithSyncingConfigToFiles[Config]()); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("A=%s B=%d\n", cfg.A, cfg.B)
}
```

See `example_test.go` for a complete, runnable example.

## Configuration Lookup Order

At initialization, confix resolves file paths as follows:

1. If `CONFIG_FILE_PATH` is set:
   - Use exactly that file.
   - If the file does not exist, it will be created and initialized with the current struct contents.
2. Else if `CONFIG_DIR_PATH` is set:
   - Look for these files inside the directory, in this order:
     - `config.json`
     - `config.toml`
     - `config.yml`
     - `config.yaml`
   - All existing files are considered; each subsequent file can override values decoded from the previous ones.
3. Else (no env vars set):
   - Look for the same file names in the current working directory and in the executable’s directory.

When multiple files are found, they are decoded sequentially into the same struct. Later files overwrite earlier values (the standard library decoders behave this way when decoding into an already-populated struct).

Empty files are ignored (treated as no content).

## Writing and Syncing Config

Use options passed to `New` to emit the effective config to disk:

- `WithWritingConfigToFile(path)` — write the config to a specific path.
- `WithSyncingConfigToFiles()` — write to all discovered config paths.

Writes are atomic: data is encoded into a temp file and then `rename`d to the target path.

Encoders format output in a stable way:
- JSON: indented with two spaces.
- YAML: indented with two spaces.
- TOML: default encoder from `BurntSushi/toml`.

## Validation

Add a validation step that runs after loading and before writing:

```go
err := confix.New(cfg, confix.WithValidation(func(c *Config) error {
    if c.B < 0 {
        return fmt.Errorf("B must be non-negative")
    }
    return nil
}))
```

If the validator returns an error, initialization fails and no write-back is performed.

## Supported Tags

Use the standard struct tags for the target encoders. For example:

```go
type Config struct {
    A string `json:"a" yaml:"a" toml:"a"`
}
```

The library does not interpret custom tags; it simply delegates to the chosen decoder.

## API Overview

```go
// Load config into cfg and optionally apply post-load options.
func New[T any](cfg *T, opts ...Option[T]) error

// Environment variables to select where config files are located.
func SetConfigDir(dir string) error      // sets CONFIG_DIR_PATH
func SetConfigPath(path string) error    // sets CONFIG_FILE_PATH

// Options
func WithValidation[T any](f func(*T) error) Option[T]
func WithWritingConfigToFile[T any](path string) Option[T]
func WithSyncingConfigToFiles[T any]() Option[T]
```

## Error Handling

- File decoding errors are wrapped with a descriptive message, e.g., "error while decoding yaml file".
- When syncing to multiple files, write errors are aggregated using `errors.Join`.
- If `CONFIG_FILE_PATH` points to a non-existent file, confix creates it and writes the current config.

## FAQ

Q: Does confix load values from environment variables into struct fields?  
A: No. Only file discovery is controlled via env vars. Field values come from your defaults and decoded file content.

Q: What happens if multiple config files exist?  
A: Files are decoded in discovery order; later files overwrite earlier fields.

Q: Are writes safe if my program crashes mid-write?  
A: Writes use temp files and an atomic rename to minimize the risk of partial files.

## Version Compatibility

- Go 1.20+.

## Contributing

Issues and PRs are welcome. Please include tests for behavior changes. Run the test suite with:

```bash
go test ./...
```

## License


This project is licensed under the MIT License.  
See the [LICENSE file](./LICENSE) for details.