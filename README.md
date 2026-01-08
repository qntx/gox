# gox

**CGO cross-compilation powered by Zig.**

Cross-compile Go programs with C dependencies to any platform—without installing platform-specific toolchains, Docker containers, or complex build configurations. `gox` leverages [Zig](https://ziglang.org/)'s hermetic C/C++ compiler to provide a seamless cross-compilation experience.

## Features

- **Zero Configuration** — Zig compiler auto-downloaded and cached on first use
- **Any Host → Any Target** — Build from Windows/macOS/Linux to any supported platform
- **Static Binaries** — Produce fully self-contained executables with musl libc

## Installation

```bash
go install github.com/qntx/gox/cmd/gox@latest
```

## Quick Start

```bash
gox build                                            # Interactive mode (TUI)
gox build --os linux --arch arm64                    # Cross-compile to Linux ARM64
gox build --os linux --arch amd64 --linkmode static  # Static binary
```

## Usage

### Basic Syntax

```bash
gox build [packages] [flags]
```

### Example

```bash
# Cross-compile to different platforms
gox build --os darwin --arch arm64                   # macOS Apple Silicon
gox build --os windows --arch amd64 -o app.exe       # Windows x64
gox build --os linux --arch riscv64                  # Linux RISC-V

# Static linking (Linux)
gox build --os linux --arch amd64 --linkmode static

# Link external C libraries
gox build --os linux --arch amd64 \
    -I/usr/include/openssl \
    -L/usr/lib \
    -lssl -lcrypto

# Pass flags to go build
gox build --flags "-tags=prod" --flags "-trimpath" --flags "-ldflags=-s -w"
```

## Command Reference

### `gox build`

| Flag | Short | Description |
| :--- | :---: | :--- |
| `--os` | | Target operating system |
| `--arch` | | Target architecture |
| `--output` | `-o` | Output binary path |
| `--linkmode` | | Link mode: `auto` (default), `static`, `dynamic` |
| `--include` | `-I` | C header include directories (repeatable) |
| `--lib` | `-L` | Library search directories (repeatable) |
| `--link` | `-l` | Libraries to link (repeatable) |
| `--zig-version` | | Zig compiler version (default: `master`) |
| `--flags` | | Additional flags passed to `go build` (repeatable) |
| `--interactive` | `-i` | Launch interactive TUI for configuration |
| `--verbose` | `-v` | Print detailed build information |

### `gox zig`

Manage Zig compiler installations cached in `~/.cache/gox/zig/`.

| Command | Description |
| :--- | :--- |
| `gox zig update [version]` | Install or update Zig (default: `master`) |
| `gox zig list` | List all cached Zig versions |
| `gox zig clean [version]` | Remove cached Zig installations |

## Platform Support

### Architecture Compatibility

`gox` supports the intersection of Go and Zig targets. The following table provides a complete reference:

| OS | Go | Zig | gox |
| :--- | :--- | :--- | :--- |
| **Linux** | amd64, arm64, 386, arm, riscv64, loong64, mips64, mips64le, ppc64, ppc64le, s390x, mips, mipsle | amd64, arm64, 386, arm, riscv64, loong64, mips64, mips64le, ppc64, ppc64le, s390x, mips, mipsle | amd64, arm64, 386, arm, riscv64, loong64, mips64, mips64le, ppc64, ppc64le, s390x, mips, mipsle |
| **macOS** | amd64, arm64 | amd64, arm64 | amd64, arm64 |
| **Windows** | amd64, 386, arm64 | amd64, 386, arm64 | amd64, 386, arm64 |
| **FreeBSD** | amd64, 386, arm, arm64, riscv64 | amd64, arm64, 386, arm | amd64, arm64, 386, arm |
| **NetBSD** | amd64, 386, arm, arm64 | amd64, arm64, 386, arm | amd64, arm64, 386, arm |
| **OpenBSD** | amd64, 386, arm, arm64, ppc64, riscv64 | amd64, arm64, 386, arm | amd64, arm64, 386, arm |
| **DragonFly** | amd64 | amd64 | amd64 |
| **Solaris** | amd64 | amd64 | amd64 |
| **illumos** | amd64 | amd64 | amd64 |
| **iOS** | amd64, arm64 | amd64, arm64 | amd64, arm64 |
| **Android** | amd64, 386, arm, arm64 | amd64, arm64, 386, arm | amd64, arm64, 386, arm |

### Unsupported Targets

| Target | Reason |
| :--- | :--- |
| `js/wasm`, `wasip1/wasm` | WebAssembly does not support CGO |
| `plan9/*` | Plan 9 does not support CGO |
| `aix/ppc64` | Zig does not provide AIX libc support |

### Static Linking

| OS | Support | Notes |
| :--- | :---: | :--- |
| Linux | ✅ | Automatically uses musl libc for fully static binaries |
| Windows | ✅ | mingw-w64 provides full static linking support |
| FreeBSD | ✅ | Native libc supports static linking |
| NetBSD | ✅ | Native libc supports static linking |
| OpenBSD | ✅ | Native libc supports static linking |
| macOS | ⚠️ | Apple discourages and limits static linking |
| iOS | ⚠️ | Limited; requires Apple code signing for deployment |
| Android | ❌ | Android requires dynamic linking to system libraries |

## How It Works

1. **Zig Download** — On first run, `gox` downloads the Zig compiler for your host platform and caches it in `~/.cache/gox/zig/<version>`.

2. **Environment Setup** — Sets `CC` and `CXX` to use Zig with the appropriate target triple:

   ```bash
   CC="zig cc -target x86_64-linux-gnu"
   CXX="zig c++ -target x86_64-linux-gnu"
   ```

3. **Build Execution** — Runs `go build` with `CGO_ENABLED=1` and the configured cross-compilation environment.

Zig's C/C++ compiler is a drop-in replacement for GCC/Clang that ships with libc headers and libraries for all supported targets, eliminating the need for platform-specific cross-compilation toolchains.

## Examples

Working examples are available in the [examples/](./example) directory:

| Example | Description |
| :--- | :--- |
| [minimal](./example/minimal) | Basic CGO with inline C code |
| [sqlite](./example/sqlite) | Cross-compile go-sqlite3 with vendored C source |
| [zlib](./example/zlib) | Link against system zlib library |

## License

Gox has BSD 3-Clause License, see LICENSE.
