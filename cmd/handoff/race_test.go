//go:build race

package main

// When the tests run under -race, the CLI they start as subprocesses is built
// with -race too, so the process-level checks run race-instrumented.
func init() { raceBuild = true }
