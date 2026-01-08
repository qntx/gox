package main

/*
#include <stdio.h>
#include <stdlib.h>

void hello(const char* name) {
    printf("Hello, %s!\n", name);
}

int add(int a, int b) {
    return a + b;
}
*/
import "C"
import "unsafe"

func main() {
	name := C.CString("gox")
	defer C.free(unsafe.Pointer(name))

	C.hello(name)

	sum := C.add(3, 4)
	println("3 + 4 =", sum)
}
