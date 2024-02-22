// Code generated by wit-bindgen-go. DO NOT EDIT.

// Package terminalstdin represents the interface "wasi:cli/terminal-stdin@0.2.0".
//
// An interface providing an optional `terminal-input` for stdin as a
// link-time authority.
package terminalstdin

import (
	"github.com/ydnar/wasm-tools-go/cm"
	terminalinput "github.com/ydnar/wasm-tools-go/wasi/cli/terminal-input"
)

// GetTerminalStdin represents function "wasi:cli/terminal-stdin@0.2.0#get-terminal-stdin".
//
// If stdin is connected to a terminal, return a `terminal-input` handle
// allowing further interaction with it.
//
//	get-terminal-stdin: func() -> option<own<terminal-input>>
//
//go:nosplit
func GetTerminalStdin() cm.Option[terminalinput.TerminalInput] {
	var result cm.Option[terminalinput.TerminalInput]
	wasmimportGetTerminalStdin(&result)
	return result
}

//go:wasmimport wasi:cli/terminal-stdin@0.2.0 get-terminal-stdin
//go:noescape
func wasmimportGetTerminalStdin(result *cm.Option[terminalinput.TerminalInput])