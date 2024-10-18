/**
# Copyright (c) Advanced Micro Devices, Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the \"License\");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an \"AS IS\" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package main

import (
	"context"
	"expvar"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
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
	Version   string
	BuildDate string
	GitCommit string
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

func startMetricsServer(c *config.Config) *http.Server {

	serverPort := c.GetServerPort()

	router := mux.NewRouter()
	router.Use(prometheusMiddleware)

	reg := mh.GetRegistry()
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	// pprof
	router.Methods("GET").Subrouter().Handle("/debug/vars", expvar.Handler())
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/", pprof.Index)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/trace", pprof.Trace)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/allocs", pprof.Handler("allocs").ServeHTTP)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/mutex", pprof.Handler("mutex").ServeHTTP)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	router.Methods("GET").Subrouter().HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)

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

func foreverWatcher() {
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
			srvHandler = startMetricsServer(runConf)

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
		agentGrpcPort = flag.Int("agent-grpc-port", globals.GPUAgentPort, "Agent GRPC port")
		versionOpt    = flag.Bool("version", false, "show version")
	)
	flag.Parse()

	if *versionOpt {
		fmt.Printf("Version : %v\n", Version)
		fmt.Printf("BuildDate: %v\n", BuildDate)
		fmt.Printf("GitCommit: %v\n", GitCommit)
		os.Exit(0)
	}

	logger.Log.Printf("Version : %v", Version)
	logger.Log.Printf("BuildDate: %v", BuildDate)
	logger.Log.Printf("GitCommit: %v", GitCommit)

	runConf = config.NewConfig(*metricsConfig)
	runConf.SetAgentPort(*agentGrpcPort)

	mh, _ = metricsutil.NewMetrics(runConf)
	mh.InitConfig()

	// do it only once, keep the same connection no need to reconnect for
	// config changes
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gpuclient, err = gpuagent.NewAgent(ctx, mh)
	if err != nil {
		logger.Log.Fatalf("GPUAgent create failed, %v", err)
		return
	}
	defer func() {
		os.Remove(globals.SlurmSock)
	}()

	foreverWatcher()
}
