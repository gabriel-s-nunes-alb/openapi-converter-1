// Package config provides configuration loading for the OpenAPI converter.
package config

import (
	configloader "github.com/GabrielNunesIT/go-libs/config-loader"
)

// Config holds the application configuration.
// This is a placeholder for future configuration options.
type Config struct {
	// Add configuration fields here as needed
	// Example:
	// OutputDir string `koanf:"output_dir"`
	// LogLevel  string `koanf:"log_level"`
}

// Load returns the application configuration using go-libs config-loader.
func Load() (*Config, error) {
	defaults := Config{}

	loader := configloader.NewConfigLoader(
		configloader.WithDefaults(defaults),
		// Future: Add file, env, flags support
		// configloader.WithFile[Config]("config.yaml"),
		// configloader.WithEnv[Config]("OPENAPI_"),
	)

	cfg, err := loader.Load()
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
