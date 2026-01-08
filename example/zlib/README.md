# zlib CGO Example

Compression using zlib with cross-compilation.

## Build

```bash
# Linux
gox build --os linux --arch amd64 -lz

# Linux static
gox build --os linux --arch amd64 --linkmode static -lz

# macOS
gox build --os darwin --arch arm64 -lz
```
