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
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"log"

	"github.com/gofrs/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/amdgpu"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/gpumetrics"
)

type metrics struct {
	gpuNodesTotal      prometheus.Gauge
	gpuFanSpeed        prometheus.GaugeVec
	gpuAvgPkgPower     prometheus.GaugeVec
	gpuEdgeTemp        prometheus.GaugeVec
	gpuJunctionTemp    prometheus.GaugeVec
	gpuMemoryTemp      prometheus.GaugeVec
	gpuHBMTemp         prometheus.GaugeVec
	gpuUsage           prometheus.GaugeVec
	gpuGFXActivity     prometheus.GaugeVec
	gpuMemUsage        prometheus.GaugeVec
	gpuMemActivity     prometheus.GaugeVec
	gpuVoltage         prometheus.GaugeVec
	gpuPCIeBandwidth   prometheus.GaugeVec
	gpuEnergeyConsumed prometheus.GaugeVec
	gpuPCIeReplayCount prometheus.GaugeVec
	gpuClock           prometheus.GaugeVec
	gpuMemoryClock     prometheus.GaugeVec
	gpuPCIeTxUsage     prometheus.GaugeVec
	gpuPCIeRxUsage     prometheus.GaugeVec
	// end of fields
	gpuPowerUsage prometheus.GaugeVec

	//static field values
	gpuTotalMemory prometheus.GaugeVec
}

const (
	amdListenPort  = "5000"
	amdMetricsFile = "/etc/metrics/config.json"
	gpuagentAddr   = "0.0.0.0:50061"
)

var (
	reg             *prometheus.Registry
	lmetrics        *metrics
	gpuClient       amdgpu.GPUSvcClient
	mandatoryLables = []string{
		gpumetrics.GPUMetricLabel_GPU_UUID.String(),
		gpumetrics.GPUMetricLabel_SERIAL_NUMBER.String(),
	}
	exportLables map[string]bool
)

func initFieldConfig(config *gpumetrics.MetricConfig) map[string]bool {
	exportFieldMap := make(map[string]bool)
	// setup metric fields in map to be monitored
	// init the map with all supported strings from enum
	enable_default := true
	if config != nil && len(config.Field) != 0 {
		enable_default = false
	}
	for _, name := range gpumetrics.GPUMetricField_name {
		log.Printf("%v set to %v", name, enable_default)
		exportFieldMap[name] = enable_default
	}
	if config == nil || len(config.Field) == 0 {
		return exportFieldMap
	}
	for _, fieldName := range config.Field {
		fieldName = strings.ToUpper(fieldName)
		if _, ok := exportFieldMap[fieldName]; ok {
			log.Printf("%v enabled", fieldName)
			exportFieldMap[fieldName] = true
		} else {
			log.Printf("Unsupported field is ignored: %v", fieldName)
		}
	}
	return exportFieldMap
}

func getExportLableList() []string {
	labelList := []string{}
	for key, enabled := range exportLables {
		if !enabled {
			continue
		}
		labelList = append(labelList, key)
	}
	return labelList
}

func initLableConfigs(config *gpumetrics.MetricConfig) {
	// list of mandatory labels
	exportLables = make(map[string]bool)
	for _, name := range gpumetrics.GPUMetricLabel_name {
		exportLables[name] = false
	}
	// only mandatory labels are set for default
	for _, name := range mandatoryLables {
		exportLables[name] = true
	}

	if config != nil {
		for _, name := range config.Label {
			name = strings.ToUpper(name)
			if _, ok := exportLables[name]; ok {
				log.Printf("label %v enabled", name)
				exportLables[name] = true
			}
		}
	}
}

