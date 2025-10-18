// Enter point to paste service.
package main

import (
	"fmt"
	"os"
)

const UsageMessage = `usage: %s <command>

Commands:
	run       Run paste server.
	apikeys   API keys management.
	ping      Ping command. Can be used for check app health.
`

const (
	SuccessCode = 0
	ErrorCode   = 1
)

func main() {
	oneArg := 2
	if len(os.Args) < oneArg {
		fmt.Fprintf(os.Stderr, UsageMessage, os.Args[0])
		os.Exit(ErrorCode)
	}

	switch os.Args[1] {
	case "run":
		runServer(os.Args[2:])
		os.Exit(SuccessCode)

	case "apikeys":
		apikeysCommand(os.Args[2:])
		os.Exit(SuccessCode)

	case "ping":
		pingCommand(os.Args[2:])
		os.Exit(SuccessCode)

	default:
		fmt.Fprintf(os.Stderr, UsageMessage, os.Args[0])
		os.Exit(ErrorCode)
	}
}
