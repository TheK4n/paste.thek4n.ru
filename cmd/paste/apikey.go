package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"
	"github.com/redis/go-redis/v9"

	"github.com/thek4n/paste.thek4n.ru/internal/application/service"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/infrastructure/repository"
)

type apikeysOptions struct {
	DBPort int    `long:"dbport" default:"6379" description:"Database port"`
	DBHost string `long:"dbhost" default:"localhost" description:"Database host"`
}

func apikeysCommand(args []string) {
	var opts apikeysOptions

	args, err := flags.NewParser(&opts, flags.Default).ParseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse params error: %s\n", err)
		os.Exit(2)
	}

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	client := newRedisClientAPIKeys(&opts)

	s := service.NewAPIKeysService(
		repository.NewRedisAPIKeyRORepository(client),
		repository.NewRedisAPIKeyWORepository(client),
	)

	switch args[0] {
	case "list":
		apikeys, err := s.FetchAll()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fail to generate apikey: %s\n", err)
			os.Exit(2)
		}

		fmt.Print(columnT(fmt.Sprintf("%s\n%s", apiKeysListHeader(), printAPIKeys(apikeys))))

	case "gen":
		apikey, err := s.GenerateAPIKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fail to generate apikey\n")
			os.Exit(2)
		}

		fmt.Print(columnT(fmt.Sprintf("Key\tId\tStatus\n%s", formatAPIKeyString(apikey))))

	case "revoke":
		args = args[1:]
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Parse params error: apikey id not provided\n")
			os.Exit(2)
		}

		err := s.InvalidateAPIKey(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fail to invalidate apikey: %s\n", err)
			os.Exit(2)
		}

	case "reauthorize":
		args = args[1:]
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Parse params error: apikey id not provided\n")
			os.Exit(2)
		}

		err := s.ReauthorizeAPIKey(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fail to reauthorize apikey: %s\n", err)
			os.Exit(2)
		}

	case "rm":
		args = args[1:]
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "Parse params error: apikey id not provided\n")
			os.Exit(2)
		}

		err := s.RemoveAPIKey(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fail to remove apikey: %s", err)
			os.Exit(2)
		}

	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func printUsage() {
	usageMessage := `usage: %s apikeys <command> [args]

Commands:
	list          List apikeys
	gen           Generate new apikey
	revoke        Revoke apikey
	reauthorize   Reauthorize revoked apikey
	rm            Reauthorize revoked apikey`

	fmt.Fprintf(os.Stderr, usageMessage, os.Args[0])
}

func printAPIKeys(apikeys []aggregate.APIKey) string {
	var res string
	for n, apikey := range apikeys {
		res = fmt.Sprintf("%s\n%d %s\n", res, n+1, formatAPIKeyString(apikey))
	}

	return res
}

func apiKeysListHeader() string {
	return "№\tKey\tId\tStatus\n"
}

func formatAPIKeyString(apikey aggregate.APIKey) string {
	validString := "✅valid"
	if !apikey.Valid() {
		validString = "❌invalid"
	}
	return fmt.Sprintf("%s\t%s\t%s", apikey.Key(), apikey.PublicID(), validString)
}

func columnT(input string) string {
	if input == "" {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(input), "\n")

	var rows [][]string
	maxCols := 0

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			rows = append(rows, fields)
			if len(fields) > maxCols {
				maxCols = len(fields)
			}
		}
	}

	if maxCols == 0 {
		return ""
	}

	colWidths := make([]int, maxCols)
	for j := 0; j < maxCols; j++ {
		for _, row := range rows {
			if j < len(row) {
				if len(row[j]) > colWidths[j] {
					colWidths[j] = len(row[j])
				}
			}
		}
	}

	var result strings.Builder
	for i, row := range rows {
		for j, field := range row {
			if j > 0 {
				result.WriteString("  ")
			}
			fmt.Fprintf(&result, "%-*s", colWidths[j], field)
		}
		if i < len(rows)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func newRedisClientAPIKeys(opts *apikeysOptions) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", opts.DBHost, opts.DBPort),
		PoolSize:     100,
		Password:     "",
		Username:     "",
		DB:           2,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
}
