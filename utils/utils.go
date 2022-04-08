package utils

import (
	"fmt"
	"os"

	formattedlogger "github.com/danthegoodman1/KubeSecretSync/formatted_logger"
)

var (
	logger = formattedlogger.NewLogger()

	LEADER = os.Getenv("LEADER") == "1"

	ENCRYPTION_KEY = GetEnvOrFail("ENCRYPTION_KEY")

	DSN = GetEnvOrFail("DSN")
)

func GetEnvOrDefault(env, defaultVal string) string {
	e := os.Getenv(env)
	if e == "" {
		return defaultVal
	} else {
		return e
	}
}

func GetEnvOrFail(env string) string {
	e := os.Getenv(env)
	if e == "" {
		logger.Error(fmt.Sprintf("Failed to find env var '%s'", env))
		os.Exit(1)
		return ""
	} else {
		return e
	}
}
