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

package nicagent

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ROCm/device-metrics-exporter/pkg/exporter/gen/exportermetrics"
	"github.com/ROCm/device-metrics-exporter/pkg/exporter/globals"
	"github.com/ROCm/device-metrics-exporter/pkg/exporter/logger"
	"github.com/ROCm/device-metrics-exporter/pkg/exporter/scheduler"
	"github.com/prometheus/client_golang/prometheus"
)

// local variables
var (
	mandatoryLables = []string{
		exportermetrics.NICMetricLabel_NIC_ID.String(),
		exportermetrics.NICMetricLabel_NIC_SERIAL_NUMBER.String(),
		exportermetrics.NICMetricLabel_NIC_HOSTNAME.String(),
	}
	exportLabels    map[string]bool
	exportFieldMap  map[string]bool
	fieldMetricsMap []prometheus.Collector
	customLabelMap  map[string]string
)

type metrics struct {
	nicNodesTotal prometheus.Gauge
	nicMaxSpeed   prometheus.GaugeVec

	// Port stats
	nicPortStatsFramesRxBadFcs       prometheus.GaugeVec
	nicPortStatsFramesRxBadAll       prometheus.GaugeVec
	nicPortStatsFramesRxPause        prometheus.GaugeVec
	nicPortStatsFramesRxBadLength    prometheus.GaugeVec
	nicPortStatsFramesRxUndersized   prometheus.GaugeVec
	nicPortStatsFramesRxOversized    prometheus.GaugeVec
	nicPortStatsFramesRxFragments    prometheus.GaugeVec
	nicPortStatsFramesRxJabber       prometheus.GaugeVec
	nicPortStatsFramesRxPripause     prometheus.GaugeVec
	nicPortStatsFramesRxStompedCrc   prometheus.GaugeVec
	nicPortStatsFramesRxTooLong      prometheus.GaugeVec
	nicPortStatsFramesRxDropped      prometheus.GaugeVec
	nicPortStatsFramesTxBad          prometheus.GaugeVec
	nicPortStatsFramesTxPause        prometheus.GaugeVec
	nicPortStatsFramesTxPripause     prometheus.GaugeVec
	nicPortStatsFramesTxLessThan64b  prometheus.GaugeVec
	nicPortStatsFramesTxTruncated    prometheus.GaugeVec
	nicPortStatsRsfecCorrectableWord prometheus.GaugeVec
	nicPortStatsRsfecChSymbolErrCnt  prometheus.GaugeVec
}

func (na *NICAgentClient) ResetMetrics() error {
	// reset all label based fields
	na.m.nicMaxSpeed.Reset()
	na.m.nicPortStatsFramesRxBadFcs.Reset()
	na.m.nicPortStatsFramesRxBadAll.Reset()
	na.m.nicPortStatsFramesRxPause.Reset()
	na.m.nicPortStatsFramesRxBadLength.Reset()
	na.m.nicPortStatsFramesRxUndersized.Reset()
	na.m.nicPortStatsFramesRxOversized.Reset()
	na.m.nicPortStatsFramesRxFragments.Reset()
	na.m.nicPortStatsFramesRxJabber.Reset()
	na.m.nicPortStatsFramesRxPripause.Reset()
	na.m.nicPortStatsFramesRxStompedCrc.Reset()
	na.m.nicPortStatsFramesRxTooLong.Reset()
	na.m.nicPortStatsFramesRxDropped.Reset()
	na.m.nicPortStatsFramesTxBad.Reset()
	na.m.nicPortStatsFramesTxPause.Reset()
	na.m.nicPortStatsFramesTxPripause.Reset()
	na.m.nicPortStatsFramesTxLessThan64b.Reset()
	na.m.nicPortStatsFramesTxTruncated.Reset()
	na.m.nicPortStatsRsfecCorrectableWord.Reset()
	na.m.nicPortStatsRsfecChSymbolErrCnt.Reset()

	return nil
}

