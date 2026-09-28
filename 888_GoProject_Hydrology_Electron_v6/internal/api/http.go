package api

//
//import (
//	"net/http"
//	"strings"
//
//	"github.com/gin-gonic/gin"
//
//	"modbus-tcp-driver-v3/internal/io"
//	"modbus-tcp-driver-v3/internal/modbus"
//	"modbus-tcp-driver-v3/internal/model"
//)
//
//type writeRequest struct {
//	Tag   string `json:"tag" binding:"required"`
//	Value any    `json:"value"`
//}
//
//// Router 创建 Gin HTTP 路由。
////
//// API：
////
////	GET  /api/health
////	GET  /api/config
////	GET  /api/io
////	GET  /api/io/:name
////	POST /api/io/write
//func Router(ioManager *io.Manager, driver *modbus.Driver, cfg model.Config) *gin.Engine {
//	r := gin.New()
//	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())
//
//	apiGroup := r.Group("/api")
//	{
//		apiGroup.GET("/health", func(c *gin.Context) {
//			c.JSON(http.StatusOK, gin.H{
//				"success": true,
//				"data": gin.H{
//					"status": "ok",
//				},
//			})
//		})
//
//		apiGroup.GET("/config", func(c *gin.Context) {
//			c.JSON(http.StatusOK, cfg)
//		})
//
//		apiGroup.GET("/io", func(c *gin.Context) {
//			c.JSON(http.StatusOK, ioManager.All())
//		})
//
//		apiGroup.GET("/io/:name", func(c *gin.Context) {
//			name := c.Param("name")
//			point, ok := ioManager.Get(name)
//			if !ok {
//				c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
//				return
//			}
//			c.JSON(http.StatusOK, point)
//		})
//
//		apiGroup.POST("/io/write", func(c *gin.Context) {
//			var req writeRequest
//			if err := c.ShouldBindJSON(&req); err != nil {
//				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
//				return
//			}
//
//			if strings.TrimSpace(req.Tag) == "" {
//				c.JSON(http.StatusBadRequest, gin.H{"error": "tag is required"})
//				return
//			}
//
//			if err := driver.Write(c.Request.Context(), req.Tag, req.Value); err != nil {
//				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
//				return
//			}
//
//			c.JSON(http.StatusOK, gin.H{
//				"success": true,
//				"data": gin.H{
//					"tag": req.Tag,
//				},
//			})
//		})
//	}
//
//	return r
//}
//
//// corsMiddleware 允许 Vue/Vite 开发服务器访问 Go API。
//// 生产环境如果前后端同端口提供服务，不需要依赖 CORS，但保留该中间件可以兼容开发环境。
//func corsMiddleware() gin.HandlerFunc {
//	return func(c *gin.Context) {
//		origin := c.GetHeader("Origin")
//		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
//			c.Header("Access-Control-Allow-Origin", origin)
//			c.Header("Vary", "Origin")
//			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept")
//			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
//		}
//
//		if c.Request.Method == http.MethodOptions {
//			c.AbortWithStatus(http.StatusNoContent)
//			return
//		}
//
//		c.Next()
//	}
//}
