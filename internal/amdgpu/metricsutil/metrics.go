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
	"sync"

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
	reg             *prometheus.Registry
	m               *metrics
	exportLables    map[string]bool
	exportFieldMap  map[string]bool
	fieldMetricsMap []prometheus.Collector
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
	gpuPowerUsage      prometheus.GaugeVec

	gpuEccCorrectTotal      prometheus.GaugeVec
	gpuEccUncorrectTotal    prometheus.GaugeVec
	gpuEccCorrectSDMA       prometheus.GaugeVec
	gpuEccUncorrectSDMA     prometheus.GaugeVec
	gpuEccCorrectGFX        prometheus.GaugeVec
	gpuEccUncorrectGFX      prometheus.GaugeVec
	gpuEccCorrectMMHUB      prometheus.GaugeVec
	gpuEccUncorrectMMHUB    prometheus.GaugeVec
	gpuEccCorrectATHUB      prometheus.GaugeVec
	gpuEccUncorrectATHUB    prometheus.GaugeVec
	gpuEccCorrectBIF        prometheus.GaugeVec
	gpuEccUncorrectBIF      prometheus.GaugeVec
	gpuEccCorrectHDP        prometheus.GaugeVec
	gpuEccUncorrectHDP      prometheus.GaugeVec
	gpuEccCorrectXgmiWAFL   prometheus.GaugeVec
	gpuEccUncorrectXgmiWAFL prometheus.GaugeVec
	gpuEccCorrectDF         prometheus.GaugeVec
	gpuEccUncorrectDF       prometheus.GaugeVec
	gpuEccCorrectSMN        prometheus.GaugeVec
	gpuEccUncorrectSMN      prometheus.GaugeVec
	gpuEccCorrectSEM        prometheus.GaugeVec
	gpuEccUncorrectSEM      prometheus.GaugeVec
	gpuEccCorrectMP0        prometheus.GaugeVec
	gpuEccUncorrectMP0      prometheus.GaugeVec
	gpuEccCorrectMP1        prometheus.GaugeVec
	gpuEccUncorrectMP1      prometheus.GaugeVec
	gpuEccCorrectFUSE       prometheus.GaugeVec
	gpuEccUncorrectFUSE     prometheus.GaugeVec
	gpuEccCorrectUMC        prometheus.GaugeVec
	gpuEccUncorrectUMC      prometheus.GaugeVec
	xgmiNbrNopTx0           prometheus.GaugeVec
	xgmiNbrReqTx0           prometheus.GaugeVec
	xgmiNbrRespTx0          prometheus.GaugeVec
	xgmiNbrBeatsTx0         prometheus.GaugeVec
	xgmiNbrNopTx1           prometheus.GaugeVec
	xgmiNbrReqTx1           prometheus.GaugeVec
	xgmiNbrRespTx1          prometheus.GaugeVec
	xgmiNbrBeatsTx1         prometheus.GaugeVec
	xgmiNbrTxTput0          prometheus.GaugeVec
	xgmiNbrTxTput1          prometheus.GaugeVec
	xgmiNbrTxTput2          prometheus.GaugeVec
	xgmiNbrTxTput3          prometheus.GaugeVec
	xgmiNbrTxTput4          prometheus.GaugeVec
	xgmiNbrTxTput5          prometheus.GaugeVec

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
		gpuEccCorrectTotal: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_total",
		},
			labels),
		gpuEccUncorrectTotal: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_total",
		},
			labels),
		gpuEccCorrectSDMA: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_sdma",
		},
			labels),
		gpuEccUncorrectSDMA: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_sdma",
		},
			labels),
		gpuEccCorrectGFX: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_gfx",
		},
			labels),
		gpuEccUncorrectGFX: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_gfx",
		},
			labels),
		gpuEccCorrectMMHUB: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_mmhub",
		},
			labels),
		gpuEccUncorrectMMHUB: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_mmhub",
		},
			labels),
		gpuEccCorrectATHUB: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_athub",
		},
			labels),
		gpuEccUncorrectATHUB: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_athub",
		},
			labels),
		gpuEccCorrectBIF: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_bif",
		},
			labels),
		gpuEccUncorrectBIF: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_bif",
		},
			labels),
		gpuEccCorrectHDP: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_hdp",
		},
			labels),
		gpuEccUncorrectHDP: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_hdp",
		},
			labels),
		gpuEccCorrectXgmiWAFL: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_xgmi_wafl",
		},
			labels),
		gpuEccUncorrectXgmiWAFL: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_xgmi_wafl",
		},
			labels),
		gpuEccCorrectDF: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_df",
		},
			labels),
		gpuEccUncorrectDF: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_df",
		},
			labels),
		gpuEccCorrectSMN: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_smn",
		},
			labels),
		gpuEccUncorrectSMN: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_smn",
		},
			labels),
		gpuEccCorrectSEM: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_sem",
		},
			labels),
		gpuEccUncorrectSEM: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_sem",
		},
			labels),
		gpuEccCorrectMP0: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_mp0",
		},
			labels),
		gpuEccUncorrectMP0: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_mp0",
		},
			labels),
		gpuEccCorrectMP1: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_mp1",
		},
			labels),
		gpuEccUncorrectMP1: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_mp1",
		},
			labels),
		gpuEccCorrectFUSE: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_fuse",
		},
			labels),
		gpuEccUncorrectFUSE: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_fuse",
		},
			labels),
		gpuEccCorrectUMC: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_correct_umc",
		},
			labels),
		gpuEccUncorrectUMC: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_ecc_uncorrect_umc",
		},
			labels),
		xgmiNbrNopTx0: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_0_nop_tx",
		},
			labels),
		xgmiNbrNopTx1: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_1_nop_tx",
		},
			labels),
		xgmiNbrReqTx0: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_0_request_tx",
		},
			labels),
		xgmiNbrReqTx1: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_1_request_tx",
		},
			labels),
		xgmiNbrRespTx0: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_0_response_tx",
		},
			labels),
		xgmiNbrRespTx1: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_1_response_tx",
		},
			labels),
		xgmiNbrBeatsTx0: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_0_beats_tx",
		},
			labels),
		xgmiNbrBeatsTx1: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_1_beats_tx",
		},
			labels),
		xgmiNbrTxTput0: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_0_tx_throughput",
		},
			labels),
		xgmiNbrTxTput1: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_1_tx_throughput",
		},
			labels),
		xgmiNbrTxTput2: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_2_tx_throughput",
		},
			labels),
		xgmiNbrTxTput3: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_3_tx_throughput",
		},
			labels),
		xgmiNbrTxTput4: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_4_tx_throughput",
		},
			labels),
		xgmiNbrTxTput5: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "xgmi_neighbor_5_tx_throughput",
		},
			labels),
	}
	initFieldMetricsMap(mh)
}

