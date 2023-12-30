package main

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

type Video struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

func (v Video) GetID() int {
	return v.ID
}

var gQueue Queue[Video]

func main() {
	SetupLogger()
	gQueue = NewQueue[Video]()

	// cli arguments
	configPath := flag.String("config_path", "./config.yml", "Path to the config yml file")
	flag.Parse()

	config, err := GetConfig(*configPath)
	if err != nil {
		log.Panic(err)
	}

	r := gin.Default()
	r.Use(LoggerMiddleware())
	r.GET("/ping", ping)
	r.GET("/queue", listVideoQueue)
	r.POST("/queue", addVideoToQueue)
	r.DELETE("/queue/:id", delVideoToQueue)

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
	id, err := strconv.Atoi(idS)
	if err != nil {
		c.String(400, err.Error())
	}

	log.WithField("id", id).Debug()
	gQueue.RemoveByID(id)
}

func listVideoQueue(c *gin.Context) {
	c.JSON(200, gQueue.items)
}
