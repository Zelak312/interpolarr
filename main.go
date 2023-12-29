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
	r.GET("/ping", ping)

	r.Run(fmt.Sprintf("%s:%d", config.BindAddress, config.Port))
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "ping",
	})
}
