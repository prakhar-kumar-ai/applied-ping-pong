package main

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed frontend/public/bg.jpg
var bgImage []byte

//go:embed frontend/public/reg-bg.jpg
var regBgImage []byte

func registerStaticRoutes(r *gin.Engine) {
	r.GET("/bg.jpg", func(c *gin.Context) {
		c.Data(http.StatusOK, "image/jpeg", bgImage)
	})
	r.GET("/reg-bg.jpg", func(c *gin.Context) {
		c.Data(http.StatusOK, "image/jpeg", regBgImage)
	})
}
