package main

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
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

func SetupLogger(logPath string) {
	// Rotating file logger setup
	lumberjackLogger := &lumberjack.Logger{
		Filename:   filepath.ToSlash(path.Join(logPath, "/current_log.log")),
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

func StructFields(data interface{}) log.Fields {
	fields := log.Fields{}

	// Use reflection to iterate through the struct's fields and add them to the fields map
	val := reflect.ValueOf(data)
	typ := reflect.TypeOf(data)

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	for i := 0; i < val.NumField(); i++ {
		fieldName := typ.Field(i).Name
		fieldValue := val.Field(i).Interface()
		fields[fieldName] = fieldValue
	}

	return fields
}
