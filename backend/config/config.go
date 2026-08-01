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
	ServerAddress       string
	DatabasePath        string
	JWTSecret           string
	SessionCookieName   string
	ObjectStoreProvider string
	S3Bucket            string
	AWSRegion           string
}

// Load reads configuration from the environment after optionally loading a local .env file.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	configuration := Config{
		ServerAddress:       strings.TrimSpace(os.Getenv("SERVER_ADDRESS")),
		DatabasePath:        strings.TrimSpace(os.Getenv("DATABASE_PATH")),
		JWTSecret:           strings.TrimSpace(os.Getenv("JWT_SECRET")),
		SessionCookieName:   strings.TrimSpace(os.Getenv("SESSION_COOKIE_NAME")),
		ObjectStoreProvider: strings.TrimSpace(os.Getenv("OBJECT_STORE_PROVIDER")),
		S3Bucket:            strings.TrimSpace(os.Getenv("S3_BUCKET")),
		AWSRegion:           strings.TrimSpace(os.Getenv("AWS_REGION")),
	}
	if configuration.ObjectStoreProvider == "" {
		configuration.ObjectStoreProvider = "unavailable"
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

	switch configuration.ObjectStoreProvider {
	case "unavailable":
	case "s3":
		if configuration.S3Bucket == "" {
			return fmt.Errorf("S3_BUCKET must not be empty when OBJECT_STORE_PROVIDER=s3")
		}
		if configuration.AWSRegion == "" {
			return fmt.Errorf("AWS_REGION must not be empty when OBJECT_STORE_PROVIDER=s3")
		}
	default:
		return fmt.Errorf(
			"OBJECT_STORE_PROVIDER must be unavailable or s3, got %q",
			configuration.ObjectStoreProvider,
		)
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
