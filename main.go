package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
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
var sqlite Sqlite

func main() {
	SetupLogger()
	configPath := flag.String("config_path", "./config.yml", "Path to the config yml file")
	flag.Parse()
	config, err := GetConfig(*configPath)
	if err != nil {
		log.Panic(err)
	}

	sqlite = NewSqlite(config.DatabasePath)
	videos, err := sqlite.GetVideos()
	if err != nil {
		log.Panic(err)
	}

	gQueue, err = NewQueue(videos)
	if err != nil {
		log.Panic(err)
	}

	r := gin.Default()
	r.Use(LoggerMiddleware())
	r.GET("/ping", ping)
	r.GET("/queue", listVideoQueue)
	r.POST("/queue", addVideoToQueue)
	r.DELETE("/queue/:id", delVideoToQueue)

	var waitGroup sync.WaitGroup
	ctx, ctxCancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Info(sig, "signal received")
		ctxCancel()
		waitGroup.Wait()
		log.Exit(1)
	}()

	go Dispatcher(ctx, &gQueue, &config, &waitGroup)
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
	_, err := sqlite.InsertVideo(&video)
	if err != nil {
		c.String(400, err.Error())
	}

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
	err = sqlite.DeleteVideoByID(id)
	if err != nil {
		c.String(400, err.Error())
	}

	video, ok := gQueue.RemoveByID(id)
	if !ok {
		c.String(400, "Didn't find video")
	}

	c.JSON(200, video)
}

func listVideoQueue(c *gin.Context) {
	c.JSON(200, gQueue.GetVideos())
}
