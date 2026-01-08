package main

/*
#cgo LDFLAGS: -lcurl
#include <stdio.h>
#include <stdlib.h>
#include <curl/curl.h>

typedef struct {
    char *data;
    size_t size;
} Response;

static size_t write_callback(void *contents, size_t size, size_t nmemb, void *userp) {
    size_t realsize = size * nmemb;
    Response *resp = (Response *)userp;
    char *ptr = realloc(resp->data, resp->size + realsize + 1);
    if (!ptr) return 0;
    resp->data = ptr;
    memcpy(&(resp->data[resp->size]), contents, realsize);
    resp->size += realsize;
    resp->data[resp->size] = 0;
    return realsize;
}

char* http_get(const char *url) {
    CURL *curl = curl_easy_init();
    if (!curl) return NULL;

    Response resp = {0};
    curl_easy_setopt(curl, CURLOPT_URL, url);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, write_callback);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &resp);
    curl_easy_setopt(curl, CURLOPT_FOLLOWLOCATION, 1L);

    CURLcode res = curl_easy_perform(curl);
    curl_easy_cleanup(curl);

    if (res != CURLE_OK) {
        free(resp.data);
        return NULL;
    }
    return resp.data;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func Get(url string) (string, error) {
	curl := C.CString(url)
	defer C.free(unsafe.Pointer(curl))

	resp := C.http_get(curl)
	if resp == nil {
		return "", fmt.Errorf("request failed")
	}
	defer C.free(unsafe.Pointer(resp))

	return C.GoString(resp), nil
}

func main() {
	body, err := Get("https://httpbin.org/get")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(body[:min(len(body), 200)])
}
