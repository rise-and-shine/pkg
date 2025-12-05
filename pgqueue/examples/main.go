package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run . <command>")
		fmt.Println("Commands:")
		fmt.Println("  simple - Run simple usage examples")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "simple":
		simple_usage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Available commands: simple")
	}
}
