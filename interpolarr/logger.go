package main

import (
	"errors"
	"os"
	"reflect"
	"time"

	"github.com/kjk/common/filerotate"
	"github.com/sirupsen/logrus"
)

// cliHook for logging Info level and above to the CLI.
var logFile *filerotate.File

type cliHook struct{}

func (h *cliHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.InfoLevel, logrus.WarnLevel, logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
}

func (h *cliHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	_, err = os.Stderr.WriteString(line)
	return err
}

func InitLogFile(logPath string) error {
	// Rotating file logger setup
	var err error
	logFile, err = filerotate.NewDaily(logPath, "log.txt", nil)
	return err
}

func CreateLogger(name string) (*logrus.Entry, error) {
	if logFile == nil {
		return nil, errors.New("log file was not initiated")
	}

	// Create logger
	log := logrus.New()

	// Logger configuration
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC1123Z,
	})
	log.SetLevel(logrus.DebugLevel)
	log.SetOutput(logFile)

	// Adding CLI hook
	log.AddHook(&cliHook{})
	return log.WithField("from", name), nil
}

func StructFields(data interface{}) logrus.Fields {
	fields := logrus.Fields{}

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
