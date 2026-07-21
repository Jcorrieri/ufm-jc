package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config contains the deployment-specific values required by the application.
type Config struct {
	ServerAddress     string
	DatabasePath      string
	JWTSecret         string
	SessionCookieName string
}

// Load reads configuration from the environment after optionally loading a local .env file.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	configuration := Config{
		ServerAddress:     strings.TrimSpace(os.Getenv("SERVER_ADDRESS")),
		DatabasePath:      strings.TrimSpace(os.Getenv("DATABASE_PATH")),
		JWTSecret:         strings.TrimSpace(os.Getenv("JWT_SECRET")),
		SessionCookieName: strings.TrimSpace(os.Getenv("SESSION_COOKIE_NAME")),
	}

	if err := configuration.validate(); err != nil {
		return Config{}, err
	}

	return configuration, nil
}

func (configuration Config) validate() error {
	requiredValues := []struct {
		name  string
		value string
	}{
		{name: "SERVER_ADDRESS", value: configuration.ServerAddress},
		{name: "DATABASE_PATH", value: configuration.DatabasePath},
		{name: "JWT_SECRET", value: configuration.JWTSecret},
		{name: "SESSION_COOKIE_NAME", value: configuration.SessionCookieName},
	}

	for _, requiredValue := range requiredValues {
		if requiredValue.value == "" {
			return fmt.Errorf("%s must not be empty", requiredValue.name)
		}
	}

	_, port, err := net.SplitHostPort(configuration.ServerAddress)
	if err != nil {
		return fmt.Errorf("SERVER_ADDRESS must use host:port format: %w", err)
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("SERVER_ADDRESS contains invalid port %q", port)
	}

	return nil
}
