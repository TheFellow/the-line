//go:build !browsercheck || !js || !wasm

package main

func attachInspection(*game) {}
func inspectFrame(*game)     {}
