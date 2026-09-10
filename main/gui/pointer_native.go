//go:build !js

package main

// Native focus is checked directly with ebiten.IsFocused.
func pointerInterrupted() bool { return false }
