# gox

Cross-platform CGO build tool powered by Zig.

## Install

```bash
go install gox.qntx.fun@latest
```

## Usage

### Interactive Mode

```bash
gox build -i
```

Select target platform, link mode, and configure paths interactively.

### Direct Build

```bash
# Build for Linux ARM64
gox build --os linux --arch arm64

# Build with static linking
gox build --os linux --arch amd64 --linkmode static

# Build with custom include/lib paths
gox build --os linux --arch amd64 -I/usr/include -L/usr/lib -lssl -lcrypto

# Specify output
gox build --os windows --arch amd64 -o app.exe
```

### Flags

| Flag | Short | Description |
| ----------- | ----------- | ------------ |
| `--output` | `-o` | Output file path |
| `--os` | | Target OS (linux, darwin, windows, freebsd) |
| `--arch` | | Target arch (amd64, arm64, 386, arm) |
| `--zig-version` | | Zig compiler version (default: master) |
| `--include` | `-I` | C header include directories |
| `--lib` | `-L` | Library search directories |
| `--link` | `-l` | Libraries to link |
| `--linkmode` | | Link mode: static, dynamic, auto |
| `--flags` | | Additional go build flags |
| `--interactive` | `-i` | Interactive mode |
| `--verbose` | `-v` | Verbose output |

## How It Works

1. Downloads Zig compiler on first run (cached in user cache directory)
2. Sets `CC` and `CXX` to `zig cc` and `zig c++` with target triple
3. Runs `go build` with `CGO_ENABLED=1` and appropriate flags

## Supported Targets

- linux/amd64, linux/arm64, linux/386, linux/arm, linux/riscv64
- darwin/amd64, darwin/arm64
- windows/amd64, windows/386, windows/arm64
- freebsd/amd64

## License

BSD 3-Clause License
