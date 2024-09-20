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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/gpumetrics"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gpuagent"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/metricsutil"
)

const (
	amdListenPort  = "5000"
	amdMetricsFile = "/etc/metrics/config.json"
)

var (
	mh        *metricsutil.MetricsHandler
	gpuclient *gpuagent.GPUAgentClient
)

// get the info from gpu agent and update the current metrics registery
func updateGPUInfo() error {
	// send the req to gpuclient
	res, err := gpuclient.GPUGet()
	if err != nil {
		logger.Log.Printf("err :%v", err)
		return err
	}
	if res != nil && res.ApiStatus != 0 {
		logger.Log.Printf("resp status :%v", res.ApiStatus)
		return fmt.Errorf("%v", res.ApiStatus)
	}
	mh.UpdateGPUInfoToMetrics(res)
	return nil
}

func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		updateGPUInfo()
		next.ServeHTTP(w, r)
	})
}

func start_metrics_server(serverPort string, configPath string) {
	var config_fields gpumetrics.MetricConfig
	pconfig_fields := &config_fields
	fields, err := ioutil.ReadFile(configPath)
	if err != nil {
		pconfig_fields = nil
	} else {
		_ = json.Unmarshal(fields, pconfig_fields)
		logger.Log.Printf("fields : %+v", pconfig_fields)
	}

	mh, _ = metricsutil.NewMetrics(pconfig_fields)
	res, err := gpuclient.GPUGet()
	if err != nil {
		logger.Log.Printf("err :%v", err)
		return
	}
	for i, gpu := range res.Response {
		logger.Log.Printf("GPU[%v].Status :%+v", i, gpu.Status)
	}
	mh.InitStaticMetrics(res)
	mh.UpdateGPUInfoToMetrics(res)

	router := mux.NewRouter()
	router.Use(prometheusMiddleware)

	reg := mh.GetRegistry()
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	logger.Log.Printf("Serving requests on port %v", serverPort)
	err = http.ListenAndServe(fmt.Sprintf(":%v", serverPort), router)
}

func main() {
	logger.Init()
	var (
		serverPort    = flag.String("amd-listen-port", amdListenPort, "AMD listener port")
		metricsConfig = flag.String("amd-metrics-config", amdMetricsFile, "AMD metrics exporter config file")
	)
	flag.Parse()
	var err error
	gpuclient, err = gpuagent.NewAgent()
	if err != nil {
		logger.Log.Fatalf("GPUAgent create failed")
		return
	}
	defer gpuclient.Close()

	start_metrics_server(*serverPort, *metricsConfig)
}
