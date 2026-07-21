package config_test

import (
	"strings"
	"testing"

	"github.com/Jcorrieri/uf-marketplace/backend/config"
)

var validEnvironment = map[string]string{
	"SERVER_ADDRESS":      "localhost:8080",
	"DATABASE_PATH":       "marketplace.db",
	"JWT_SECRET":          "test-secret",
	"SESSION_COOKIE_NAME": "session_token",
}

func TestLoad(t *testing.T) {
	setEnvironment(t, validEnvironment)

	configuration, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned an error: %v", err)
	}

	if configuration.ServerAddress != validEnvironment["SERVER_ADDRESS"] {
		t.Errorf("ServerAddress = %q", configuration.ServerAddress)
	}
	if configuration.DatabasePath != validEnvironment["DATABASE_PATH"] {
		t.Errorf("DatabasePath = %q", configuration.DatabasePath)
	}
	if configuration.JWTSecret != validEnvironment["JWT_SECRET"] {
		t.Errorf("JWTSecret = %q", configuration.JWTSecret)
	}
	if configuration.SessionCookieName != validEnvironment["SESSION_COOKIE_NAME"] {
		t.Errorf("SessionCookieName = %q", configuration.SessionCookieName)
	}
}

func TestLoadRejectsMissingOrBlankValues(t *testing.T) {
	for variableName := range validEnvironment {
		t.Run(variableName, func(t *testing.T) {
			values := copyEnvironment(validEnvironment)
			values[variableName] = "  "
			setEnvironment(t, values)

			_, err := config.Load()
			if err == nil || !strings.Contains(err.Error(), variableName) {
				t.Fatalf("Load() error = %v, want error containing %s", err, variableName)
			}
		})
	}
}

func TestLoadRejectsMalformedServerAddress(t *testing.T) {
	testCases := []string{"localhost", "localhost:not-a-port", "localhost:0", "localhost:65536"}

	for _, serverAddress := range testCases {
		t.Run(serverAddress, func(t *testing.T) {
			values := copyEnvironment(validEnvironment)
			values["SERVER_ADDRESS"] = serverAddress
			setEnvironment(t, values)

			_, err := config.Load()
			if err == nil || !strings.Contains(err.Error(), "SERVER_ADDRESS") {
				t.Fatalf("Load() error = %v, want SERVER_ADDRESS error", err)
			}
		})
	}
}

func setEnvironment(t *testing.T, values map[string]string) {
	t.Helper()
	for variableName, value := range values {
		t.Setenv(variableName, value)
	}
}

func copyEnvironment(environment map[string]string) map[string]string {
	copy := make(map[string]string, len(environment))
	for variableName, value := range environment {
		copy[variableName] = value
	}
	return copy
}
