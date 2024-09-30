//
// Copyright(C) Advanced Micro Devices, Inc. All rights reserved.
//
// You may not use this software and documentation (if any) (collectively,
// the "Materials") except in compliance with the terms and conditions of
// the Software License Agreement included with the Materials or otherwise as
// set forth in writing and signed by you and an authorized signatory of AMD.
// If you do not have a copy of the Software License Agreement, contact your
// AMD representative for a copy.
//
// You agree that you will not reverse engineer or decompile the Materials,
// in whole or in part, except as allowed by applicable law.
//
// THE MATERIALS ARE DISTRIBUTED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OR
// REPRESENTATIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED.
//

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/pensando/device-metrics-exporter/internal/amdgpu/config"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gpuagent"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/metricsutil"
)

// single instance handlers
var (
	mh        *metricsutil.MetricsHandler
	gpuclient *gpuagent.GPUAgentClient
	runConf   *config.Config
)

// get the info from gpu agent and update the current metrics registery
func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = mh.UpdateMetrics()
		next.ServeHTTP(w, r)
	})
}

func start_metrics_server(c *config.Config) *http.Server {

	serverPort := c.GetServerPort()

	router := mux.NewRouter()
	router.Use(prometheusMiddleware)

	reg := mh.GetRegistry()
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%v", serverPort),
		Handler: router,
	}
	go func() {
		logger.Log.Printf("serving requests on port %v", serverPort)
		err := srv.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
		logger.Log.Printf("server on port %v shutdown gracefully", serverPort)
	}()

	return srv
}

func forever_watcher() {
	var srvHandler *http.Server
	configPath := runConf.GetMetricsConfigPath()

	pollTimer := time.NewTicker(5 * time.Second)
	defer pollTimer.Stop()

	serverRunning := func() bool {
		return srvHandler != nil
	}

	startServer := func() {
		if !serverRunning() {
			mh.InitConfig()
			serverPort := runConf.GetServerPort()
			logger.Log.Printf("starting server on %v", serverPort)
			srvHandler = start_metrics_server(runConf)

		}
	}
	stopServer := func() {
		if serverRunning() {
			logger.Log.Printf("stopping server")
			srvCtx, srvCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := srvHandler.Shutdown(srvCtx); err != nil {
				panic(err) // failure/timeout shutting down the server gracefully
			}
			srvCancel()
			time.Sleep(1 * time.Second)
			srvHandler = nil
		}
	}

	lastChangedTime := func() time.Time {
		fileInfo, err := os.Stat(configPath)
		if err != nil {
			return time.Now()
		}
		return fileInfo.ModTime()
	}()

	fileChanged := func() bool {
		fileInfo, err := os.Stat(configPath)
		if err != nil {
			// error not be logged as this is in a timer loop
			// ignore error
			return false
		}
		modifiedTime := fileInfo.ModTime()
		if modifiedTime != lastChangedTime {
			lastChangedTime = modifiedTime
			return true
		}
		return false
	}

	logger.Log.Printf("starting file watcher")
	startServer()

	// Start listening for events.
	go func() {
		for range pollTimer.C {
			if fileChanged() {
				logger.Log.Printf("config update detected, loading new config")
				// stop server if running
				stopServer()
				// start server
				startServer()
			}
		}
	}()

	<-make(chan struct{})
}

func main() {
	logger.Init()
	var err error
	var (
		metricsConfig = flag.String("amd-metrics-config", globals.AMDMetricsFile, "AMD metrics exporter config file")
	)
	flag.Parse()

	runConf = config.NewConfig(*metricsConfig)

	mh, _ = metricsutil.NewMetrics(runConf)
	mh.InitConfig()

	// do it only once, keep the same connection no need to reconnect for
	// config changes
	gpuclient, err = gpuagent.NewAgent(mh)
	if err != nil {
		logger.Log.Fatalf("GPUAgent create failed")
		return
	}

	forever_watcher()
}