func initMetrics(reg prometheus.Registerer, config *gpumetrics.MetricConfig) *metrics {
	initLableConfigs(config)
	labels := getExportLableList()
	labelsWithIndex := append(labels, "hbm_index")
	m := &metrics{
		gpuNodesTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "gpu_nodes_total",
				Help: "Number of nodes with GPUs",
			},
		),
		gpuFanSpeed: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_fan_speed",
			Help: "Current fan speed",
		},
			labels),
		gpuAvgPkgPower: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_average_package_power",
			Help: "Average package power in Watts",
		},
			labels),
		gpuEdgeTemp: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_edge_temperature",
			Help: "Current edge temperature in celsius",
		},
			labels),
		gpuJunctionTemp: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_junction_temperature",
			Help: "Current junction/hotspot temperature in celsius",
		},
			labels),
		gpuMemoryTemp: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_temperature",
			Help: "Current memory temperature in celsius",
		},
			labels),
		gpuHBMTemp: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_hbm_temperature",
			Help: "Current HBM temperature in celsius",
		},
			labelsWithIndex),
		gpuUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_usage",
			Help: "Current usage as percentage of time the GPU is busy.",
		},
			labels),
		gpuGFXActivity: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_gfx_activity",
			Help: "Current GFX activity",
		},
			labels),
		gpuMemUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_usage",
			Help: "Current memory usage as percentage of available memory in use",
		},
			labels),
		gpuMemActivity: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_activity",
			Help: "Current memory usage activity",
		},
			labels),
		gpuVoltage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_voltage",
			Help: "Current voltage draw in mV",
		},
			labels),
		gpuPCIeBandwidth: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pcie_bandwidth",
			Help: "estimated maximum PCIe bandwidth over the last second in MB/s",
		},
			labels),
		gpuEnergeyConsumed: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_energy_consumed",
			Help: "accumulated energy consumed in uJ",
		},
			labels),
		gpuPCIeReplayCount: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pcie_replay_count",
			Help: "PCIe replay count",
		},
			labels),
		gpuClock: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_clock",
			Help: "current GPU clock frequency in MHz",
		},
			labels),
		gpuMemoryClock: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_clock",
			Help: "current memory clock frequency in MHz",
		},
			labels),
		gpuPCIeTxUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pcie_tx",
			Help: "PCIe Tx utilization",
		},
			labels),
		gpuPCIeRxUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pcie_rx",
			Help: "PCIe Rx utilization",
		},
			labels),
		gpuPowerUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_power_usage",
			Help: "power usage in Watts",
		},
			labels),
		gpuTotalMemory: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_total_memory",
			Help: "total VRAM memory of the GPU (in MB)",
		},
			labels),
	}
	metricMap := initFieldConfig(config)

	for field, enabled := range metricMap {
		if !enabled {
			continue
		}
		switch field {
		case gpumetrics.GPUMetricField_GPU_NODES_TOTAL.String():
			reg.MustRegister(m.gpuNodesTotal)
		case gpumetrics.GPUMetricField_GPU_FAN_SPEED.String():
			reg.MustRegister(m.gpuFanSpeed)
		case gpumetrics.GPUMetricField_GPU_AVERAGE_PACKAGE_POWER.String():
			reg.MustRegister(m.gpuAvgPkgPower)
		case gpumetrics.GPUMetricField_GPU_EDGE_TEMPERATURE.String():
			reg.MustRegister(m.gpuEdgeTemp)
		case gpumetrics.GPUMetricField_GPU_JUNCTION_TEMPERATURE.String():
			reg.MustRegister(m.gpuJunctionTemp)
		case gpumetrics.GPUMetricField_GPU_MEMORY_TEMPERATURE.String():
			reg.MustRegister(m.gpuMemoryTemp)
		case gpumetrics.GPUMetricField_GPU_HBM_TEMPERATURE.String():
			reg.MustRegister(m.gpuHBMTemp)
		case gpumetrics.GPUMetricField_GPU_USAGE.String():
			reg.MustRegister(m.gpuUsage)
		case gpumetrics.GPUMetricField_GPU_GFX_ACTIVITY.String():
			reg.MustRegister(m.gpuGFXActivity)
		case gpumetrics.GPUMetricField_GPU_MEMORY_USAGE.String():
			reg.MustRegister(m.gpuMemUsage)
		case gpumetrics.GPUMetricField_GPU_MEMORY_ACTIVITY.String():
			reg.MustRegister(m.gpuMemActivity)
		case gpumetrics.GPUMetricField_GPU_VOLTAGE.String():
			reg.MustRegister(m.gpuVoltage)
		case gpumetrics.GPUMetricField_PCIE_BANDWIDTH.String():
			reg.MustRegister(m.gpuPCIeBandwidth)
		case gpumetrics.GPUMetricField_GPU_ENERGY_CONSUMED.String():
			reg.MustRegister(m.gpuEnergeyConsumed)
		case gpumetrics.GPUMetricField_PCIE_REPLAY_COUNT.String():
			reg.MustRegister(m.gpuPCIeReplayCount)
		case gpumetrics.GPUMetricField_GPU_CLOCK.String():
			reg.MustRegister(m.gpuClock)
		case gpumetrics.GPUMetricField_GPU_MEMORY_CLOCK.String():
			reg.MustRegister(m.gpuMemoryClock)
		case gpumetrics.GPUMetricField_PCIE_TX.String():
			reg.MustRegister(m.gpuPCIeTxUsage)
		case gpumetrics.GPUMetricField_PCIE_RX.String():
			reg.MustRegister(m.gpuPCIeRxUsage)
		case gpumetrics.GPUMetricField_GPU_POWER_USAGE.String():
			reg.MustRegister(m.gpuPowerUsage)
		case gpumetrics.GPUMetricField_GPU_TOTAL_MEMORY.String():
			reg.MustRegister(m.gpuTotalMemory)
		default:
			log.Printf("Invalid field encountered %v", field)
		}
	}

	return m
}

