package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danthegoodman1/KubeSecretSync/db"
	formattedlogger "github.com/danthegoodman1/KubeSecretSync/formatted_logger"
	"github.com/danthegoodman1/KubeSecretSync/migrations"
	"github.com/danthegoodman1/KubeSecretSync/utils"

	_ "github.com/jackc/pgx/v4/stdlib"
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

	err = migrateUp()
	if err != nil {
		logger.Fatal(err)
	}

	err = initK8sClient()
	if err != nil {
		logger.Fatalf("Error connecting to k8s client: %s", err)
	}

	stopChan := make(chan struct{})

	if utils.LEADER {
		logger.Info("Running as Leader")
		go startLoop(tickLeader, stopChan)
	} else {
		logger.Info("Running as Follower")
		go startLoop(tickFollower, stopChan)
	}
	if err != nil {
		logger.Fatal(err)
	}

	// Listen for shutdown signal
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	<-exit
	logger.Info("Got exit signal, cleaning up")
	if os.Getenv("LOCAL") != "1" {
		stopChan <- struct{}{}
	}
	db.PGPool.Close()
	logger.Info("Exiting")
}

func startLoop(tickFunc func(ctx context.Context) error, stopChan chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.TICK_SECONDS))
	defer cancel()

	if os.Getenv("LOCAL") == "1" {
		// Just run and exit
		logger.Warn("Running locally, triggering tick once then exiting")
		err := tickFunc(ctx)
		if err != nil {
			logger.Error(err)
		}
		return
	}

	ticker := time.NewTicker(time.Second * time.Duration(utils.TICK_SECONDS))

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
			return
		}
	}
}

func migrateUp() error {
	logger.Debug("Migrating up")
	db, err := sql.Open("pgx", utils.DSN)
	if err != nil {
		logger.Error("Failed to open DB for migrations")
		return err
	}
	cnt, err := migrations.RunMigrations(db)
	if err != nil {
		logger.Error("Error running migrations")
		return err
	}
	logger.Debugf("Applied %d migrations", cnt)

	return nil
}
