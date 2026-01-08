# gox

**Zero-dependency CGO cross-compilation using Zig as the C/C++ toolchain.**

`gox` eliminates the complexity of cross-compiling Go projects with CGO by leveraging [Zig](https://ziglang.org/)'s universal C/C++ compiler. No platform-specific toolchains, no Docker containers—just one command.

## Installation

```bash
go install github.com/qntx/gox/cmd/gox@latest
```

## Quick Start

```bash
gox build
```



## Usage

```bash
gox build --os <OS> --arch <ARCH> [flags] [packages]
```

**Examples:**

```bash
gox build                                              # Interactive mode
gox build --os darwin --arch arm64                     # macOS Apple Silicon
gox build --os windows --arch amd64 -o app.exe         # Windows x64
gox build --os linux --arch arm64 --linkmode static    # Static Linux ARM64

# With C libraries
gox build --os linux --arch amd64 \
  -I/usr/include/openssl -L/usr/lib -lssl -lcrypto

# Additional go build flags
gox build --flags "-tags=prod" --flags "-trimpath"
```

## Build Flags

| Flag | Alias | Description |
| ------ | ------ | ------ |
| `--os` | | Target OS |
| `--arch` | | Target architecture |
| `--output` | `-o` | Output path |
| `--linkmode` | | `static`, `dynamic`, `auto` |
| `--include` | `-I` | C header directories |
| `--lib` | `-L` | Library search paths |
| `--link` | `-l` | Libraries to link |
| `--zig-version` | | Zig version (default: `master`) |
| `--flags` | | Additional `go build` flags |
| `--interactive` | `-i` | Interactive TUI mode |
| `--verbose` | `-v` | Verbose output |

## Zig Management

```bash
gox zig update [version]   # Install/update Zig (default: master)
gox zig list               # List installed versions
gox zig clean [version]    # Remove cached installations
```

## Platform Support

### Architecture Compatibility Matrix

| OS | Go Supported | Zig Supported | gox Supported |
| ------ | ------ | ------ | ------ |
| **Linux** | amd64, arm64, 386, arm, riscv64, loong64, mips64, mips64le, ppc64, ppc64le, s390x, mips, mipsle | amd64, arm64, 386, arm, riscv64, loong64, mips64le, ppc64le, s390x | ✅ amd64, arm64, 386, arm, riscv64, loong64, ppc64le, s390x |
| **macOS** | amd64, arm64 | amd64, arm64 | ✅ amd64, arm64 |
| **Windows** | amd64, 386, arm64 | amd64, 386, arm64 | ✅ amd64, 386, arm64 |
| **FreeBSD** | amd64, 386, arm, arm64, riscv64 | amd64, arm64, 386 | ✅ amd64, arm64 |
| **NetBSD** | amd64, 386, arm, arm64 | amd64 | ✅ amd64 |
| **OpenBSD** | amd64, 386, arm, arm64, ppc64, riscv64 | amd64, arm64 | ✅ amd64 |
| **Android** | amd64, 386, arm, arm64 | amd64, arm64, 386, arm | ✅ arm64, amd64 |

### Unsupported Targets

| Target | Reason |
| ------ | ------ |
| js/wasm, wasip1/wasm | No CGO support in WebAssembly |
| plan9/* | No CGO support in Plan 9 |
| ios/* | Requires Xcode and code signing |
| aix/ppc64 | Zig does not support AIX |
| illumos, solaris, dragonfly | Limited Zig libc support |

### Static Linking

| OS | Support | Notes |
| ------ | ------ | ------ |
| Linux | ✅ Full | Auto-switches to musl libc |
| Windows | ✅ Full | mingw-w64 static support |
| FreeBSD | ✅ Full | Native static linking |
| macOS | ⚠️ Limited | Apple discourages static linking |
| Android | ❌ None | Requires dynamic linking |

## How It Works

1. **Downloads Zig** on first run → `~/.cache/gox/zig/<version>`
2. **Sets CC/CXX** → `zig cc -target <triple>` (e.g., `x86_64-linux-gnu`)
3. **Runs go build** with `CGO_ENABLED=1`

## Examples

See [examples/](./example):

| Example | Description |
| ------ | ------ |
| [minimal](./example/minimal) | Inline C code |
| [sqlite](./example/sqlite) | go-sqlite3 with vendored C |
| [zlib](./example/zlib) | System library linking |

## License

BSD 3-Clause
