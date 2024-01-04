package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"strconv"

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
	// cli arguments
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

	go Dispatcher(context.Background(), &gQueue, config.ProcessFolder, config.RifeBinary, config.Model, config.Workers)
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