func populateLabelsFromGPU(gpu *amdgpu.GPU) map[string]string {
	labels := make(map[string]string)
	for key, enabled := range exportLables {
		if !enabled {
			continue
		}
		switch key {
		case gpumetrics.GPUMetricLabel_GPU_UUID.String():
			uuid, _ := uuid.FromBytes(gpu.Spec.Id)
			labels[key] = uuid.String()
		case gpumetrics.GPUMetricLabel_SERIAL_NUMBER.String():
			labels[key] = gpu.Status.SerialNum
		case gpumetrics.GPUMetricLabel_CARD_SERIES.String():
			labels[key] = gpu.Status.CardSeries
		case gpumetrics.GPUMetricLabel_CARD_MODEL.String():
			labels[key] = gpu.Status.CardModel
		case gpumetrics.GPUMetricLabel_CARD_VENDOR.String():
			labels[key] = gpu.Status.CardVendor
		case gpumetrics.GPUMetricLabel_DRIVER_VERSION.String():
			labels[key] = gpu.Status.DriverVersion
		case gpumetrics.GPUMetricLabel_VBIOS_VERSION.String():
			labels[key] = gpu.Status.VBIOSVersion
		default:
			log.Printf("Invalid label is ignored %v", key)
		}
	}
	return labels
}

func initStaticMetrics(resp *amdgpu.GPUGetResponse, m *metrics) {
	m.gpuNodesTotal.Set(float64(len(resp.Response)))
	for _, gpu := range resp.Response {
		status := gpu.Status
		labels := populateLabelsFromGPU(gpu)
		m.gpuTotalMemory.With(labels).Set(float64(status.TotalMemory))
	}
}

