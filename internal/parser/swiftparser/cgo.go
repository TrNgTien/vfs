package swiftparser

// Compile the external scanner alongside the parser from the qiyue01 module.
// scanner.c is vendored here because the upstream binding omits it.

// #cgo CFLAGS: -std=c11 -fPIC -I${SRCDIR}
import "C"
