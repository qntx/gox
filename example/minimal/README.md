# Minimal CGO Example

Basic CGO with inline C code.

## Build

```bash
# Native
gox build

# Cross-compile to Linux
gox build --os linux --arch amd64

# Cross-compile to Windows
gox build --os windows --arch amd64 -o minimal.exe
```
