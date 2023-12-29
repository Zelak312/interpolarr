package main

import (
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// cliHook for logging Info level and above to the CLI.
type cliHook struct{}

func (h *cliHook) Levels() []log.Level {
	return []log.Level{log.InfoLevel, log.WarnLevel, log.ErrorLevel, log.FatalLevel, log.PanicLevel}
}

func (h *cliHook) Fire(entry *log.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	_, err = os.Stderr.WriteString(line)
	return err
}

func SetupLogger() {
	// Rotating file logger setup
	lumberjackLogger := &lumberjack.Logger{
		Filename:   filepath.ToSlash("./logs/current_log.log"),
		MaxSize:    5, // in MB
		MaxBackups: 10,
		MaxAge:     30,   // in days
		Compress:   true, // compress old log files
	}

	// Logger configuration
	log.SetFormatter(&log.JSONFormatter{
		TimestampFormat: time.RFC1123Z,
	})
	log.SetLevel(log.DebugLevel)
	log.SetOutput(lumberjackLogger)

	// Adding CLI hook
	log.AddHook(&cliHook{})
}
