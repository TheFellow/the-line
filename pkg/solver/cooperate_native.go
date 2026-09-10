//go:build !js

package solver

// Native goroutines already permit the UI thread to run independently.
func cooperate() {}
