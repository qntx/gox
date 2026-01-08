package main

/*
#cgo LDFLAGS: -lz
#include <zlib.h>
#include <stdlib.h>
#include <string.h>

int compress_data(const char *input, size_t input_len, char *output, size_t *output_len) {
    return compress((Bytef *)output, (uLongf *)output_len, (const Bytef *)input, input_len);
}

int decompress_data(const char *input, size_t input_len, char *output, size_t *output_len) {
    return uncompress((Bytef *)output, (uLongf *)output_len, (const Bytef *)input, input_len);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func Compress(data []byte) ([]byte, error) {
	outLen := C.size_t(C.compressBound(C.uLong(len(data))))
	out := make([]byte, outLen)

	ret := C.compress_data(
		(*C.char)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.char)(unsafe.Pointer(&out[0])),
		&outLen,
	)
	if ret != C.Z_OK {
		return nil, fmt.Errorf("compress failed: %d", ret)
	}
	return out[:outLen], nil
}

func Decompress(data []byte, origSize int) ([]byte, error) {
	outLen := C.size_t(origSize)
	out := make([]byte, origSize)

	ret := C.decompress_data(
		(*C.char)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.char)(unsafe.Pointer(&out[0])),
		&outLen,
	)
	if ret != C.Z_OK {
		return nil, fmt.Errorf("decompress failed: %d", ret)
	}
	return out[:outLen], nil
}

func main() {
	original := []byte("Hello, gox! This is a test of zlib compression via CGO cross-compilation.")
	fmt.Printf("Original: %d bytes\n", len(original))

	compressed, err := Compress(original)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Compressed: %d bytes\n", len(compressed))

	decompressed, err := Decompress(compressed, len(original))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Decompressed: %s\n", decompressed)
}
