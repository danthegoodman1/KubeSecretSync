package main

import (
	"context"
	"time"

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

	// stopChan := make(chan struct{})

	if utils.LEADER {
		logger.Info("Running as Leader")
		err = tickLeader(context.Background())
		// startLoop(tickLeader, stopChan)
	} else {
		logger.Info("Running as Follower")
		err = tickFollower(context.Background())
		// startLoop(tickFollower, stopChan)
	}
	if err != nil {
		logger.Fatal(err)
	}
}

func startLoop(tickFunc func(ctx context.Context) error, stopChan chan struct{}) (returnChan chan struct{}) {
	ticker := time.NewTicker(time.Second * time.Duration(utils.TICK_SECONDS))
	ctx, cancel := context.WithCancel(context.Background())

	for {
		select {
		case <-ticker.C:
			s := time.Now()
			err := tickFunc(ctx)
			if err != nil {
				logger.Error(err)
			} else {
				logger.Debugf("Ticked in %s", time.Since(s))
			}
		case <-stopChan:
			logger.Info("Received on stop channel, shutting down")
			cancel()
			return
		}
	}
}
