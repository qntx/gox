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
gox build                                            # Build for current platform
gox build --os linux --arch arm64                    # Cross-compile to Linux ARM64
gox build --os linux --arch amd64 --linkmode static  # Static binary with musl
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

# Standard layout: output to ./myapp/bin/myapp with rpath for ./myapp/lib/
gox build --os linux --arch amd64 --prefix ./myapp

# Build and create archive for distribution
gox build --os linux --arch amd64 -o ./app --pack
# → ./app, ./app-linux-amd64.tar.gz

gox build --os windows --arch amd64 --prefix ./myapp --pack
# → ./myapp/bin/myapp.exe, ./myapp-windows-amd64.zip
```

## Configuration File

Create a `gox.toml` file in your project root to define reusable build configurations:

```toml
[default]
zig-version = "0.15.2"
verbose = true

[[target]]
name = "linux-cuda"
os = "linux"
arch = "amd64"
include = ["C:\\cuda\\include"]
lib = ["./lib"]
link = ["cudart", "cublas"]
prefix = "./dist/linux"
flags = ["-tags=cuda"]

[[target]]
name = "windows-release"
os = "windows"
arch = "amd64"
linkmode = "static"
prefix = "./dist/windows"
pack = true
```

```bash
# Build all targets in config
gox build

# Build specific target
gox build --target linux-cuda

# CLI flags override config values
gox build --target linux-cuda --verbose=false

# Specify custom config file
gox build --config ./build/gox.toml --target linux-cuda
```

### Configuration Reference

**`[default]`** — Global defaults applied to all targets:

| Key | Type | Description |
| :--- | :--- | :--- |
| `zig-version` | string | Zig compiler version |
| `verbose` | bool | Enable verbose output |
| `pack` | bool | Create archive after build |
| `linkmode` | string | Link mode: `auto`, `static`, `dynamic` |

**`[[target]]`** — Build target definitions (can define multiple):

| Key | Type | Description |
| :--- | :--- | :--- |
| `name` | string | Target identifier for `--target` flag |
| `os` | string | Target operating system |
| `arch` | string | Target architecture |
| `output` | string | Output binary path |
| `prefix` | string | Output prefix directory |
| `no-rpath` | bool | Disable rpath |
| `include` | []string | C header include directories |
| `lib` | []string | Library search directories |
| `link` | []string | Libraries to link |
| `packages` | []string | Pre-built packages to download |
| `linkmode` | string | Link mode (overrides default) |
| `flags` | []string | Additional go build flags |
| `zig-version` | string | Zig version (overrides default) |
| `verbose` | bool | Verbose output (overrides default) |
| `pack` | bool | Create archive (overrides default) |

## Package Management

Automatically download and configure pre-built libraries for cross-compilation:

```bash
# GitHub Release format: owner/repo@version/asset
gox build --os linux --arch amd64 --pkg qntx/libs@1.0.0/cuda-linux-amd64.tar.gz

# Direct URL format
gox build --os linux --arch amd64 --pkg https://example.com/openssl-1.1.1-linux.tar.gz

# Multiple packages
gox build --pkg qntx/cuda@1.0/cuda-linux.tar.gz --pkg qntx/ssl@3.0/ssl-linux.tar.gz
```

TOML configuration:

```toml
[[target]]
name = "linux-cuda"
os = "linux"
arch = "amd64"
packages = [
  "qntx/cuda@13.1.0/cuda-linux-amd64.tar.gz",
  "qntx/ssl@3.0/openssl-linux-amd64.tar.gz"
]
link = ["cudart", "cublas", "ssl", "crypto"]
```

**Package Structure Requirements:**

Downloaded packages must contain `include/` and/or `lib/` directories:

```text
package.tar.gz
├── include/     # → automatically added to CGO_CFLAGS -I
└── lib/         # → automatically added to CGO_LDFLAGS -L
```

**Cache Location:** `~/.cache/gox/pkg/`

## Command Reference

### `gox build`

| Flag | Short | Description |
| :--- | :---: | :--- |
| `--config` | `-c` | Config file path (default: `gox.toml`) |
| `--target` | `-t` | Build target name from config |
| `--os` | | Target operating system |
| `--arch` | | Target architecture |
| `--output` | `-o` | Output binary path |
| `--prefix` | | Output prefix directory (creates `bin/lib` structure with rpath) |
| `--no-rpath` | | Disable rpath when using `--prefix` |
| `--pack` | | Create archive after build (`.tar.gz` for Linux/macOS, `.zip` for Windows) |
| `--linkmode` | | Link mode: `auto` (default), `static`, `dynamic` |
| `--include` | `-I` | C header include directories (repeatable) |
| `--lib` | `-L` | Library search directories (repeatable) |
| `--link` | `-l` | Libraries to link (repeatable) |
| `--pkg` | | Pre-built packages to download (repeatable) |
| `--zig-version` | | Zig compiler version (default: `master`) |
| `--flags` | | Additional flags passed to `go build` (repeatable) |
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
| **Linux** | amd64, arm64, 386, arm, riscv64, loong64, mips64, mips64le, ppc64, ppc64le, s390x, mips, mipsle | amd64, arm64, 386, arm, riscv64, loong64, mips64, mips64le, ppc64, ppc64le, s390x, mips, mipsle | amd64, arm64, 386, arm, riscv64, loong64, ppc64le, s390x |
| **macOS** | amd64, arm64 | amd64, arm64 | amd64 |
| **Windows** | amd64, 386, arm64 | amd64, 386, arm64 | amd64, 386, arm64 |
| **FreeBSD** | amd64, 386, arm, arm64, riscv64 | amd64, arm64, 386, arm | amd64, 386 |
| **NetBSD** | amd64, 386, arm, arm64 | amd64, arm64, 386, arm | amd64, arm64, 386, arm |
| **OpenBSD** | amd64, 386, arm, arm64, ppc64, riscv64 | amd64, arm64, 386, arm | — |
| **DragonFly** | amd64 | amd64 | — |
| **Solaris** | amd64 | amd64 | — |
| **illumos** | amd64 | amd64 | — |
| **iOS** | amd64, arm64 | amd64, arm64 | — |
| **Android** | amd64, 386, arm, arm64 | amd64, arm64, 386, arm | — |

### Unsupported Targets

| Target | Reason |
| :--- | :--- |
| `js/wasm`, `wasip1/wasm` | WebAssembly does not support CGO |
| `plan9/*` | Plan 9 does not support CGO |
| `aix/ppc64` | Zig does not provide AIX libc |
| `linux/mips*` | Go requires hard-float ABI; Zig MIPS backend incompatible |
| `linux/ppc64` | Go does not support external linking for big-endian ppc64 |
| `openbsd/*` | Zig linker does not support `-nopie` flag required by Go |
| `dragonfly/*` | Zig does not ship DragonFly BSD libc headers |
| `solaris/*`, `illumos/*` | Zig does not recognize Solaris as a valid target OS |
| `ios/*` | Zig does not ship iOS SDK headers |
| `freebsd/arm*` | Go linker requires `ld.bfd` for FreeBSD ARM |
| `android/*` | Zig does not ship Android NDK headers |
| `darwin/arm64` | Go runtime requires CoreFoundation framework unavailable in Zig |

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

Gox has BSD 3-Clause License, see [LICENSE](./LICENSE).
