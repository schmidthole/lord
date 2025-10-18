package main

import (
	"fmt"
	"os"
)

func printConsoleError(message string, err error) {
	fmt.Println()
	fmt.Printf("\n----- error -----\n%s\n\n----- reason -----\n%s\n\n*if this error is not clear, check the trace above to see which command failed*\n\n", message, err)
	os.Exit(1)
}
