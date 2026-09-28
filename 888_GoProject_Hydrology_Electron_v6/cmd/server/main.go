package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"modbus-tcp-driver-v3/internal/api"
	"modbus-tcp-driver-v3/internal/config"
	"modbus-tcp-driver-v3/internal/driver"
	"modbus-tcp-driver-v3/internal/history"
	"modbus-tcp-driver-v3/internal/io"
	"modbus-tcp-driver-v3/internal/modbus"
	"modbus-tcp-driver-v3/internal/model"
	"modbus-tcp-driver-v3/internal/opcda"
	"modbus-tcp-driver-v3/internal/service"
)

func main() {
	modbusCfg, err := config.LoadJSON("configs/modbus.json")
	if err != nil {
		log.Fatal(err)
	}
	opcCfg, err := config.LoadJSON("configs/opcda.json")
	if err != nil {
		log.Fatal(err)
	}
	cfg := modbusCfg
	cfg.ModbusEnabled = modbusCfg.ModbusEnabled
	cfg.ModbusEnabledSet = modbusCfg.ModbusEnabledSet
	cfg.OPCDAEnabled = opcCfg.OPCDAEnabled
	cfg.OPCDAEnabledSet = opcCfg.OPCDAEnabledSet
	cfg.OPCDADevices = opcCfg.OPCDADevices
	if err := config.LoadTagsCSV("configs/Tags.csv", &cfg); err != nil {
		log.Fatal(err)
	}
	historyCfg, err := config.LoadHistoryJSON("configs/history.json")
	if err != nil {
		log.Fatal(err)
	}
	cfg.History = historyCfg
	alarmCfg, err := config.LoadAlarmCSV("configs/Alarm.csv", cfg)
	if err != nil {
		log.Fatal(err)
	}
	cfg.Alarm = alarmCfg
	if err := config.ValidateConfig(cfg); err != nil {
		log.Fatal(err)
	}

	historyStore, err := history.OpenWithConfig(filepath.Join("data", "history.db"), historyCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer historyStore.Close()
	log.Printf("history database ready: data/history.db; interval=%dmin retention=%dd archive=%t query_max_points=%d",
		historyCfg.SampleIntervalMinutes, historyCfg.RetentionDays, historyCfg.ArchiveEnabled, historyCfg.QueryMaxPoints)

	ioManager := io.NewManager()
	definitions := make([]io.PointDefinition, 0, 64)
	for _, d := range cfg.Devices {
		for _, t := range d.Tags {
			definitions = append(definitions, io.PointDefinition{Name: t.Name, DataType: t.DataType, Description: t.Description, History: t.History})
		}
	}
	for _, d := range cfg.OPCDADevices {
		for _, t := range d.Tags {
			definitions = append(definitions, io.PointDefinition{Name: t.Name, DataType: t.DataType, Description: t.Description, History: t.History})
		}
	}
	ioManager.RegisterPointDefinitions(definitions)
	ioManager.SetHistoryStore(historyStore)
	ioManager.SetHistoryConfig(config.HistoryFlags(cfg))
	ioManager.SetHistoryPolicy(historyCfg.Enabled, time.Duration(historyCfg.SampleIntervalMinutes)*time.Minute, historyCfg.QueryMaxPoints)
	ioManager.SetAlarmConfig(alarmCfg)
	var modbusDriver *modbus.Driver
	if cfg.IsModbusEnabled() {
		modbusDriver, err = modbus.NewDriver(cfg, ioManager)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("Modbus TCP driver disabled by configs/modbus.json")
	}
	var opcdaDriver *opcda.Driver
	if cfg.IsOPCDAEnabled() {
		opcdaDriver, err = opcda.NewDriver(cfg, ioManager)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("OPC DA driver disabled by configs/opcda.json")
	}

	portsList := make([]driver.Port, 0, 2)
	if modbusDriver != nil {
		portsList = append(portsList, modbusDriver)
	}
	if opcdaDriver != nil {
		portsList = append(portsList, opcdaDriver)
	}
	ports := driver.NewComposite(portsList...)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go historyStore.StartMaintenance(ctx)
	if modbusDriver != nil {
		go modbusDriver.Start(ctx)
	}
	if opcdaDriver != nil {
		go opcdaDriver.Start(ctx)
	}

	monitorService := service.NewMonitorService(cfg, ioManager, ports)
	monitorService.SetAlarmSaver(func(alarmCfg model.AlarmConfig) error {
		return config.SaveAlarmCSV("configs/Alarm.csv", alarmCfg)
	})
	r := api.Router(monitorService)
	registerFrontend(r)
	server := &http.Server{Addr: ":8080", Handler: r, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Println("Gin HTTP server listening on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func registerFrontend(r *gin.Engine) {
	staticDir := filepath.Join("frontend", "dist")
	indexFile := filepath.Join(staticDir, "index.html")
	if _, err := os.Stat(indexFile); err != nil {
		log.Printf("Vue build not found: %s; Gin will serve API only", indexFile)
		return
	}
	api.RegisterFrontend(r, indexFile, filepath.Join(staticDir, "assets"))
}