func convertGPUInfoToMetrics(resp *amdgpu.GPUGetResponse, m *metrics) {
	for _, gpu := range resp.Response {
		labels := populateLabelsFromGPU(gpu)
		labelsWithIndex := populateLabelsFromGPU(gpu)
		stats := gpu.Stats
		m.gpuFanSpeed.With(labels).Set(float64(stats.FanSpeed))
		m.gpuAvgPkgPower.With(labels).Set(float64(stats.AvgPackagePower))

		// gpu temp stats
		tempStats := stats.Temperature
		m.gpuEdgeTemp.With(labels).Set(float64(tempStats.EdgeTemperature))
		m.gpuJunctionTemp.With(labels).Set(float64(tempStats.JunctionTemperature))
		m.gpuMemoryTemp.With(labels).Set(float64(tempStats.MemoryTemperature))
		for j, temp := range tempStats.HBMTemperature {
			labelsWithIndex["hbm_index"] = fmt.Sprintf("%v", j)
			m.gpuHBMTemp.With(labelsWithIndex).Set(float64(temp))
		}
		// gpu usage
		m.gpuUsage.With(labels).Set(float64(stats.Usage.Usage))
		m.gpuGFXActivity.With(labels).Set(float64(stats.Usage.GFXActivity))

		// gpu memory usage
		m.gpuMemUsage.With(labels).Set(float64(stats.MemoryUsage.MemoryUsage))
		m.gpuMemActivity.With(labels).Set(float64(stats.MemoryUsage.Activity))

		m.gpuVoltage.With(labels).Set(float64(stats.Voltage))

		// pcie stats
		m.gpuPCIeBandwidth.With(labels).Set(float64(stats.PCIeBandwidth))
		m.gpuEnergeyConsumed.With(labels).Set(float64(stats.EnergyConsumed))
		m.gpuPCIeReplayCount.With(labels).Set(float64(stats.PCIeReplayCount))

		// clock stats
		m.gpuClock.With(labels).Set(float64(stats.GPUClock))
		m.gpuMemoryClock.With(labels).Set(float64(stats.MemoryClock))

		// pcie usage
		m.gpuPCIeTxUsage.With(labels).Set(float64(stats.PCIeTxUsage))
		m.gpuPCIeRxUsage.With(labels).Set(float64(stats.PCIeRxUsage))

		m.gpuPowerUsage.With(labels).Set(float64(stats.PowerUsage))
	}
}

// get the info from gpu agent and update the current metrics registery
func updateGPUInfo() error {
	m, _ := getMetricsHandle()
	// send the req to gpuagent
	cl, err := getGpuAgentSession()
	if err != nil {
		return err
	}
	req := &amdgpu.GPUGetRequest{}
	res, err := cl.GPUGet(context.Background(), req)
	if err != nil {
		log.Printf("err :%v", err)
		return err
	}
	if res != nil && res.ApiStatus != 0 {
		log.Printf("resp status :%v", res.ApiStatus)
		return fmt.Errorf("%v", res.ApiStatus)
	}
	convertGPUInfoToMetrics(res, m)
	return nil
}

func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		updateGPUInfo()
		next.ServeHTTP(w, r)
	})
}

func getGpuAgentSession() (amdgpu.GPUSvcClient, error) {
	if gpuClient == nil {
		return nil, fmt.Errorf("invalid client")
	}
	return gpuClient, nil
}

func getRegistry() (*prometheus.Registry, error) {
	return reg, nil
}

func getMetricsHandle() (*metrics, error) {
	return lmetrics, nil
}

func start_metrics_server(serverPort string, configPath string) {
	var config_fields gpumetrics.MetricConfig
	pconfig_fields := &config_fields
	fields, err := ioutil.ReadFile(configPath)
	if err != nil {
		pconfig_fields = nil
	} else {
		_ = json.Unmarshal(fields, pconfig_fields)
		log.Printf("fields : %+v", pconfig_fields)
	}

	reg = prometheus.NewRegistry()
	lmetrics = initMetrics(reg, pconfig_fields)
	conn, err := grpc.Dial(gpuagentAddr, grpc.WithInsecure())
	if err != nil {
		log.Printf("err :%v", err)
		return
	}
	defer conn.Close()
	gpuClient = amdgpu.NewGPUSvcClient(conn)
	req := &amdgpu.GPUGetRequest{}
	res, err := gpuClient.GPUGet(context.Background(), req)
	if err != nil {
		log.Printf("err :%v", err)
		return
	}
	for i, gpu := range res.Response {
		log.Printf("GPU[%v].Status :%+v", i, gpu.Status)
	}
	initStaticMetrics(res, lmetrics)

	router := mux.NewRouter()
	router.Use(prometheusMiddleware)

	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	log.Printf("Serving requests on port %v", serverPort)
	err = http.ListenAndServe(fmt.Sprintf(":%v", serverPort), router)
}

func main() {
	var (
		serverPort    = flag.String("amd-listen-port", amdListenPort, "AMD listener port")
		metricsConfig = flag.String("amd-metrics-config", amdMetricsFile, "AMD metrics exporter config file")
	)
	flag.Parse()

	start_metrics_server(*serverPort, *metricsConfig)
}
