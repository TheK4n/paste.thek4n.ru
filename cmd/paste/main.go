// Enter point to paste service.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "run/apikeys/ping")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runServer(os.Args[2:])
		os.Exit(0)

	case "apikeys":
		apikeysCommand(os.Args[2:])
		fmt.Println("apikeys")

	case "ping":
		pingCommand(os.Args[2:])
		os.Exit(0)

	default:
		fmt.Fprintln(os.Stderr, "run/apikeys/ping")
		os.Exit(1)
	}
}
