# SQLite CGO Example

Cross-compile go-sqlite3 with gox.

## Run

```bash
gox run .
```

## Build

```bash
# Linux static binary (fully portable)
gox build --os linux --arch amd64 --linkmode static -o sqlite-linux

# Windows
gox build --os windows --arch amd64 -o sqlite.exe
```
