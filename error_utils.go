package main

import "fmt"

func printConsoleError(message string, err error) {
	fmt.Println()
	fmt.Printf("\n----- error -----\n%s\nreason: %s\n\n*if this error is not clear, check the trace above to see which command failed*\n", message, err)
}
