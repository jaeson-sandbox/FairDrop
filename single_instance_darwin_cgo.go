//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>
static char *fairdropNativeTempDir(void) {
	@autoreleasepool { return strdup([NSTemporaryDirectory() UTF8String]); }
}
*/
import "C"

import "unsafe"

// Wails uses NSTemporaryDirectory, which can differ from Go's os.TempDir in
// a sandbox. Probe that exact location, not an unrelated writable directory.
func nativeLockTempDir() string {
	directory := C.fairdropNativeTempDir()
	if directory == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(directory))
	return C.GoString(directory)
}
