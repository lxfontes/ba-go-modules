// Code generated by wit-bindgen-go. DO NOT EDIT.

// Package preopens represents the interface "wasi:filesystem/preopens@0.2.0".
package preopens

import (
	"github.com/ydnar/wasm-tools-go/cm"
	"github.com/ydnar/wasm-tools-go/wasi/filesystem/types"
)

// GetDirectories represents function "wasi:filesystem/preopens@0.2.0#get-directories".
//
// Return the set of preopened directories, and their path.
//
//	get-directories: func() -> list<tuple<own<descriptor>, string>>
//
//go:nosplit
func GetDirectories() cm.List[cm.Tuple[types.Descriptor, string]] {
	var result cm.List[cm.Tuple[types.Descriptor, string]]
	wasmimportGetDirectories(&result)
	return result
}

//go:wasmimport wasi:filesystem/preopens@0.2.0 get-directories
//go:noescape
func wasmimportGetDirectories(result *cm.List[cm.Tuple[types.Descriptor, string]])
