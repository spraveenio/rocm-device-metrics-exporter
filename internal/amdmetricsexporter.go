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

	"github.com/pensando/device-metrics-exporter/gen/amdgpu"
	"github.com/pensando/device-metrics-exporter/gen/gpumetrics"
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
	amdMetricsFile = "export_configs.json"
	gpuagentAddr   = "0.0.0.0:50061"
)

var (
	reg       *prometheus.Registry
	lmetrics  *metrics
	gpuClient amdgpu.GPUSvcClient
)

func initFieldConfig(config *gpumetrics.MonitorFields) map[string]bool {
	exportFieldMap := make(map[string]bool)
	// setup metric fields in map to be monitored
	// init the map with all supported strings from enum
	enable_default := (config == nil)
	for _, name := range gpumetrics.GPUMetricField_name {
		log.Printf("%v set to %v", name, enable_default)
		exportFieldMap[name] = enable_default
	}
	if config == nil {
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

func initMetrics(reg prometheus.Registerer, config *gpumetrics.MonitorFields) *metrics {
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
			[]string{"gpu_index"}),
		gpuAvgPkgPower: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_average_package_power",
			Help: "Average package power in Watts",
		},
			[]string{"gpu_index"}),
		gpuEdgeTemp: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_edge_temperature",
			Help: "Current edge temperature in celsius",
		},
			[]string{"gpu_index"}),
		gpuJunctionTemp: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_junction_temperature",
			Help: "Current junction/hotspot temperature in celsius",
		},
			[]string{"gpu_index"}),
		gpuMemoryTemp: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_temperature",
			Help: "Current memory temperature in celsius",
		},
			[]string{"gpu_index"}),
		gpuHBMTemp: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_hbm_temperature",
			Help: "Current HBM temperature in celsius",
		},
			[]string{"gpu_index", "hbm_index"}),
		gpuUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_usage",
			Help: "Current usage as percentage of time the GPU is busy.",
		},
			[]string{"gpu_index"}),
		gpuGFXActivity: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_gfx_activity",
			Help: "Current GFX activity",
		},
			[]string{"gpu_index"}),
		gpuMemUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_usage",
			Help: "Current memory usage as percentage of available memory in use",
		},
			[]string{"gpu_index"}),
		gpuMemActivity: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_activity",
			Help: "Current memory usage activity",
		},
			[]string{"gpu_index"}),
		gpuVoltage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_voltage",
			Help: "Current voltage draw in mV",
		},
			[]string{"gpu_index"}),
		gpuPCIeBandwidth: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pcie_bandwidth",
			Help: "estimated maximum PCIe bandwidth over the last second in MB/s",
		},
			[]string{"gpu_index"}),
		gpuEnergeyConsumed: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_energy_consumed",
			Help: "accumulated energy consumed in uJ",
		},
			[]string{"gpu_index"}),
		gpuPCIeReplayCount: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pcie_replay_count",
			Help: "PCIe replay count",
		},
			[]string{"gpu_index"}),
		gpuClock: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_clock",
			Help: "current GPU clock frequency in MHz",
		},
			[]string{"gpu_index"}),
		gpuMemoryClock: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_clock",
			Help: "current memory clock frequency in MHz",
		},
			[]string{"gpu_index"}),
		gpuPCIeTxUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pcie_tx",
			Help: "PCIe Tx utilization",
		},
			[]string{"gpu_index"}),
		gpuPCIeRxUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pcie_rx",
			Help: "PCIe Rx utilization",
		},
			[]string{"gpu_index"}),
		gpuPowerUsage: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_power_usage",
			Help: "power usage in Watts",
		},
			[]string{"gpu_index"}),
		gpuTotalMemory: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_total_memory",
			Help: "total VRAM memory of the GPU (in MB)",
		},
			[]string{"gpu_index", "serial_number", "card_series",
				"card_model", "card_vendor", "driver_version", "vbios_version"}),
	}
	metricMap := initFieldConfig(config)

	for field, enabled := range metricMap {
		if !enabled {
			continue
		}
		switch field {
		case "GPU_NODES_TOTAL":
			reg.MustRegister(m.gpuNodesTotal)
		case "GPU_FAN_SPEED":
			reg.MustRegister(m.gpuFanSpeed)
		case "GPU_AVERAGE_PACKAGE_POWER":
			reg.MustRegister(m.gpuAvgPkgPower)
		case "GPU_EDGE_TEMPERATURE":
			reg.MustRegister(m.gpuEdgeTemp)
		case "GPU_JUNCTION_TEMPERATURE":
			reg.MustRegister(m.gpuJunctionTemp)
		case "GPU_MEMORY_TEMPERATURE":
			reg.MustRegister(m.gpuMemoryTemp)
		case "GPU_HBM_TEMPERATURE":
			reg.MustRegister(m.gpuHBMTemp)
		case "GPU_USAGE":
			reg.MustRegister(m.gpuUsage)
		case "GPU_GFX_ACTIVITY":
			reg.MustRegister(m.gpuGFXActivity)
		case "GPU_MEMORY_USAGE":
			reg.MustRegister(m.gpuMemUsage)
		case "GPU_MEMORY_ACTIVITY":
			reg.MustRegister(m.gpuMemActivity)
		case "GPU_VOLTAGE":
			reg.MustRegister(m.gpuVoltage)
		case "PCIE_BANDWIDTH":
			reg.MustRegister(m.gpuPCIeBandwidth)
		case "GPU_ENERGY_CONSUMED":
			reg.MustRegister(m.gpuEnergeyConsumed)
		case "PCIE_REPLAY_COUNT":
			reg.MustRegister(m.gpuPCIeReplayCount)
		case "GPU_CLOCK":
			reg.MustRegister(m.gpuClock)
		case "GPU_MEMORY_CLOCK":
			reg.MustRegister(m.gpuMemoryClock)
		case "PCIE_TX":
			reg.MustRegister(m.gpuPCIeTxUsage)
		case "PCIE_RX":
			reg.MustRegister(m.gpuPCIeRxUsage)
		case "GPU_POWER_USAGE":
			reg.MustRegister(m.gpuPowerUsage)
		case "GPU_TOTAL_MEMORY":
			reg.MustRegister(m.gpuTotalMemory)
		default:
			log.Printf("Invalid field encountered %v", field)
		}
	}

	return m
}

