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

package gpuagent

import (
	"context"
	"fmt"
	"github.com/pensando/device-metrics-exporter/internal/k8s"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/amdgpu"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/gpumetrics"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/parserutil"
	"github.com/prometheus/client_golang/prometheus"
)

// local variables
var (
	mandatoryLables = []string{
		gpumetrics.GPUMetricLabel_GPU_UUID.String(),
		gpumetrics.GPUMetricLabel_SERIAL_NUMBER.String(),
	}
	exportLables    map[string]bool
	exportFieldMap  map[string]bool
	fieldMetricsMap []prometheus.Collector
	gpuSelectorMap  map[int]bool
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

func (ga *GPUAgentClient) GetExportLabels() []string {
	labelList := []string{}
	for key, enabled := range exportLables {
		if !enabled {
			continue
		}
		labelList = append(labelList, key)
	}
	return labelList
}

func (ga *GPUAgentClient) initLableConfigs(config *gpumetrics.GPUMetricConfig) {
	k8sLabels := map[string]bool{
		gpumetrics.GPUMetricLabel_POD.String():       true,
		gpumetrics.GPUMetricLabel_NAMESPACE.String(): true,
		gpumetrics.GPUMetricLabel_CONTAINER.String(): true,
		// todo: include gpu index
	}

	// list of mandatory labels
	exportLables = make(map[string]bool)
	for _, name := range gpumetrics.GPUMetricLabel_name {
		if _, ok := k8sLabels[name]; ok && !ga.isKubernetes {
			continue
		}
		exportLables[name] = false
	}
	// only mandatory labels are set for default
	for _, name := range mandatoryLables {
		exportLables[name] = true
	}

	if config != nil {
		for _, name := range config.GetLabels() {
			name = strings.ToUpper(name)
			if _, ok := exportLables[name]; ok {
				logger.Log.Printf("label %v enabled", name)
				exportLables[name] = true
			}
		}
	}
}

func initGPUSelectorConfig(config *gpumetrics.GPUMetricConfig) {
	if config != nil && config.GetSelector() != "" {
		selector := config.GetSelector()
		indices, err := parserutil.RangeStrToIntIndices(selector)
		if err != nil {
			logger.Log.Printf("GPUConfig.Selector parsing err :%v", err)
			logger.Log.Printf("monitoring all gpu instances")
			return
		}
		for _, ins := range indices {
			gpuSelectorMap[ins] = true
		}
	}
}

func initFieldConfig(config *gpumetrics.GPUMetricConfig) {
	exportFieldMap = make(map[string]bool)
	// setup metric fields in map to be monitored
	// init the map with all supported strings from enum
	enable_default := true
	if config != nil && len(config.GetFields()) != 0 {
		enable_default = false
	}
	for _, name := range gpumetrics.GPUMetricField_name {
		logger.Log.Printf("%v set to %v", name, enable_default)
		exportFieldMap[name] = enable_default
	}
	if config == nil || len(config.GetFields()) == 0 {
		return
	}
	for _, fieldName := range config.GetFields() {
		fieldName = strings.ToUpper(fieldName)
		if _, ok := exportFieldMap[fieldName]; ok {
			logger.Log.Printf("%v enabled", fieldName)
			exportFieldMap[fieldName] = true
		}
	}
	return
}

func (ga *GPUAgentClient) initFieldMetricsMap() {
	// must follow index mapping to fields.proto (GPUMetricField)
	fieldMetricsMap = []prometheus.Collector{
		ga.m.gpuNodesTotal,
		ga.m.gpuFanSpeed,
		ga.m.gpuAvgPkgPower,
		ga.m.gpuEdgeTemp,
		ga.m.gpuJunctionTemp,
		ga.m.gpuMemoryTemp,
		ga.m.gpuHBMTemp,
		ga.m.gpuUsage,
		ga.m.gpuGFXActivity,
		ga.m.gpuMemUsage,
		ga.m.gpuMemActivity,
		ga.m.gpuVoltage,
		ga.m.gpuPCIeBandwidth,
		ga.m.gpuEnergeyConsumed,
		ga.m.gpuPCIeReplayCount,
		ga.m.gpuClock,
		ga.m.gpuMemoryClock,
		ga.m.gpuPCIeTxUsage,
		ga.m.gpuPCIeRxUsage,
		ga.m.gpuPowerUsage,
		ga.m.gpuTotalMemory,
		ga.m.gpuEccCorrectTotal,
		ga.m.gpuEccUncorrectTotal,
		ga.m.gpuEccCorrectSDMA,
		ga.m.gpuEccUncorrectSDMA,
		ga.m.gpuEccCorrectGFX,
		ga.m.gpuEccUncorrectGFX,
		ga.m.gpuEccCorrectMMHUB,
		ga.m.gpuEccUncorrectMMHUB,
		ga.m.gpuEccCorrectATHUB,
		ga.m.gpuEccUncorrectATHUB,
		ga.m.gpuEccCorrectBIF,
		ga.m.gpuEccUncorrectBIF,
		ga.m.gpuEccCorrectHDP,
		ga.m.gpuEccUncorrectHDP,
		ga.m.gpuEccCorrectXgmiWAFL,
		ga.m.gpuEccUncorrectXgmiWAFL,
		ga.m.gpuEccCorrectDF,
		ga.m.gpuEccUncorrectDF,
		ga.m.gpuEccCorrectSMN,
		ga.m.gpuEccUncorrectSMN,
		ga.m.gpuEccCorrectSEM,
		ga.m.gpuEccUncorrectSEM,
		ga.m.gpuEccCorrectMP0,
		ga.m.gpuEccUncorrectMP0,
		ga.m.gpuEccCorrectMP1,
		ga.m.gpuEccUncorrectMP1,
		ga.m.gpuEccCorrectFUSE,
		ga.m.gpuEccUncorrectFUSE,
		ga.m.gpuEccCorrectUMC,
		ga.m.gpuEccUncorrectUMC,
		ga.m.xgmiNbrNopTx0,
		ga.m.xgmiNbrReqTx0,
		ga.m.xgmiNbrRespTx0,
		ga.m.xgmiNbrBeatsTx0,
		ga.m.xgmiNbrNopTx1,
		ga.m.xgmiNbrReqTx1,
		ga.m.xgmiNbrRespTx1,
		ga.m.xgmiNbrBeatsTx1,
		ga.m.xgmiNbrTxTput0,
		ga.m.xgmiNbrTxTput1,
		ga.m.xgmiNbrTxTput2,
		ga.m.xgmiNbrTxTput3,
		ga.m.xgmiNbrTxTput4,
		ga.m.xgmiNbrTxTput5,
	}

}

func (ga *GPUAgentClient) initPrometheusMetrics() {
	labels := ga.GetExportLabels()
	labelsWithIndex := append(labels, "hbm_index")
	ga.m = &metrics{
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
	ga.initFieldMetricsMap()
}

func (ga *GPUAgentClient) initFieldRegistration() error {
	for field, enabled := range exportFieldMap {
		if !enabled {
			continue
		}
		fieldIndex, ok := gpumetrics.GPUMetricField_value[field]
		if !ok {
			logger.Log.Printf("Invalid field %v, ignored", field)
			continue
		}
		ga.mh.GetRegistry().MustRegister(fieldMetricsMap[fieldIndex])
	}

	return nil
}

func (ga *GPUAgentClient) InitConfigs() error {
	filedConfigs := ga.mh.GetMetricsConfig()
	ga.initLableConfigs(filedConfigs)
	initFieldConfig(filedConfigs)
	initGPUSelectorConfig(filedConfigs)
	ga.initPrometheusMetrics()
	return ga.initFieldRegistration()
}

func getGPUInstanceID(gpu *amdgpu.GPU) int {
	return int(gpu.Status.Index)
}

func (ga *GPUAgentClient) UpdateStaticMetrics() error {
	// send the req to gpuclient
	resp, err := ga.getMetrics()
	if err != nil {
		logger.Log.Printf("err :%v", err)
		return err
	}
	if resp != nil && resp.ApiStatus != 0 {
		logger.Log.Printf("resp status :%v", resp.ApiStatus)
		return fmt.Errorf("%v", resp.ApiStatus)
	}
	for i, gpu := range resp.Response {
		logger.Log.Printf("GPU[%v].Status :%+v", i, gpu.Status)
	}

	ga.m.gpuNodesTotal.Set(float64(len(resp.Response)))
	for _, gpu := range resp.Response {
		if !ga.exporterEnabledGPU(getGPUInstanceID(gpu)) {
			continue
		}
		status := gpu.Status
		labels := ga.populateLabelsFromGPU(gpu)

		ga.m.gpuTotalMemory.With(labels).Set(float64(status.TotalMemory))
	}
	return nil
}

func (ga *GPUAgentClient) UpdateMetricsStats() error {
	// send the req to gpuclient
	res, err := ga.getMetrics()
	if err != nil {
		logger.Log.Printf("err :%v", err)
		return err
	}
	if res != nil && res.ApiStatus != 0 {
		logger.Log.Printf("resp status :%v", res.ApiStatus)
		return fmt.Errorf("%v", res.ApiStatus)
	}
	ga.updateGPUToMetrics(res)
	return nil
}

func (ga *GPUAgentClient) populateLabelsFromGPU(gpu *amdgpu.GPU) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	var podInfo k8s.PodResourceInfo

	if ga.isKubernetes {
		if pods, err := ga.kubeClient.ListPods(ctx); err == nil {
			podInfo = pods[strings.ToLower(gpu.Status.PCIeBusId)]
		} else {
			logger.Log.Printf("failed to list pod resources, %v", err)
			// continue
		}
	}

	labels := make(map[string]string)

	for key, enabled := range exportLables {
		if !enabled {
			continue
		}
		switch key {
		case gpumetrics.GPUMetricLabel_GPU_UUID.String():
			uuid, _ := uuid.FromBytes(gpu.Spec.Id)
			labels[key] = uuid.String()
		case gpumetrics.GPUMetricLabel_POD.String():
			labels[key] = podInfo.Pod
		case gpumetrics.GPUMetricLabel_NAMESPACE.String():
			labels[key] = podInfo.Namespace
		case gpumetrics.GPUMetricLabel_CONTAINER.String():
			labels[key] = podInfo.Container
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

func (ga *GPUAgentClient) exporterEnabledGPU(instance int) bool {
	if gpuSelectorMap == nil {
		return true
	}
	_, enabled := gpuSelectorMap[instance]
	return enabled

}

func (ga *GPUAgentClient) updateGPUInfoToMetrics(gpu *amdgpu.GPU) {
	if !ga.exporterEnabledGPU(getGPUInstanceID(gpu)) {
		return
	}

	labels := ga.populateLabelsFromGPU(gpu)
	labelsWithIndex := ga.populateLabelsFromGPU(gpu)
	stats := gpu.Stats
	ga.m.gpuFanSpeed.With(labels).Set(float64(stats.FanSpeed))
	ga.m.gpuAvgPkgPower.With(labels).Set(float64(stats.AvgPackagePower))

	// gpu temp stats
	tempStats := stats.Temperature
	if tempStats != nil {
		ga.m.gpuEdgeTemp.With(labels).Set(float64(tempStats.EdgeTemperature))
		ga.m.gpuJunctionTemp.With(labels).Set(float64(tempStats.JunctionTemperature))
		ga.m.gpuMemoryTemp.With(labels).Set(float64(tempStats.MemoryTemperature))
		for j, temp := range tempStats.HBMTemperature {
			labelsWithIndex["hbm_index"] = fmt.Sprintf("%v", j)
			ga.m.gpuHBMTemp.With(labelsWithIndex).Set(float64(temp))
		}
	}

	// gpu usage
	gpuUsage := stats.Usage
	if gpuUsage != nil {
		ga.m.gpuUsage.With(labels).Set(float64(gpuUsage.Usage))
		ga.m.gpuGFXActivity.With(labels).Set(float64(gpuUsage.GFXActivity))
	}

	// gpu memory usage
	memUsage := stats.MemoryUsage
	if memUsage != nil {
		ga.m.gpuMemUsage.With(labels).Set(float64(memUsage.MemoryUsage))
		ga.m.gpuMemActivity.With(labels).Set(float64(memUsage.Activity))
	}

	ga.m.gpuVoltage.With(labels).Set(float64(stats.Voltage))

	// pcie stats
	ga.m.gpuPCIeBandwidth.With(labels).Set(float64(stats.PCIeBandwidth))
	ga.m.gpuEnergeyConsumed.With(labels).Set(float64(stats.EnergyConsumed))
	ga.m.gpuPCIeReplayCount.With(labels).Set(float64(stats.PCIeReplayCount))

	// clock stats
	ga.m.gpuClock.With(labels).Set(float64(stats.GPUClock))
	ga.m.gpuMemoryClock.With(labels).Set(float64(stats.MemoryClock))

	// pcie usage
	ga.m.gpuPCIeTxUsage.With(labels).Set(float64(stats.PCIeTxUsage))
	ga.m.gpuPCIeRxUsage.With(labels).Set(float64(stats.PCIeRxUsage))

	ga.m.gpuPowerUsage.With(labels).Set(float64(stats.PowerUsage))

	ga.m.gpuEccCorrectTotal.With(labels).Set(float64(stats.TotalCorrectableErrors))
	ga.m.gpuEccUncorrectTotal.With(labels).Set(float64(stats.TotalUncorrectableErrors))
	ga.m.gpuEccCorrectSDMA.With(labels).Set(float64(stats.SDMACorrectableErrors))
	ga.m.gpuEccUncorrectSDMA.With(labels).Set(float64(stats.SDMAUncorrectableErrors))
	ga.m.gpuEccCorrectGFX.With(labels).Set(float64(stats.GFXCorrectableErrors))
	ga.m.gpuEccUncorrectGFX.With(labels).Set(float64(stats.GFXUncorrectableErrors))
	ga.m.gpuEccCorrectMMHUB.With(labels).Set(float64(stats.MMHUBCorrectableErrors))
	ga.m.gpuEccUncorrectMMHUB.With(labels).Set(float64(stats.MMHUBUncorrectableErrors))
	ga.m.gpuEccCorrectATHUB.With(labels).Set(float64(stats.ATHUBCorrectableErrors))
	ga.m.gpuEccUncorrectATHUB.With(labels).Set(float64(stats.ATHUBUncorrectableErrors))

	ga.m.gpuEccCorrectBIF.With(labels).Set(float64(stats.BIFCorrectableErrors))
	ga.m.gpuEccUncorrectBIF.With(labels).Set(float64(stats.BIFUncorrectableErrors))
	ga.m.gpuEccCorrectHDP.With(labels).Set(float64(stats.HDPCorrectableErrors))
	ga.m.gpuEccUncorrectHDP.With(labels).Set(float64(stats.HDPUncorrectableErrors))
	ga.m.gpuEccCorrectXgmiWAFL.With(labels).Set(float64(stats.XGMIWAFLCorrectableErrors))
	ga.m.gpuEccUncorrectXgmiWAFL.With(labels).Set(float64(stats.XGMIWAFLUncorrectableErrors))
	ga.m.gpuEccCorrectDF.With(labels).Set(float64(stats.DFCorrectableErrors))
	ga.m.gpuEccUncorrectDF.With(labels).Set(float64(stats.DFUncorrectableErrors))
	ga.m.gpuEccCorrectSMN.With(labels).Set(float64(stats.SMNCorrectableErrors))
	ga.m.gpuEccUncorrectSMN.With(labels).Set(float64(stats.SMNUncorrectableErrors))
	ga.m.gpuEccCorrectSEM.With(labels).Set(float64(stats.SEMCorrectableErrors))
	ga.m.gpuEccUncorrectSEM.With(labels).Set(float64(stats.SEMUncorrectableErrors))

	ga.m.gpuEccCorrectMP0.With(labels).Set(float64(stats.MP0CorrectableErrors))
	ga.m.gpuEccUncorrectMP0.With(labels).Set(float64(stats.MP0UncorrectableErrors))
	ga.m.gpuEccCorrectMP1.With(labels).Set(float64(stats.MP1CorrectableErrors))
	ga.m.gpuEccUncorrectMP1.With(labels).Set(float64(stats.MP1UncorrectableErrors))
	ga.m.gpuEccCorrectFUSE.With(labels).Set(float64(stats.FUSECorrectableErrors))
	ga.m.gpuEccUncorrectFUSE.With(labels).Set(float64(stats.FUSEUncorrectableErrors))
	ga.m.gpuEccCorrectUMC.With(labels).Set(float64(stats.UMCCorrectableErrors))
	ga.m.gpuEccUncorrectUMC.With(labels).Set(float64(stats.UMCUncorrectableErrors))

	ga.m.xgmiNbrNopTx0.With(labels).Set(float64(stats.XGMINeighbor0TxNOPs))
	ga.m.xgmiNbrReqTx0.With(labels).Set(float64(stats.XGMINeighbor0TxRequests))
	ga.m.xgmiNbrRespTx0.With(labels).Set(float64(stats.XGMINeighbor0TxResponses))
	ga.m.xgmiNbrBeatsTx0.With(labels).Set(float64(stats.XGMINeighbor0TXBeats))

	ga.m.xgmiNbrNopTx1.With(labels).Set(float64(stats.XGMINeighbor1TxNOPs))
	ga.m.xgmiNbrReqTx1.With(labels).Set(float64(stats.XGMINeighbor1TxRequests))
	ga.m.xgmiNbrRespTx1.With(labels).Set(float64(stats.XGMINeighbor1TxResponses))
	ga.m.xgmiNbrBeatsTx1.With(labels).Set(float64(stats.XGMINeighbor1TXBeats))

	ga.m.xgmiNbrTxTput0.With(labels).Set(float64(stats.XGMINeighbor0TxThroughput))
	ga.m.xgmiNbrTxTput1.With(labels).Set(float64(stats.XGMINeighbor1TxThroughput))
	ga.m.xgmiNbrTxTput2.With(labels).Set(float64(stats.XGMINeighbor2TxThroughput))
	ga.m.xgmiNbrTxTput3.With(labels).Set(float64(stats.XGMINeighbor3TxThroughput))
	ga.m.xgmiNbrTxTput4.With(labels).Set(float64(stats.XGMINeighbor4TxThroughput))
	ga.m.xgmiNbrTxTput5.With(labels).Set(float64(stats.XGMINeighbor5TxThroughput))
}

// parallel update for each gpu metrics. metrics package is atomic and all the
// entries are unique
func (ga *GPUAgentClient) updateGPUToMetrics(resp *amdgpu.GPUGetResponse) {
	var wg sync.WaitGroup
	for _, gpu := range resp.Response {
		wg.Add(1)
		go func(gpu *amdgpu.GPU) {
			defer wg.Done()
			ga.updateGPUInfoToMetrics(gpu)
		}(gpu)
	}
	wg.Wait()
}
