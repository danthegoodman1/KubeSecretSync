package utils

import (
	"os"
	"strconv"

	formattedlogger "github.com/danthegoodman1/KubeSecretSync/formatted_logger"
)

var (
	logger = formattedlogger.NewLogger()

	LEADER = os.Getenv("LEADER") == "1"

	ENCRYPTION_KEY = GetEnvOrFail("ENCRYPTION_KEY")

	DSN = GetEnvOrFail("DSN")

	TICK_SECONDS = GetEnvOrDefaultInt("TICK_SECONDS", 20)
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
		logger.Fatalf("Failed to find env var '%s'", env)
		return ""
	} else {
		return e
	}
}

func GetEnvOrDefaultInt(env string, defaultVal int) int {
	e := os.Getenv(env)
	if e == "" {
		return defaultVal
	} else {
		// Try to cast to int
		result, err := strconv.Atoi(e)
		if err != nil {
			logger.Fatalf("Failed to parse %s to int of value %s: %s", env, e, err.Error())
		}
		return result
	}
}