func (na *NICAgentClient) GetExportLabels() []string { //TODO .. move to exporter/utils
	labelList := []string{}
	for key, enabled := range exportLabels {
		if !enabled {
			continue
		}
		labelList = append(labelList, strings.ToLower(key))
	}

	for key := range customLabelMap {
		exists := false
		for _, label := range labelList {
			if key == label {
				exists = true
				break
			}
		}

		// Add only unique labels to export labels
		if !exists {
			labelList = append(labelList, strings.ToLower(key))
		}
	}

	logger.Log.Printf("Get_export-labels returns %v", labelList)
	return labelList
}

func (na *NICAgentClient) initLabelConfigs(config *exportermetrics.NICMetricConfig) {

	// list of mandatory labels
	exportLabels = make(map[string]bool)
	for _, name := range exportermetrics.NICMetricLabel_name {
		exportLabels[name] = false
	}
	// only mandatory labels are set for default
	for _, name := range mandatoryLables {
		exportLabels[name] = true
	}

	k8sLabels := scheduler.GetExportLabels(scheduler.Kubernetes)

	if config != nil {
		for _, name := range config.GetLabels() {
			name = strings.ToUpper(name)
			if _, ok := exportLabels[name]; ok {
				// export labels must have atleast one label exported by
				// kubernets client, otherwise don't enable the label
				if _, ok := k8sLabels[name]; ok && !na.isKubernetes {
					continue
				}

				logger.Log.Printf("label %v enabled", name)
				exportLabels[name] = true
			}
		}
	}
	logger.Log.Printf("export-labels updated to %v", exportLabels)
}

func (na *NICAgentClient) initCustomLabels(config *exportermetrics.NICMetricConfig) {
	customLabelMap = make(map[string]string)
	if config != nil && config.GetCustomLabels() != nil {
		cl := config.GetCustomLabels()
		labelCount := 0

		for l, value := range cl {
			if labelCount >= globals.MaxSupportedCustomLabels {
				logger.Log.Printf("Max custom labels supported: %v, ignoring extra labels.", globals.MaxSupportedCustomLabels)
				break
			}
			label := strings.ToLower(l)

			// Check if custom label is a mandatory label, ignore if true
			found := false
			for _, mlabel := range mandatoryLables {
				if strings.ToLower(mlabel) == label {
					logger.Log.Printf("Detected mandatory label %s in custom label, ignoring...", mlabel)
					found = true
					break
				}
			}
			if found {
				continue
			}

			// Store all custom labels
			customLabelMap[label] = value
			labelCount++
		}
	}
	logger.Log.Printf("custom labels being exported: %v", customLabelMap)
}

func (na *NICAgentClient) initFieldConfig(config *exportermetrics.NICMetricConfig) {
	exportFieldMap = make(map[string]bool)
	// setup metric fields in map to be monitored
	// init the map with all supported strings from enum
	enable_default := true
	if config != nil && len(config.GetFields()) != 0 {
		enable_default = false
	}
	for _, name := range exportermetrics.NICMetricField_name {
		exportFieldMap[name] = enable_default
	}
	if config == nil || len(config.GetFields()) == 0 {
		return
	}
	for _, fieldName := range config.GetFields() {
		fieldName = strings.ToUpper(fieldName)
		if _, ok := exportFieldMap[fieldName]; ok {
			exportFieldMap[fieldName] = true
		}
	}
	// print disabled short list
	for k, v := range exportFieldMap {
		if !v {
			logger.Log.Printf("%v field is disabled", k)
		}
	}
}

func (na *NICAgentClient) initFieldMetricsMap() {
	// must follow index mapping to fields.proto (NICMetricField)
	fieldMetricsMap = []prometheus.Collector{
		na.m.nicNodesTotal,
		na.m.nicMaxSpeed,
		na.m.nicPortStatsFramesRxBadFcs,
		na.m.nicPortStatsFramesRxBadAll,
		na.m.nicPortStatsFramesRxPause,
		na.m.nicPortStatsFramesRxBadLength,
		na.m.nicPortStatsFramesRxUndersized,
		na.m.nicPortStatsFramesRxOversized,
		na.m.nicPortStatsFramesRxFragments,
		na.m.nicPortStatsFramesRxJabber,
		na.m.nicPortStatsFramesRxPripause,
		na.m.nicPortStatsFramesRxStompedCrc,
		na.m.nicPortStatsFramesRxTooLong,
		na.m.nicPortStatsFramesRxDropped,
		na.m.nicPortStatsFramesTxBad,
		na.m.nicPortStatsFramesTxPause,
		na.m.nicPortStatsFramesTxPripause,
		na.m.nicPortStatsFramesTxLessThan64b,
		na.m.nicPortStatsFramesTxTruncated,
		na.m.nicPortStatsRsfecCorrectableWord,
		na.m.nicPortStatsRsfecChSymbolErrCnt,
	}
}

