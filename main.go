package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// cli arguments
	configPath := flag.String("config_path", "./config.yml", "Path to the config yml file")
	flag.Parse()

	config, err := GetConfig(*configPath)
	if err != nil {
		panic(err)
	}

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "ping",
		})
	})

	r.Run(fmt.Sprintf("%s:%d", config.BindAddress, config.Port))
}
