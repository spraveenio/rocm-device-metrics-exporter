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
	"fmt"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/amdgpu"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/gpumetrics"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/logger"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	mandatoryLables = []string{
		gpumetrics.GPUMetricLabel_GPU_UUID.String(),
		gpumetrics.GPUMetricLabel_SERIAL_NUMBER.String(),
	}
)

type MetricsHandler struct {
	reg            *prometheus.Registry
	m              *metrics
	exportLables   map[string]bool
	exportFieldMap map[string]bool
}

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

func NewMetrics(config *gpumetrics.MetricConfig) (*MetricsHandler, error) {
	metricsHandler := MetricsHandler{
		reg: prometheus.NewRegistry(),
	}
	metricsHandler.initMetrics(config)
	return &metricsHandler, nil
}

func (mh *MetricsHandler) GetRegistry() *prometheus.Registry {
	return mh.reg
}

func (mh *MetricsHandler) initConfig(config *gpumetrics.MetricConfig) {
	mh.initLableConfigs(config)
	mh.initFieldConfig(config)
}

func (mh *MetricsHandler) initFieldConfig(config *gpumetrics.MetricConfig) {
	mh.exportFieldMap = make(map[string]bool)
	// setup metric fields in map to be monitored
	// init the map with all supported strings from enum
	enable_default := true
	if config != nil && len(config.Field) != 0 {
		enable_default = false
	}
	for _, name := range gpumetrics.GPUMetricField_name {
		logger.Log.Printf("%v set to %v", name, enable_default)
		mh.exportFieldMap[name] = enable_default
	}
	if config == nil || len(config.Field) == 0 {
		return
	}
	for _, fieldName := range config.Field {
		fieldName = strings.ToUpper(fieldName)
		if _, ok := mh.exportFieldMap[fieldName]; ok {
			logger.Log.Printf("%v enabled", fieldName)
			mh.exportFieldMap[fieldName] = true
		} else {
			logger.Log.Printf("Unsupported field is ignored: %v", fieldName)
		}
	}
	return
}

func (mh *MetricsHandler) initLableConfigs(config *gpumetrics.MetricConfig) {
	// list of mandatory labels
	mh.exportLables = make(map[string]bool)
	for _, name := range gpumetrics.GPUMetricLabel_name {
		mh.exportLables[name] = false
	}
	// only mandatory labels are set for default
	for _, name := range mandatoryLables {
		mh.exportLables[name] = true
	}

	if config != nil {
		for _, name := range config.Label {
			name = strings.ToUpper(name)
			if _, ok := mh.exportLables[name]; ok {
				logger.Log.Printf("label %v enabled", name)
				mh.exportLables[name] = true
			}
		}
	}
}

func (mh *MetricsHandler) getExportLableList() []string {
	labelList := []string{}
	for key, enabled := range mh.exportLables {
		if !enabled {
			continue
		}
		labelList = append(labelList, key)
	}
	return labelList
}

func initPrometheusMetrics(mh *MetricsHandler) {
	labels := mh.getExportLableList()
	labelsWithIndex := append(labels, "hbm_index")
    mh.m = &metrics{
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
}

func (mh *MetricsHandler) initFieldRegistration() error {
	for field, enabled := range mh.exportFieldMap {
		if !enabled {
			continue
		}
		switch field {
		case gpumetrics.GPUMetricField_GPU_NODES_TOTAL.String():
			mh.reg.MustRegister(mh.m.gpuNodesTotal)
		case gpumetrics.GPUMetricField_GPU_FAN_SPEED.String():
			mh.reg.MustRegister(mh.m.gpuFanSpeed)
		case gpumetrics.GPUMetricField_GPU_AVERAGE_PACKAGE_POWER.String():
			mh.reg.MustRegister(mh.m.gpuAvgPkgPower)
		case gpumetrics.GPUMetricField_GPU_EDGE_TEMPERATURE.String():
			mh.reg.MustRegister(mh.m.gpuEdgeTemp)
		case gpumetrics.GPUMetricField_GPU_JUNCTION_TEMPERATURE.String():
			mh.reg.MustRegister(mh.m.gpuJunctionTemp)
		case gpumetrics.GPUMetricField_GPU_MEMORY_TEMPERATURE.String():
			mh.reg.MustRegister(mh.m.gpuMemoryTemp)
		case gpumetrics.GPUMetricField_GPU_HBM_TEMPERATURE.String():
			mh.reg.MustRegister(mh.m.gpuHBMTemp)
		case gpumetrics.GPUMetricField_GPU_USAGE.String():
			mh.reg.MustRegister(mh.m.gpuUsage)
		case gpumetrics.GPUMetricField_GPU_GFX_ACTIVITY.String():
			mh.reg.MustRegister(mh.m.gpuGFXActivity)
		case gpumetrics.GPUMetricField_GPU_MEMORY_USAGE.String():
			mh.reg.MustRegister(mh.m.gpuMemUsage)
		case gpumetrics.GPUMetricField_GPU_MEMORY_ACTIVITY.String():
			mh.reg.MustRegister(mh.m.gpuMemActivity)
		case gpumetrics.GPUMetricField_GPU_VOLTAGE.String():
			mh.reg.MustRegister(mh.m.gpuVoltage)
		case gpumetrics.GPUMetricField_PCIE_BANDWIDTH.String():
			mh.reg.MustRegister(mh.m.gpuPCIeBandwidth)
		case gpumetrics.GPUMetricField_GPU_ENERGY_CONSUMED.String():
			mh.reg.MustRegister(mh.m.gpuEnergeyConsumed)
		case gpumetrics.GPUMetricField_PCIE_REPLAY_COUNT.String():
			mh.reg.MustRegister(mh.m.gpuPCIeReplayCount)
		case gpumetrics.GPUMetricField_GPU_CLOCK.String():
			mh.reg.MustRegister(mh.m.gpuClock)
		case gpumetrics.GPUMetricField_GPU_MEMORY_CLOCK.String():
			mh.reg.MustRegister(mh.m.gpuMemoryClock)
		case gpumetrics.GPUMetricField_PCIE_TX.String():
			mh.reg.MustRegister(mh.m.gpuPCIeTxUsage)
		case gpumetrics.GPUMetricField_PCIE_RX.String():
			mh.reg.MustRegister(mh.m.gpuPCIeRxUsage)
		case gpumetrics.GPUMetricField_GPU_POWER_USAGE.String():
			mh.reg.MustRegister(mh.m.gpuPowerUsage)
		case gpumetrics.GPUMetricField_GPU_TOTAL_MEMORY.String():
			mh.reg.MustRegister(mh.m.gpuTotalMemory)
		default:
			logger.Log.Printf("Invalid field %v is ignored", field)
		}
	}

	return nil
}

func (mh *MetricsHandler) initMetrics(config *gpumetrics.MetricConfig) error {
	mh.initConfig(config)
	initPrometheusMetrics(mh)
	return mh.initFieldRegistration()
}

func populateLabelsFromGPU(mh *MetricsHandler, gpu *amdgpu.GPU) map[string]string {
	labels := make(map[string]string)
	for key, enabled := range mh.exportLables {
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
			logger.Log.Printf("Invalid label is ignored %v", key)
		}
	}
	return labels
}

