package formattedlogger

import (
	"os"
	"runtime"

	"github.com/sirupsen/logrus"
)

type UTCFormatter struct {
	logrus.Formatter
}

func (u UTCFormatter) Format(e *logrus.Entry) ([]byte, error) {
	e.Time = e.Time.UTC()
	return u.Formatter.Format(e)
}

func NewLogger() *logrus.Logger {
	logger := logrus.New()
	if os.Getenv("DEBUG") == "1" {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	if os.Getenv("PRETTY") != "1" {
		logger.SetFormatter(UTCFormatter{&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z",
			CallerPrettyfier: func(f *runtime.Frame) (function string, file string) {
				return f.Function, ""
			},
		}})
	}
	logger.SetReportCaller(true)
	return logger
}
