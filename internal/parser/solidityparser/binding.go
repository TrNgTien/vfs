package solidityparser

// Vendor the tree-sitter-solidity parser (v1.2.13) because the upstream
// Go binding (github.com/JoranHonig/tree-sitter-solidity) declares a
// mismatched module path and depends on the legacy smacker/go-tree-sitter
// library instead of the official tree-sitter/go-tree-sitter used by vfs.

// #cgo CFLAGS: -std=c11 -I${SRCDIR}/src
// #cgo !windows CFLAGS: -fPIC
// #include "src/parser.c"
import "C"

import "unsafe"

// Language returns the tree-sitter Language pointer for Solidity.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_solidity())
}
