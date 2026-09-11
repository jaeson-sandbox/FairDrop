//go:build !darwin

package main

func nativeSingleInstanceLockUsable() bool { return true }
