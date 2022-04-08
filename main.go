package main

import formattedlogger "github.com/danthegoodman1/KubeSecretSync/formatted_logger"

var (
	logger = formattedlogger.NewLogger()
)

func main() {
	logger.Info("Starting KubeSecretSync")
}
