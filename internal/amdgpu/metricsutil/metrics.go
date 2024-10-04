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

package metricsutil

import (
	"encoding/json"
	"io/ioutil"
	"sync"

	"github.com/pensando/device-metrics-exporter/internal/amdgpu/config"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/gpumetrics"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsHandler struct {
	reg          *prometheus.Registry
	runConf      *config.Config
	metricConfig *gpumetrics.MetricConfig
	clients      []MetricsInterface
}

func readConfig(c *config.Config) *gpumetrics.MetricConfig {
	var config_fields gpumetrics.MetricConfig
	pmConfigs := &config_fields
	mConfigs, err := ioutil.ReadFile(c.GetMetricsConfigPath())
	if err != nil {
		pmConfigs = nil
	} else {
		_ = json.Unmarshal(mConfigs, pmConfigs)
	}
	return pmConfigs

}

func NewMetrics(c *config.Config) (*MetricsHandler, error) {
	metricsHandler := MetricsHandler{
		runConf: c,
	}
	metricsHandler.clients = []MetricsInterface{}
	return &metricsHandler, nil
}

// GetRunConfig : returns the running config handle
func (mh *MetricsHandler) GetRunConfig() *config.Config {
	return mh.runConf
}

// GetRegistry : returns the registry handle
func (mh *MetricsHandler) GetRegistry() *prometheus.Registry {
	return mh.reg
}

func (mh *MetricsHandler) RegisterMetricsClient(client MetricsInterface) {
	mh.clients = append(mh.clients, client)
}

func (mh *MetricsHandler) InitConfig() {
	mh.reg = prometheus.NewRegistry()
	pmConfigs := readConfig(mh.runConf)
	mh.metricConfig = pmConfigs
	mh.updateServerPort()
	var wg sync.WaitGroup
	for _, client := range mh.clients {
		wg.Add(1)
		go func(client MetricsInterface) {
			defer wg.Done()
			client.InitConfigs()
			client.UpdateStaticMetrics()
		}(client)
	}
	wg.Wait()
}

// UpdateMetrics : send on demand update metrics request
func (mh *MetricsHandler) UpdateMetrics() error {
	var wg sync.WaitGroup
	for _, client := range mh.clients {
		wg.Add(1)
		go func(client MetricsInterface) {
			defer wg.Done()
			client.UpdateMetricsStats()
		}(client)
	}
	wg.Wait()
	return nil
}

// ResetMetrics : send reset requet to all clients
func (mh *MetricsHandler) ResetMetrics() error {
	var wg sync.WaitGroup
	for _, client := range mh.clients {
		wg.Add(1)
		go func(client MetricsInterface) {
			defer wg.Done()
			client.ResetMetrics()
		}(client)
	}
	wg.Wait()
	return nil
}

func (mh *MetricsHandler) GetMetricsConfig() *gpumetrics.GPUMetricConfig {
	if mh.metricConfig != nil {
		return mh.metricConfig.GetGPUConfig()
	}
	return nil
}

func (mh *MetricsHandler) GetAgentAddr() string {
	return mh.runConf.GetAgentAddr()
}

func (mh *MetricsHandler) updateServerPort() {
	if mh.metricConfig != nil && mh.metricConfig.GetServerPort() != 0 {
		mh.runConf.SetServerPort(mh.metricConfig.GetServerPort())
	}
}
