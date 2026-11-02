//go:build !windows && (!darwin || !cgo)

package main

func nativeCompletionCue(alertRequest)        {}
func nativeCompletionInvalidate(uint64)       {}
func nativeCompletionSnapshot() focusSnapshot { return focusSnapshot{} }
func nativeCompletionStart()                  {}
func nativeCompletionEnable()                 {}
func nativeCompletionStop()                   {}
