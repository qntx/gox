# zlib CGO Example

Compression using zlib. Demonstrates linking external C libraries.

> **Note**: Cross-compiling with external C libraries requires target platform headers and libraries.

## Native Build

```bash
gox build -lz
```

## Cross-Compile

Requires pre-built zlib for target platform:

```bash
gox build --os linux --arch amd64 \
    -I /path/to/linux-amd64/include \
    -L /path/to/linux-amd64/lib \
    -lz
```

## Static Linking

```bash
gox build --os linux --arch amd64 --linkmode static \
    -I /path/to/linux-amd64/include \
    -L /path/to/linux-amd64/lib \
    -lz
```
