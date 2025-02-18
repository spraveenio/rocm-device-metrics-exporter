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
}

func (na *NICAgentClient) ResetMetrics() error {
	// reset all label based fields
	na.m.nicMaxSpeed.Reset()
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

func initCustomLabels(config *exportermetrics.NICMetricConfig) {
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

func initFieldConfig(config *exportermetrics.NICMetricConfig) {
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
	}

}

func (na *NICAgentClient) initPrometheusMetrics() {
	labels := na.GetExportLabels()
	na.m = &metrics{
		nicNodesTotal: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ainic_nodes_total",
				Help: "Number of NICs in the node",
			},
		),
		nicMaxSpeed: *prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ainic_max_speed",
			Help: "Maximum NIC speed in Gbps",
		},
			labels),
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

	initCustomLabels(filedConfigs)
	na.initLabelConfigs(filedConfigs)
	initFieldConfig(filedConfigs)
	//initAinicSelectorConfig(filedConfigs)
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

func (na *NICAgentClient) populateLabelsFromNIC() map[string]string {

	labels := make(map[string]string)

	for ckey, enabled := range exportLabels {
		if !enabled {
			continue
		}
		key := strings.ToLower(ckey)
		switch ckey {
		case exportermetrics.NICMetricLabel_NIC_UUID.String():
			labels[key] = "42424650-4c32-3434-3530-304534000000" //TODO
		case exportermetrics.NICMetricLabel_NIC_ID.String():
			labels[key] = "8" //TODO
		case exportermetrics.NICMetricLabel_NIC_SERIAL_NUMBER.String():
			labels[key] = "FPL244500E4" //TODO
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

func GetNICAgentMandatoryLabels() []string {
	return mandatoryLables
}
