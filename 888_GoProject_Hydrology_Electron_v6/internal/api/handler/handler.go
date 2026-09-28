package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"modbus-tcp-driver-v3/internal/model"
	"modbus-tcp-driver-v3/internal/service"
)

type writeRequest struct {
	Tag   string `json:"tag" binding:"required"`
	Value any    `json:"value"`
}

type Handler struct {
	service *service.MonitorService
}

func New(s *service.MonitorService) *Handler {
	return &Handler{service: s}
}

func (h *Handler) Health(c *gin.Context) {
	devices := h.service.Devices()
	connected := 0
	for _, d := range devices {
		if d.Connected {
			connected++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status":          "ok",
			"device_count":    len(devices),
			"connected_count": connected,
		},
	})
}

func (h *Handler) Config(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.service.Config()})
}

func (h *Handler) Points(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.service.Points()})
}

func (h *Handler) Point(c *gin.Context) {
	point, ok := h.service.Point(c.Param("name"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "tag not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": point})
}

func (h *Handler) History(c *gin.Context) {
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	if startDate != "" || endDate != "" {
		if startDate == "" || endDate == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "开始日期和结束日期必须同时填写"})
			return
		}
		start, err := time.ParseInLocation("2006-01-02", startDate, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "开始日期格式错误，应为 YYYY-MM-DD"})
			return
		}
		endDay, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "结束日期格式错误，应为 YYYY-MM-DD"})
			return
		}
		end := endDay.Add(24 * time.Hour)
		if !start.Before(end) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "结束日期必须晚于或等于开始日期"})
			return
		}
		if end.Sub(start) > 10*24*time.Hour {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "查询时间跨度过长，最多只能查询10天，请重新选择日期"})
			return
		}
		history, err := h.service.HistoryRange(c.Param("name"), start.UTC(), end.UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": history})
		return
	}

	// Backward-compatible sample-count query used by other monitoring pages.
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "60"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid limit"})
		return
	}
	history, err := h.service.History(c.Param("name"), limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": history})
}

func (h *Handler) Devices(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.service.Devices()})
}

func (h *Handler) Events(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid limit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.service.Events(limit)})
}

func (h *Handler) AlarmHistory(c *gin.Context) {
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "开始日期和结束日期必须同时填写"})
		return
	}
	start, err := time.ParseInLocation("2006-01-02", startDate, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "开始日期格式错误，应为 YYYY-MM-DD"})
		return
	}
	endDay, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "结束日期格式错误，应为 YYYY-MM-DD"})
		return
	}
	end := endDay.Add(24 * time.Hour)
	if !start.Before(end) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "结束日期必须晚于或等于开始日期"})
		return
	}
	items, err := h.service.AlarmHistory(strings.TrimSpace(c.Query("tag")), start.UTC(), end.UTC())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *Handler) Alarms(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.service.Alarms()})
}

func (h *Handler) AlarmConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.service.AlarmConfig()})
}

func (h *Handler) UpdateAlarmConfig(c *gin.Context) {
	var cfg model.AlarmConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.service.UpdateAlarmConfig(cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": h.service.AlarmConfig()})
}

func (h *Handler) Write(c *gin.Context) {
	var req writeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := h.service.Write(c.Request.Context(), req.Tag, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"tag": req.Tag, "message": "write successful"},
	})
}
