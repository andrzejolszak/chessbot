// Package config is used to configure the application
package config

import (
	"github.com/caarlos0/env"
)

// Configuration holds all application configuration
type Configuration struct {
	Port              int    `env:"PORT" envDefault:"8080"`
	Hostname          string `env:"HOSTNAME" envDefault:"localhost:8080"`
	SigningKey        string `env:"SIGNINGKEY"`
	SqlitePath        string `env:"SQLITEPATH"`
	SlackAppID        string `env:"SLACKAPPID"`
	SlackClientID     string `env:"SLACKCLIENTID"`
	SlackClientSecret string `env:"SLACKCLIENTSECRET"`
	SlackSigningKey   string `env:"SLACKSIGNINGKEY"`
}

// ParseConfiguration retrieves values from environment variables and returns a Configuration struct
func ParseConfiguration() (Configuration, error) {
	config := Configuration{}
	if err := env.Parse(&config); err != nil {
		return config, err
	}
	return config, nil
}
