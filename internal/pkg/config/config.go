package config

import (
	"os"

	"braces.dev/errtrace"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// InitConfig loads optional local defaults and lets runtime environment win.
// Configuration values are never printed, since they include credentials.
func InitConfig[T any]() (*T, error) {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return nil, errtrace.New("load env file failed")
	}

	cfg := new(T)
	if err := envconfig.Process("", cfg); err != nil {
		return nil, errtrace.Wrap(err)
	}

	return cfg, nil
}
