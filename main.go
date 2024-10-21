//go:generate templ generate ./views
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Video struct {
	ID         int64  `json:"id"`
	Path       string `json:"path"`
	OutputPath string `json:"outPath"`
}

var gQueue Queue
var poolWorker *PoolWorker
var hub *Hub
var sqlite Sqlite

var log *logrus.Entry

func setupLoggers(config *Config) {
	err := InitLogFile(config.LogPath)
	if err != nil {
		panic("Couldn't init log file: " + err.Error())
	}

	log, err = CreateLogger("server")
	if err != nil {
		panic("Couldn't create logger server")
	}
}

//go:embed views/*
var viewFiles embed.FS

func main() {
	configPath := flag.String("config_path", "./config.yml", "Path to the config yml file")
	flag.Parse()
	config, err := GetConfig(*configPath)
	if err != nil {
		panic("Error get config: " + err.Error())
	}

	setupLoggers(&config)
	log.WithFields(StructFields(config)).Debug("Parsed config")
	if *config.DeleteInputFileWhenFinished {
		log.Warn("DeleteInputFileWhenFinished is ON, it will delete the input file when finished!!!")
	}

	sqlite = NewSqlite(config.DatabasePath)
	sqlite.RunMigrations()

	videos, err := sqlite.GetVideos()
	if err != nil {
		log.Panic("Error getting videos: ", err)
	}

	gQueue, err = NewQueue(videos)
	if err != nil {
		log.Panic("Error creating the queue: ", err)
	}

	hub, err = NewHub()
	if err != nil {
		log.Panic("error creating the hub: ", err)
	}

	go hub.Run()
	r := gin.Default()
	r.Use(LoggerMiddleware())

	// UI
	r.Use(static.Serve("/", static.EmbedFolder(viewFiles, "views")))

	// API
	api := r.Group("/api")
	{
		api.GET("/ping", ping)

		api.GET("/queue", listVideoQueue)
		api.POST("/queue", addVideoToQueue)
		api.DELETE("/queue/:id", delVideoToQueue)

		api.GET("/workers", listWorkers)

		api.GET("/ws", func(c *gin.Context) {
			hub.HandleConnections(c)
		})
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	poolWorker = NewPoolWorker(ctx, &gQueue, &config)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Info("Signal received: ", sig, " shuting down")
		ctxCancel()

		timer := time.NewTimer(time.Second * 30)
		go func() {
			<-timer.C
			log.Info("Taking too long to shutdown, exiting forcefully")
			os.Exit(1)
		}()

		poolWorker.waitGroup.Wait()
		os.Exit(1)
	}()

	// Start running things
	go poolWorker.RunDispatcherBlocking()
	err = r.Run(fmt.Sprintf("%s:%d", config.BindAddress, config.Port))
	if err != nil {
		log.Panic("Error running web server: ", err)
	}
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
		return
	}

	videoExist, err := FileExist(video.Path)
	if err != nil {
		c.String(400, err.Error())
		return
	}

	if !videoExist {
		c.String(400, "video source not found")
		return
	}

	// TODO: I want to do something to check if the output path
	// is somewhat valid, but I also want it so that my app
	// can construct subpath to a video that may not exist yet
	// example:
	// show1/episode1 I want it to not error if show1 doesn't exist
	// since it could create it
	// videoOutDirExist, err := FileExist(video.OutputPath)
	// if err != nil {
	// 	c.String(400, err.Error())
	// 	return
	// }

	// if !videoOutDirExist {
	// 	c.String(400, "video Output path not found")
	// 	return
	// }

	log.WithFields(StructFields(video)).Debug("Adding video to queue")

	_, err = sqlite.InsertVideo(&video)
	if err != nil {
		log.WithFields(StructFields(video)).Error("Error inserting the video: ", err)
		c.String(400, err.Error())
		return
	}

	gQueue.Enqueue(video)
	log.WithFields(StructFields(video)).Info("Sucessfully video to queue")
	c.String(200, "Success")
}

func delVideoToQueue(c *gin.Context) {
	idS := c.Param("id")
	id, err := strconv.ParseInt(idS, 10, 64)
	if err != nil {
		c.String(400, err.Error())
		return
	}

	log.WithField("id", id).Debug("Deleting video by id")
	err = sqlite.DeleteVideoByID(nil, id)
	if err != nil {
		c.String(400, err.Error())
		return
	}

	video, ok := gQueue.RemoveByID(id)
	if !ok {
		log.WithField("id", id).Error("Failed to delete the video by id: ", err)
		c.String(400, "Didn't find video")
		return
	}

	log.WithField("id", id).Info("Sucessfully delete video by id")
	c.JSON(200, video)
}

func listVideoQueue(c *gin.Context) {
	log.Debug("Getting video queue")
	c.JSON(200, gQueue.GetVideos())
}

func listWorkers(c *gin.Context) {
	log.Debug("Getting worker list")
	c.JSON(200, &poolWorker.workers)
}
