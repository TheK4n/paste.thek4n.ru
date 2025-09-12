// Enter point to paste service.
package main

import (
	"fmt"
	"os"
)

var usageMessage = `usage: %s <command>

Commands:
	run       Run paste server.
	apikeys   API keys management.
	ping      Ping command. Can be used for check app health.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, usageMessage, os.Args[0])
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
		fmt.Fprintf(os.Stderr, usageMessage, os.Args[0])
		os.Exit(1)
	}
}
