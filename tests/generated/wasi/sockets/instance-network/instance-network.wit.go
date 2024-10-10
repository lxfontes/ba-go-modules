// Code generated by wit-bindgen-go. DO NOT EDIT.

// Package instancenetwork represents the imported interface "wasi:sockets/instance-network@0.2.0".
//
// This interface provides a value-export of the default network handle..
package instancenetwork

import (
	"github.com/bytecodealliance/wasm-tools-go/cm"
	"tests/generated/wasi/sockets/network"
)

// Network represents the imported type alias "wasi:sockets/instance-network@0.2.0#network".
//
// See [network.Network] for more information.
type Network = network.Network

// InstanceNetwork represents the imported function "instance-network".
//
// Get a handle to the default network.
//
//	instance-network: func() -> network
//
//go:nosplit
func InstanceNetwork() (result Network) {
	result0 := wasmimport_InstanceNetwork()
	result = cm.Reinterpret[Network]((uint32)(result0))
	return
}
