package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"modbus-tcp-driver-v3/internal/api/handler"
	"modbus-tcp-driver-v3/internal/service"
)

func Router(s *service.MonitorService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	h := handler.New(s)
	api := r.Group("/api")
	{
		api.GET("/health", h.Health)
		api.GET("/config", h.Config)
		api.GET("/io", h.Points)
		api.GET("/io/:name", h.Point)
		api.GET("/io/:name/history", h.History)
		api.GET("/devices", h.Devices)
		api.GET("/events", h.Events)
		api.GET("/alarm-history", h.AlarmHistory)
		api.GET("/alarms", h.Alarms)
		api.GET("/alarm-config", h.AlarmConfig)
		api.POST("/alarm-config", h.UpdateAlarmConfig)
		api.POST("/io/write", h.Write)
	}
	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		// Electron 使用 file:// 加载前端时，Chromium 通常会发送 Origin: null。
		// 同时兼容 Vite 开发环境的 localhost/127.0.0.1 来源。
		if origin == "null" || origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept")
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func RegisterFrontend(r *gin.Engine, indexFile, assetsDir string) {
	if assetsDir != "" {
		r.StaticFS("/assets", http.Dir(assetsDir))
	}
	r.GET("/", func(c *gin.Context) { c.File(indexFile) })
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "api route not found"})
			return
		}
		c.File(indexFile)
	})
}
