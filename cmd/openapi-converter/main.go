// Package main provides the entry point for the OpenAPI converter CLI.
package main

import (
	"os"

	"github.com/GabrielNunesIT/go-libs/logger"
	"github.com/GabrielNunesIT/openapi-converter/internal/cli"
)

func main() {
	log := logger.NewConsoleLogger(os.Stdout)

	app := cli.New(log)
	if err := app.Execute(); err != nil {
		log.Errorf("Error: %v", err)
		os.Exit(1)
	}
}