func (na *NICAgentClient) initPrometheusMetrics() {
	labels := na.GetExportLabels()
	na.m = &metrics{
		nicNodesTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_NODES_TOTAL.String()),
			Help: "Number of NICs in the node",
		}),

		nicMaxSpeed: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_MAX_SPEED.String()),
			Help: "Maximum NIC speed in Gbps",
		}, labels),

		nicPortStatsFramesRxBadFcs: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_BAD_FCS.String()),
			Help: "Bad frames received due to a Frame Check Sequence (FCS) error on a network port",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxBadAll: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_BAD_ALL.String()),
			Help: "Total number of frames received on a network port that are bad",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxPause: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_PAUSE.String()),
			Help: "Total number of pause frames received on a network port",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxBadLength: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_BAD_LENGTH.String()),
			Help: "Total number of frames received that have an incorrect or invalid length",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxUndersized: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_UNDERSIZED.String()),
			Help: "Total number of frames received that are smaller than the minimum frame size allowed by the network protocol",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxOversized: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_OVERSIZED.String()),
			Help: " Total number of frames received that exceed the maximum allowed size for the network protocol",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxFragments: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_FRAGMENTS.String()),
			Help: "Total number of frames received that are fragments of larger packets",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxJabber: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_JABBER.String()),
			Help: "Total number of frames received that are considered jabber frames",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxPripause: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_PRIPAUSE.String()),
			Help: "Total number of priority pause frames received",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxStompedCrc: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_STOMPED_CRC.String()),
			Help: "Total number of frames received that had a valid CRC (Cyclic Redundancy Check) but were stomped",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxTooLong: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_TOO_LONG.String()),
			Help: "Total number of frames received that exceed the maximum allowable size for frames on the network",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesRxDropped: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_RX_DROPPED.String()),
			Help: "Total number of frames that were received but dropped due to various reasons such as buffer overflows or hardware limitations",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesTxBad: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_TX_BAD.String()),
			Help: "Total number of transmitted frames that are considered bad",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesTxPause: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_TX_PAUSE.String()),
			Help: "Total number of pause frames transmitted",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesTxPripause: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_TX_PRIPAUSE.String()),
			Help: "Total number of priority pause frames transmitted",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesTxLessThan64b: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_TX_LESS_THAN_64B.String()),
			Help: "Total number of frames transmitted that are smaller than the minimum frame size i.e 64 bytes",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsFramesTxTruncated: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_FRAMES_TX_TRUNCATED.String()),
			Help: "Total number of frames that were transmitted but truncated",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsRsfecCorrectableWord: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_RSFEC_CORRECTABLE_WORD.String()),
			Help: "Total number of RS-FEC (Reed-Solomon Forward Error Correction) correctable words received or transmitted",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),

		nicPortStatsRsfecChSymbolErrCnt: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: strings.ToLower(exportermetrics.NICMetricField_NIC_PORT_STATS_RSFEC_CH_SYMBOL_ERR_CNT.String()),
			Help: "Total count of channel symbol errors detected by the RS-FEC (Reed-Solomon Forward Error Correction) mechanism.",
		}, append([]string{LabelPortName, LabelPortID}, labels...)),
	}
	na.initFieldMetricsMap()
}

func (na *NICAgentClient) initFieldRegistration() error {
	for field, enabled := range exportFieldMap {
		if !enabled {
			continue
		}
		fieldIndex, ok := exportermetrics.NICMetricField_value[field]
		if !ok {
			logger.Log.Printf("Invalid field %v, ignored", field)
			continue
		}
		na.mh.GetRegistry().MustRegister(fieldMetricsMap[fieldIndex])
	}

	return nil
}

