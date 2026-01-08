# gox Examples

Common CGO cross-compilation scenarios.

## Examples

| Directory | Description | Dependencies |
| ----------- | ----------- | ------------ |
| `minimal` | Inline C code | None |
| `sqlite` | go-sqlite3 database | None (vendored) |
| `zlib` | Compression | zlib |

## Quick Start

```bash
# Install gox
go install github.com/qntx/gox/cmd/gox@latest

# Build minimal example for Linux
cd minimal
gox build --os linux --arch amd64

# Interactive mode
gox build -i
```

## Cross-Compile Matrix

```bash
# Linux targets
gox build --os linux --arch amd64
gox build --os linux --arch arm64
gox build --os linux --arch 386

# macOS targets
gox build --os darwin --arch amd64
gox build --os darwin --arch arm64

# Windows targets
gox build --os windows --arch amd64
gox build --os windows --arch arm64
```

## Static Linking

```bash
# Fully static binary (Linux musl)
gox build --os linux --arch amd64 --linkmode static
```
