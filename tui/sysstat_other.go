//go:build !linux

package tui

// systemCPUTimes and systemMemory are Linux-only for now (no cgo); elsewhere
// the HUD hides its CPU and memory badges rather than show a guess.
func systemCPUTimes() (busy, total uint64, ok bool) { return 0, 0, false }

func systemMemory() (used, total int64, ok bool) { return 0, 0, false }