func initFieldMetricsMap(mh *MetricsHandler) {
	// must follow index mapping to fields.proto (GPUMetricField)
	mh.fieldMetricsMap = []prometheus.Collector{
		mh.m.gpuNodesTotal,
		mh.m.gpuFanSpeed,
		mh.m.gpuAvgPkgPower,
		mh.m.gpuEdgeTemp,
		mh.m.gpuJunctionTemp,
		mh.m.gpuMemoryTemp,
		mh.m.gpuHBMTemp,
		mh.m.gpuUsage,
		mh.m.gpuGFXActivity,
		mh.m.gpuMemUsage,
		mh.m.gpuMemActivity,
		mh.m.gpuVoltage,
		mh.m.gpuPCIeBandwidth,
		mh.m.gpuEnergeyConsumed,
		mh.m.gpuPCIeReplayCount,
		mh.m.gpuClock,
		mh.m.gpuMemoryClock,
		mh.m.gpuPCIeTxUsage,
		mh.m.gpuPCIeRxUsage,
		mh.m.gpuPowerUsage,
		mh.m.gpuTotalMemory,
		mh.m.gpuEccCorrectTotal,
		mh.m.gpuEccUncorrectTotal,
		mh.m.gpuEccCorrectSDMA,
		mh.m.gpuEccUncorrectSDMA,
		mh.m.gpuEccCorrectGFX,
		mh.m.gpuEccUncorrectGFX,
		mh.m.gpuEccCorrectMMHUB,
		mh.m.gpuEccUncorrectMMHUB,
		mh.m.gpuEccCorrectATHUB,
		mh.m.gpuEccUncorrectATHUB,
		mh.m.gpuEccCorrectBIF,
		mh.m.gpuEccUncorrectBIF,
		mh.m.gpuEccCorrectHDP,
		mh.m.gpuEccUncorrectHDP,
		mh.m.gpuEccCorrectXgmiWAFL,
		mh.m.gpuEccUncorrectXgmiWAFL,
		mh.m.gpuEccCorrectDF,
		mh.m.gpuEccUncorrectDF,
		mh.m.gpuEccCorrectSMN,
		mh.m.gpuEccUncorrectSMN,
		mh.m.gpuEccCorrectSEM,
		mh.m.gpuEccUncorrectSEM,
		mh.m.gpuEccCorrectMP0,
		mh.m.gpuEccUncorrectMP0,
		mh.m.gpuEccCorrectMP1,
		mh.m.gpuEccUncorrectMP1,
		mh.m.gpuEccCorrectFUSE,
		mh.m.gpuEccUncorrectFUSE,
		mh.m.gpuEccCorrectUMC,
		mh.m.gpuEccUncorrectUMC,
		mh.m.xgmiNbrNopTx0,
		mh.m.xgmiNbrReqTx0,
		mh.m.xgmiNbrRespTx0,
		mh.m.xgmiNbrBeatsTx0,
		mh.m.xgmiNbrNopTx1,
		mh.m.xgmiNbrReqTx1,
		mh.m.xgmiNbrRespTx1,
		mh.m.xgmiNbrBeatsTx1,
		mh.m.xgmiNbrTxTput0,
		mh.m.xgmiNbrTxTput1,
		mh.m.xgmiNbrTxTput2,
		mh.m.xgmiNbrTxTput3,
		mh.m.xgmiNbrTxTput4,
		mh.m.xgmiNbrTxTput5,
	}

}

