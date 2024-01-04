package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

type Video struct {
	ID         int64  `json:"id"`
	Path       string `json:"path"`
	OutputPath string `json:"outPath"`
	Done       bool   `json:"done"`
}

var gQueue Queue

func main() {
	SetupLogger()
	configPath := flag.String("config_path", "./config.yml", "Path to the config yml file")
	flag.Parse()
	config, err := GetConfig(*configPath)
	if err != nil {
		log.Panic(err)
	}

	InitQueuePool(config.DatabasePath)
	gQueue, err = NewQueue()
	if err != nil {
		log.Panic(err)
	}

	r := gin.Default()
	r.Use(LoggerMiddleware())
	r.GET("/ping", ping)
	r.GET("/queue", listVideoQueue)
	r.POST("/queue", addVideoToQueue)
	r.DELETE("/queue/:id", delVideoToQueue)

	ctx, ctxCancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Info(sig, "signal received")
		ctxCancel()
		// TODO: add sync group
		log.Exit(1)
	}()

	go Dispatcher(ctx, &gQueue, config.ProcessFolder, config.RifeBinary, config.Model, config.Workers)
	r.Run(fmt.Sprintf("%s:%d", config.BindAddress, config.Port))
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "ping",
	})
}

func addVideoToQueue(c *gin.Context) {
	var video Video

	if err := c.ShouldBind(&video); err != nil {
		c.String(400, err.Error())
	}

	log.WithFields(StructFields(video)).Debug()
	gQueue.Enqueue(video)

	c.String(200, "Success")
}

func delVideoToQueue(c *gin.Context) {
	idS := c.Param("id")
	id, err := strconv.ParseInt(idS, 10, 64)
	if err != nil {
		c.String(400, err.Error())
	}

	log.WithField("id", id).Debug()
	gQueue.RemoveByID(id)
}

func listVideoQueue(c *gin.Context) {
	c.JSON(200, gQueue.items)
}
