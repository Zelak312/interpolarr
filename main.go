package main

import (
	"flag"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

func main() {
	SetupLogger()

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
	r.POST("/queue", addVideoToQueue)

	r.Run(fmt.Sprintf("%s:%d", config.BindAddress, config.Port))
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "ping",
	})
}

type Video struct {
	Path string `json:"path"`
}

func addVideoToQueue(c *gin.Context) {
	var video Video

	if err := c.ShouldBind(&video); err != nil {
		c.String(400, err.Error())
	}

	log.WithFields(StructFields(video)).Debug()

	c.String(200, "Success")
}