func (mh *MetricsHandler) initFieldRegistration() error {
	for field, enabled := range mh.exportFieldMap {
		if !enabled {
			continue
		}
		fieldIndex, ok := gpumetrics.GPUMetricField_value[field]
		if !ok {
			logger.Log.Printf("Invalid field %v, ignored", field)
			continue
		}
		mh.reg.MustRegister(mh.fieldMetricsMap[fieldIndex])
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

func (mh *MetricsHandler) updateGPUInfoToMetrics(gpu *amdgpu.GPU) {
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

	mh.m.gpuEccCorrectTotal.With(labels).Set(float64(stats.TotalCorrectableErrors))
	mh.m.gpuEccUncorrectTotal.With(labels).Set(float64(stats.TotalUncorrectableErrors))
	mh.m.gpuEccCorrectSDMA.With(labels).Set(float64(stats.SDMACorrectableErrors))
	mh.m.gpuEccUncorrectSDMA.With(labels).Set(float64(stats.SDMAUncorrectableErrors))
	mh.m.gpuEccCorrectGFX.With(labels).Set(float64(stats.GFXCorrectableErrors))
	mh.m.gpuEccUncorrectGFX.With(labels).Set(float64(stats.GFXUncorrectableErrors))
	mh.m.gpuEccCorrectMMHUB.With(labels).Set(float64(stats.MMHUBCorrectableErrors))
	mh.m.gpuEccUncorrectMMHUB.With(labels).Set(float64(stats.MMHUBUncorrectableErrors))
	mh.m.gpuEccCorrectATHUB.With(labels).Set(float64(stats.ATHUBCorrectableErrors))
	mh.m.gpuEccUncorrectATHUB.With(labels).Set(float64(stats.ATHUBUncorrectableErrors))

	mh.m.gpuEccCorrectBIF.With(labels).Set(float64(stats.BIFCorrectableErrors))
	mh.m.gpuEccUncorrectBIF.With(labels).Set(float64(stats.BIFUncorrectableErrors))
	mh.m.gpuEccCorrectHDP.With(labels).Set(float64(stats.HDPCorrectableErrors))
	mh.m.gpuEccUncorrectHDP.With(labels).Set(float64(stats.HDPUncorrectableErrors))
	mh.m.gpuEccCorrectXgmiWAFL.With(labels).Set(float64(stats.XGMIWAFLCorrectableErrors))
	mh.m.gpuEccUncorrectXgmiWAFL.With(labels).Set(float64(stats.XGMIWAFLUncorrectableErrors))
	mh.m.gpuEccCorrectDF.With(labels).Set(float64(stats.DFCorrectableErrors))
	mh.m.gpuEccUncorrectDF.With(labels).Set(float64(stats.DFUncorrectableErrors))
	mh.m.gpuEccCorrectSMN.With(labels).Set(float64(stats.SMNCorrectableErrors))
	mh.m.gpuEccUncorrectSMN.With(labels).Set(float64(stats.SMNUncorrectableErrors))
	mh.m.gpuEccCorrectSEM.With(labels).Set(float64(stats.SEMCorrectableErrors))
	mh.m.gpuEccUncorrectSEM.With(labels).Set(float64(stats.SEMUncorrectableErrors))

	mh.m.gpuEccCorrectMP0.With(labels).Set(float64(stats.MP0CorrectableErrors))
	mh.m.gpuEccUncorrectMP0.With(labels).Set(float64(stats.MP0UncorrectableErrors))
	mh.m.gpuEccCorrectMP1.With(labels).Set(float64(stats.MP1CorrectableErrors))
	mh.m.gpuEccUncorrectMP1.With(labels).Set(float64(stats.MP1UncorrectableErrors))
	mh.m.gpuEccCorrectFUSE.With(labels).Set(float64(stats.FUSECorrectableErrors))
	mh.m.gpuEccUncorrectFUSE.With(labels).Set(float64(stats.FUSEUncorrectableErrors))
	mh.m.gpuEccCorrectUMC.With(labels).Set(float64(stats.UMCCorrectableErrors))
	mh.m.gpuEccUncorrectUMC.With(labels).Set(float64(stats.UMCUncorrectableErrors))

	mh.m.xgmiNbrNopTx0.With(labels).Set(float64(stats.XGMINeighbor0TxNOPs))
	mh.m.xgmiNbrReqTx0.With(labels).Set(float64(stats.XGMINeighbor0TxRequests))
	mh.m.xgmiNbrRespTx0.With(labels).Set(float64(stats.XGMINeighbor0TxResponses))
	mh.m.xgmiNbrBeatsTx0.With(labels).Set(float64(stats.XGMINeighbor0TXBeats))

	mh.m.xgmiNbrNopTx1.With(labels).Set(float64(stats.XGMINeighbor1TxNOPs))
	mh.m.xgmiNbrReqTx1.With(labels).Set(float64(stats.XGMINeighbor1TxRequests))
	mh.m.xgmiNbrRespTx1.With(labels).Set(float64(stats.XGMINeighbor1TxResponses))
	mh.m.xgmiNbrBeatsTx1.With(labels).Set(float64(stats.XGMINeighbor1TXBeats))

	mh.m.xgmiNbrTxTput0.With(labels).Set(float64(stats.XGMINeighbor0TxThroughput))
	mh.m.xgmiNbrTxTput1.With(labels).Set(float64(stats.XGMINeighbor1TxThroughput))
	mh.m.xgmiNbrTxTput2.With(labels).Set(float64(stats.XGMINeighbor2TxThroughput))
	mh.m.xgmiNbrTxTput3.With(labels).Set(float64(stats.XGMINeighbor3TxThroughput))
	mh.m.xgmiNbrTxTput4.With(labels).Set(float64(stats.XGMINeighbor4TxThroughput))
	mh.m.xgmiNbrTxTput5.With(labels).Set(float64(stats.XGMINeighbor5TxThroughput))
}

// parallel update for each gpu metrics. metrics package is atomic and all the
// entries are unique
func (mh *MetricsHandler) UpdateGPUInfoToMetrics(resp *amdgpu.GPUGetResponse) {
	var wg sync.WaitGroup
	for _, gpu := range resp.Response {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mh.updateGPUInfoToMetrics(gpu)
		}()
	}
	wg.Wait()
}
