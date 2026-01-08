# SQLite CGO Example

Cross-compile go-sqlite3 with gox.

## Build

```bash
# Linux static binary (fully portable)
gox build --os linux --arch amd64 --linkmode static -o sqlite-linux

# macOS
gox build --os darwin --arch arm64 -o sqlite-darwin

# Windows
gox build --os windows --arch amd64 -o sqlite.exe
```
