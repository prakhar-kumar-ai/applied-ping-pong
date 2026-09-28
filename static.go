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

//go:embed frontend/public/bracket-bg.jpg
var bracketBgImage []byte

func registerStaticRoutes(r *gin.Engine) {
	r.GET("/bg.jpg", func(c *gin.Context) {
		c.Data(http.StatusOK, "image/jpeg", bgImage)
	})
	r.GET("/reg-bg.jpg", func(c *gin.Context) {
		c.Data(http.StatusOK, "image/jpeg", regBgImage)
	})
	r.GET("/bracket-bg.jpg", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/jpeg", bracketBgImage)
	})
}