func (mh *MetricsHandler) InitStaticMetrics(resp *amdgpu.GPUGetResponse) {
	mh.m.gpuNodesTotal.Set(float64(len(resp.Response)))
	for _, gpu := range resp.Response {
		status := gpu.Status
		labels := populateLabelsFromGPU(mh, gpu)
		mh.m.gpuTotalMemory.With(labels).Set(float64(status.TotalMemory))
	}
}

func (mh *MetricsHandler) UpdateGPUInfoToMetrics(resp *amdgpu.GPUGetResponse) {
	for _, gpu := range resp.Response {
		labels := populateLabelsFromGPU(mh, gpu)
		labelsWithIndex := populateLabelsFromGPU(mh, gpu)
		stats := gpu.Stats
		mh.m.gpuFanSpeed.With(labels).Set(float64(stats.FanSpeed))
		mh.m.gpuAvgPkgPower.With(labels).Set(float64(stats.AvgPackagePower))

		// gpu temp stats
		tempStats := stats.Temperature
		mh.m.gpuEdgeTemp.With(labels).Set(float64(tempStats.EdgeTemperature))
		mh.m.gpuJunctionTemp.With(labels).Set(float64(tempStats.JunctionTemperature))
		mh.m.gpuMemoryTemp.With(labels).Set(float64(tempStats.MemoryTemperature))
		for j, temp := range tempStats.HBMTemperature {
			labelsWithIndex["hbm_index"] = fmt.Sprintf("%v", j)
			mh.m.gpuHBMTemp.With(labelsWithIndex).Set(float64(temp))
		}
		// gpu usage
		mh.m.gpuUsage.With(labels).Set(float64(stats.Usage.Usage))
		mh.m.gpuGFXActivity.With(labels).Set(float64(stats.Usage.GFXActivity))

		// gpu memory usage
		mh.m.gpuMemUsage.With(labels).Set(float64(stats.MemoryUsage.MemoryUsage))
		mh.m.gpuMemActivity.With(labels).Set(float64(stats.MemoryUsage.Activity))

		mh.m.gpuVoltage.With(labels).Set(float64(stats.Voltage))

		// pcie stats
		mh.m.gpuPCIeBandwidth.With(labels).Set(float64(stats.PCIeBandwidth))
		mh.m.gpuEnergeyConsumed.With(labels).Set(float64(stats.EnergyConsumed))
		mh.m.gpuPCIeReplayCount.With(labels).Set(float64(stats.PCIeReplayCount))

		// clock stats
		mh.m.gpuClock.With(labels).Set(float64(stats.GPUClock))
		mh.m.gpuMemoryClock.With(labels).Set(float64(stats.MemoryClock))

		// pcie usage
		mh.m.gpuPCIeTxUsage.With(labels).Set(float64(stats.PCIeTxUsage))
		mh.m.gpuPCIeRxUsage.With(labels).Set(float64(stats.PCIeRxUsage))

		mh.m.gpuPowerUsage.With(labels).Set(float64(stats.PowerUsage))
	}
}
