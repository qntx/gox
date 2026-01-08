# libcurl CGO Example

HTTP client using libcurl with cross-compilation.

## Build

```bash
# Linux with system libcurl
gox build --os linux --arch amd64 -lcurl

# Static build (requires static libcurl)
gox build --os linux --arch amd64 --linkmode static -lcurl -lssl -lcrypto -lz

# Custom library path
gox build --os linux --arch amd64 -L/opt/curl/lib -I/opt/curl/include -lcurl
```

## Note

Requires libcurl development headers on build host for target platform.
