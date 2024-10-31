//go:build release
// +build release

package main

import "github.com/gin-gonic/gin"

func initGin() {
	gin.SetMode(gin.ReleaseMode)
}
