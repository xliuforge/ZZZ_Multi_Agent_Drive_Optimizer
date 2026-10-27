//go:build !windows

package main

// Scanner launching and startup updates are Windows-only. Keep portable tests
// and source builds available; the in-process installer lock still applies.
func lockScannerInstall(root string) (func(), error) { return func() {}, nil }

func scannerProcessRunning() (bool, error) { return false, nil }
