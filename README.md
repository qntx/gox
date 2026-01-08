# gox

**Zero-dependency CGO cross-compilation using Zig as the C/C++ toolchain.**

`gox` eliminates the complexity of cross-compiling Go projects with CGO by leveraging [Zig](https://ziglang.org/)'s universal C/C++ compiler. No platform-specific toolchains, no Docker containers—just one command.

## Features

- **Zero Setup**: Automatically downloads and caches Zig compiler on first use
- **Universal Toolchain**: Cross-compile to any target from any host platform
- **Static Linking**: Produce fully static binaries with `--linkmode static`
- **Interactive Mode**: TUI for selecting targets and configuring build options
- **Drop-in Replacement**: Works with existing CGO projects without modification

## Installation

```bash
go install github.com/qntx/gox/cmd/gox@latest
```

## Quick Start

```bash
# Interactive mode - guided configuration
gox build -i

# Cross-compile to Linux ARM64
gox build --os linux --arch arm64

# Static binary for Linux
gox build --os linux --arch amd64 --linkmode static -o app

# With external C libraries
gox build --os linux --arch amd64 \
  -I/usr/include/openssl \
  -L/usr/lib/x86_64-linux-gnu \
  -lssl -lcrypto
```

## Usage

### Basic Cross-Compilation

```bash
# Target specification
gox build --os <OS> --arch <ARCH> [flags] [packages]

# Examples
gox build --os darwin --arch arm64        # macOS Apple Silicon
gox build --os windows --arch amd64       # Windows x64
gox build --os linux --arch arm64         # Linux ARM64
```

### Static Linking

```bash
# Fully static binary (no libc dependency)
gox build --os linux --arch amd64 --linkmode static

# Static with external libraries
gox build --os linux --arch amd64 --linkmode static \
  -L/path/to/libs -lsqlite3
```

### C Library Integration

```bash
# Specify include paths and libraries
gox build \
  --include /usr/local/include \
  --lib /usr/local/lib \
  --link curl \
  --link ssl
```

### Advanced Options

```bash
# Custom Zig version
gox build --zig-version 0.11.0 --os linux --arch amd64

# Pass additional go build flags
gox build --flags "-tags=prod" --flags "-trimpath"

# Verbose output for debugging
gox build -v --os linux --arch arm64
```

## Command Reference

### Flags

| Flag | Alias | Description |
| ------ | ------ | ------ |
| `--os` | | Target OS: `linux`, `darwin`, `windows`, `freebsd` |
| `--arch` | | Target architecture: `amd64`, `arm64`, `386`, `arm`, `riscv64` |
| `--output` | `-o` | Output binary path |
| `--linkmode` | | Linking mode: `static`, `dynamic`, `auto` (default) |
| `--include` | `-I` | C header include directories (repeatable) |
| `--lib` | `-L` | Library search paths (repeatable) |
| `--link` | `-l` | Libraries to link (repeatable) |
| `--zig-version` | | Zig compiler version (default: `master`) |
| `--flags` | | Additional `go build` flags (repeatable) |
| `--interactive` | `-i` | Launch interactive TUI mode |
| `--verbose` | `-v` | Enable verbose output |

## Supported Targets

| OS | Architectures |
| ------ | ------ |
| Linux | amd64, arm64, 386, arm, riscv64 |
| macOS | amd64, arm64 |
| Windows | amd64, 386, arm64 |
| FreeBSD | amd64 |

## How It Works

1. **Zig Download**: On first run, downloads the Zig compiler matching your host platform to `~/.cache/gox/zig/<version>`
2. **Compiler Setup**: Configures `CC=zig cc` and `CXX=zig c++` with the target triple (e.g., `x86_64-linux-gnu`)
3. **CGO Build**: Executes `go build` with `CGO_ENABLED=1` and the configured environment

Zig's C/C++ compiler is a drop-in replacement for gcc/clang that can cross-compile to any target without installing target-specific toolchains.

## Examples

See [examples/](./example) for complete working examples:

- **minimal**: Inline C code with no external dependencies
- **sqlite**: Cross-compile go-sqlite3 with vendored C library
- **zlib**: Link against system zlib library

## Troubleshooting

### Binary not found after installation

Ensure `$GOPATH/bin` (or `$GOBIN`) is in your `PATH`:

```bash
# Check GOPATH
go env GOPATH

# Add to PATH (Linux/macOS)
export PATH="$(go env GOPATH)/bin:$PATH"

# Add to PATH (Windows PowerShell)
$env:PATH += ";$(go env GOPATH)\bin"
```

### Zig download fails

Specify a stable Zig version instead of `master`:

```bash
gox build --zig-version 0.11.0 --os linux --arch amd64
```

### Linking errors with external libraries

Ensure library paths are correct and libraries exist for the target architecture:

```bash
# Verify library exists
ls /usr/lib/x86_64-linux-gnu/libssl.a

# Use absolute paths
gox build -L/usr/lib/x86_64-linux-gnu -lssl
```

## License

BSD 3-Clause License