func (na *NICAgentClient) InitConfigs() error {
	filedConfigs := na.mh.GetNICMetricsConfig()

	na.initCustomLabels(filedConfigs)
	na.initLabelConfigs(filedConfigs)
	na.initFieldConfig(filedConfigs)
	na.initPrometheusMetrics()
	return na.initFieldRegistration()
}

func (na *NICAgentClient) UpdateStaticMetrics() error {
	na.m.nicNodesTotal.Set(float64(1)) //TODO
	return nil
}

func (na *NICAgentClient) UpdateMetricsStats() error {
	return na.getMetricsAll()
}

func GetNICMandatoryLabels() []string {
	return mandatoryLables
}

func (na *NICAgentClient) GetNICCustomeLabels() map[string]string {
	return customLabelMap
}

func (na *NICAgentClient) populateLabelsFromNIC(UUID string) map[string]string {
	labels := make(map[string]string)

	nic, found := na.nics[UUID]
	if !found {
		logger.Log.Printf("could not find NIC: %s from the local cache", UUID)
		return labels
	}

	for ckey, enabled := range exportLabels {
		if !enabled {
			continue
		}
		key := strings.ToLower(ckey)
		switch ckey {
		case exportermetrics.NICMetricLabel_NIC_UUID.String():
			labels[key] = nic.UUID
		case exportermetrics.NICMetricLabel_NIC_ID.String():
			labels[key] = nic.Index
		case exportermetrics.NICMetricLabel_NIC_SERIAL_NUMBER.String():
			labels[key] = nic.SerialNumber
		case exportermetrics.NICMetricLabel_NIC_HOSTNAME.String():
			labels[key] = "ubuntu" //TODO
		default:
			logger.Log.Printf("Invalid label is ignored %v", key)
		}
	}

	// Add custom labels
	for label, value := range customLabelMap {
		labels[label] = value
	}
	return labels
}

// getNICs fetches all the static data that we need related to NIC including port
func (na *NICAgentClient) getNICs() (map[string]*NIC, error) {
	type Response struct {
		NIC []struct {
			ID           string `json:"id"`
			ProductName  string `json:"product_name"`
			SerialNumber string `json:"serial_number"`
			Port         []struct {
				Spec struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"spec"`
			} `json:"port"`
		} `json:"nic"`
	}

	nics := map[string]*NIC{}

	nicResp, err := exec.Command("/bin/bash", "-c", "nicctl show card --json").Output()
	if err != nil {
		logger.Log.Printf("failed to get nic data, err: %+v", err)
		return nics, err
	}
	var resp Response
	err = json.Unmarshal(nicResp, &resp)
	if err != nil {
		logger.Log.Printf("error unmarshalling nic data: %v", err)
		return nics, err
	}

	// fetch port details for each NIC
	for index, nic := range resp.NIC {
		nics[nic.ID] = &NIC{
			Index:        fmt.Sprintf("%v", index),
			UUID:         nic.ID,
			ProductName:  nic.ProductName,
			SerialNumber: nic.SerialNumber,
		}

		cmd := fmt.Sprintf("nicctl show port --card %s --json", nic.ID)
		portResp, err := exec.Command("/bin/bash", "-c", cmd).Output()
		if err != nil {
			logger.Log.Printf("NIC: %s, failed to get port data, err: %+v", nic.ID, err)
			return nics, err
		}
		var resp Response
		err = json.Unmarshal(portResp, &resp)
		if err != nil {
			logger.Log.Printf("NIC: %s, error unmarshalling port data: %v", nic.ID, err)
			return nics, err
		}

		for _, nic := range resp.NIC {
			nics[nic.ID].Ports = map[string]*Port{}
			for index, port := range nic.Port {
				nics[nic.ID].Ports[port.Spec.ID] = &Port{
					Index: fmt.Sprintf("%v", index),
					UUID:  port.Spec.ID,
					Name:  port.Spec.Name,
				}
			}
		}
	}

	return nics, nil
}