func initStaticMetrics(resp *amdgpu.GPUGetResponse, m *metrics) {
	m.gpuNodesTotal.Set(float64(len(resp.Response)))
	for _, gpu := range resp.Response {
		status := gpu.Status
		uuid, _ := uuid.FromBytes(gpu.Spec.Id)
		gpu_index := uuid.String()
		labels := map[string]string{
			"gpu_index":      gpu_index,
			"serial_number":  status.SerialNum,
			"card_series":    status.CardSeries,
			"card_model":     status.CardModel,
			"card_vendor":    status.CardVendor,
			"driver_version": status.DriverVersion,
			"vbios_version":  status.VBIOSVersion,
		}
		m.gpuTotalMemory.With(labels).Set(float64(status.TotalMemory))
	}
}

func convertGPUInfoToMetrics(resp *amdgpu.GPUGetResponse, m *metrics) {
	for _, gpu := range resp.Response {
		stats := gpu.Stats
		uuid, _ := uuid.FromBytes(gpu.Spec.Id)
		gpu_index := uuid.String()
		m.gpuFanSpeed.WithLabelValues(gpu_index).Set(float64(stats.FanSpeed))
		m.gpuAvgPkgPower.WithLabelValues(gpu_index).Set(float64(stats.AvgPackagePower))

		// gpu temp stats
		tempStats := stats.Temperature
		m.gpuEdgeTemp.WithLabelValues(gpu_index).Set(float64(tempStats.EdgeTemperature))
		m.gpuJunctionTemp.WithLabelValues(gpu_index).Set(float64(tempStats.JunctionTemperature))
		m.gpuMemoryTemp.WithLabelValues(gpu_index).Set(float64(tempStats.MemoryTemperature))
		for j, temp := range tempStats.HBMTemperature {
			hbm_index := fmt.Sprintf("%v", j)
			m.gpuHBMTemp.WithLabelValues(gpu_index, hbm_index).Set(float64(temp))
		}
		// gpu usage
		m.gpuUsage.WithLabelValues(gpu_index).Set(float64(stats.Usage.Usage))
		m.gpuGFXActivity.WithLabelValues(gpu_index).Set(float64(stats.Usage.GFXActivity))

		// gpu memory usage
		m.gpuMemUsage.WithLabelValues(gpu_index).Set(float64(stats.MemoryUsage.MemoryUsage))
		m.gpuMemActivity.WithLabelValues(gpu_index).Set(float64(stats.MemoryUsage.Activity))

		m.gpuVoltage.WithLabelValues(gpu_index).Set(float64(stats.Voltage))

		// pcie stats
		m.gpuPCIeBandwidth.WithLabelValues(gpu_index).Set(float64(stats.PCIeBandwidth))
		m.gpuEnergeyConsumed.WithLabelValues(gpu_index).Set(float64(stats.EnergyConsumed))
		m.gpuPCIeReplayCount.WithLabelValues(gpu_index).Set(float64(stats.PCIeReplayCount))

		// clock stats
		m.gpuClock.WithLabelValues(gpu_index).Set(float64(stats.GPUClock))
		m.gpuMemoryClock.WithLabelValues(gpu_index).Set(float64(stats.MemoryClock))

		// pcie usage
		m.gpuPCIeTxUsage.WithLabelValues(gpu_index).Set(float64(stats.PCIeTxUsage))
		m.gpuPCIeRxUsage.WithLabelValues(gpu_index).Set(float64(stats.PCIeRxUsage))

		m.gpuPowerUsage.WithLabelValues(gpu_index).Set(float64(stats.PowerUsage))
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
	var config_fields gpumetrics.MonitorFields
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
