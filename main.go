package main

import (
	"context"

	"github.com/danthegoodman1/KubeSecretSync/db"
	formattedlogger "github.com/danthegoodman1/KubeSecretSync/formatted_logger"
	"github.com/danthegoodman1/KubeSecretSync/utils"
)

var (
	logger = formattedlogger.NewLogger()
)

func main() {
	logger.Info("Starting KubeSecretSync")

	err := db.ConnectToDB()
	if err != nil {
		logger.Fatalf("Error connecting to DB: %s", err)
	}

	err = initK8sClient()
	if err != nil {
		logger.Fatalf("Error connecting to k8s client: %s", err)
	}

	if utils.LEADER {
		logger.Info("Running as Leader")
		err = tickLeader(context.Background())
		if err != nil {
			logger.Fatalf("Error ticking leader: %s", err.Error())
		}
	} else {
		logger.Info("Running as Follower")
	}
}
