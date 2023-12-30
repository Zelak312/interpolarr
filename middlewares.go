package main

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request, then log
		c.Next()

		// Log example
		log.WithFields(log.Fields{
			"status":  c.Writer.Status(),
			"method":  c.Request.Method,
			"path":    c.Request.URL.Path,
			"ip":      c.ClientIP(),
			"latency": c.Writer.Header().Get("X-Response-Time"),
		}).Debug()
	}
}
