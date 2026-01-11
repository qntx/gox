# gox

**CGO cross-compilation powered by Zig.**

Cross-compile Go programs with C dependencies to any platform—without installing platform-specific toolchains, Docker containers, or complex build configurations. `gox` leverages [Zig](https://ziglang.org/)'s hermetic C/C++ compiler to provide a seamless cross-compilation experience.

## Features

- **Zero Configuration** — Zig compiler auto-downloaded and cached on first use
- **Any Host → Any Target** — Build from Windows/macOS/Linux to any supported platform
- **Static Binaries** — Produce fully self-contained executables with musl libc
- **Production Builds** — Strip symbols with `-s` for minimal binary size

## Installation

```bash
go install github.com/qntx/gox/cmd/gox@latest
```

## Usage

### Syntax

```bash
gox build [packages] [flags]
```

### Examples

```bash
# cross-compile to different platforms
gox build --os darwin --arch arm64                    # macOS Apple Silicon
gox build --os windows --arch amd64 -o app.exe        # Windows x64
gox build --os linux --arch riscv64                   # Linux RISC-V

# static linking
gox build --os linux --arch amd64 --linkmode static

# production build with stripped symbols
gox build -s -o app

# link external C libraries
gox build -I/usr/include/openssl -L/usr/lib -lssl -lcrypto

# pass flags to go build
gox build --flags "-tags=prod" --flags "-trimpath"

# output to prefix directory with rpath
gox build --os linux --arch amd64 --prefix ./dist

# build and create archive
gox build --os linux --arch amd64 -o ./app --pack     # creates app-linux-amd64.tar.gz
gox build --os windows --arch amd64 --prefix ./dist --pack

# parallel builds
gox build -j
```

See examples in the [example](./example) directory.

## Configuration

Create `gox.toml` in your project root:

```toml
[default]
zig-version = "0.15.2"
strip       = true
verbose     = false

[[target]]
name     = "linux-amd64"
os       = "linux"
arch     = "amd64"
prefix   = "./dist/linux"
packages = ["gocnn-lib/cudart@v12.9.79/linux-amd64.tar.xz"]
link     = ["cuda", "cublas"]
flags    = ["-tags=cuda"]

[[target]]
name     = "windows-amd64"
os       = "windows"
arch     = "amd64"
prefix   = "./dist/windows"
pack     = true
```

```bash
gox build                                             # build all targets
gox build -j                                          # build all targets in parallel
gox build -t linux-amd64                              # build specific target
gox build -t linux-amd64 --verbose                    # override config
gox build -c ./build/gox.toml                         # custom config path
```

### Configuration Reference

#### `[default]`

Global defaults applied to all targets.

| Key | Type | Description |
| :--- | :--- | :--- |
| `zig-version` | `string` | Zig compiler version |
| `linkmode` | `string` | Link mode: `auto`, `static`, `dynamic` |
| `include` | `[]string` | C header include directories |
| `lib` | `[]string` | Library search directories |
| `link` | `[]string` | Libraries to link |
| `packages` | `[]string` | Pre-built packages to download |
| `flags` | `[]string` | Additional go build flags |
| `strip` | `bool` | Strip symbols (`-ldflags="-s -w"`) |
| `verbose` | `bool` | Enable verbose output |

#### `[[target]]`

Build target definitions. Multiple targets can be defined.

| Key | Type | Description |
| :--- | :--- | :--- |
| `name` | `string` | Target identifier for `--target` flag |
| `os` | `string` | Target operating system |
| `arch` | `string` | Target architecture |
| `output` | `string` | Output binary path |
| `prefix` | `string` | Output prefix directory |
| `zig-version` | `string` | Zig version (overrides default) |
| `linkmode` | `string` | Link mode (overrides default) |
| `include` | `[]string` | C header include directories |
| `lib` | `[]string` | Library search directories |
| `link` | `[]string` | Libraries to link |
| `packages` | `[]string` | Pre-built packages to download |
| `flags` | `[]string` | Additional go build flags |
| `no-rpath` | `bool` | Disable rpath |
| `pack` | `bool` | Create archive after build |
| `strip` | `bool` | Strip symbols (overrides default) |
| `verbose` | `bool` | Verbose output (overrides default) |

## Package Management

Download and configure pre-built libraries automatically:

```bash
# GitHub release: owner/repo@version/asset
gox build --pkg owner/repo@v1.0.0/lib-linux-amd64.tar.gz

# direct URL
gox build --pkg https://example.com/lib-1.0.0-linux.tar.gz

# multiple packages
gox build --pkg owner/cuda@v1.0/cuda.tar.gz --pkg owner/ssl@v3.0/ssl.tar.gz
```

### Package Structure

Downloaded packages must contain `include/` and/or `lib/` directories:

```text
package.tar.gz
├── include/          # added to CGO_CFLAGS -I
└── lib/              # added to CGO_LDFLAGS -L
```

**Cache:** `~/.cache/gox/pkg/`

## Command Reference

### `gox build`

| Flag | Short | Description |
| :--- | :---: | :--- |
| `--config` | `-c` | Config file path (default: `gox.toml`) |
| `--target` | `-t` | Build target name from config |
| `--os` | | Target operating system |
| `--arch` | | Target architecture |
| `--output` | `-o` | Output binary path |
| `--prefix` | | Output prefix directory with rpath |
| `--zig-version` | | Zig compiler version (default: `master`) |
| `--linkmode` | | Link mode: `auto`, `static`, `dynamic` |
| `--include` | `-I` | C header include directories |
| `--lib` | `-L` | Library search directories |
| `--link` | `-l` | Libraries to link |
| `--pkg` | | Pre-built packages to download |
| `--flags` | | Additional flags passed to `go build` |
| `--no-rpath` | | Disable rpath when using `--prefix` |
| `--pack` | | Create archive after build |
| `--strip` | `-s` | Strip symbols (`-ldflags="-s -w"`) |
| `--verbose` | `-v` | Print detailed build information |
| `--parallel` | `-j` | Build targets in parallel |

### `gox pkg`

Manage cached dependency packages in `~/.cache/gox/pkg/`.

| Command | Description |
| :--- | :--- |
| `gox pkg list` | List cached packages |
| `gox pkg info <name>` | Show package details |
| `gox pkg install <source>...` | Download packages to cache |
| `gox pkg clean [name]` | Remove cached packages |

### `gox zig`

Manage Zig compiler installations in `~/.cache/gox/zig/`.

| Command | Description |
| :--- | :--- |
| `gox zig update [version]` | Install or update Zig (default: `master`) |
| `gox zig list` | List cached Zig versions |
| `gox zig clean [version]` | Remove cached Zig installations |

## Platform Support

### Supported Targets

| OS | Architectures |
| :--- | :--- |
| Linux | amd64, arm64, 386, arm, riscv64, loong64, ppc64le, s390x |
| Windows | amd64, arm64, 386 |
| macOS | amd64 |
| FreeBSD | amd64, 386 |
| NetBSD | amd64, arm64, 386, arm |

### Unsupported Targets

| Target | Reason |
| :--- | :--- |
| `darwin/arm64` | Go runtime requires CoreFoundation framework unavailable in Zig |
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

## How It Works

1. **Zig Download** — Downloads the Zig compiler for your host platform and caches it in `~/.cache/gox/zig/<version>`.

2. **Environment Setup** — Sets `CC` and `CXX` to use Zig with the appropriate target triple:

   ```bash
   CC="zig cc -target x86_64-linux-gnu"
   CXX="zig c++ -target x86_64-linux-gnu"
   ```

3. **Build Execution** — Runs `go build` with `CGO_ENABLED=1` and the configured environment.

Zig's C/C++ compiler is a drop-in replacement for GCC/Clang that ships with libc headers and libraries for all supported targets, eliminating the need for platform-specific cross-compilation toolchains.

## License

BSD 3-Clause License. See [LICENSE](./LICENSE).
