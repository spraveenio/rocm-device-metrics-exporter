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

package gpuagent

import (
	"sync"

	"github.com/ROCm/device-metrics-exporter/pkg/exporter/logger"
	"github.com/ROCm/device-metrics-exporter/pkg/exporter/utils"
	"github.com/prometheus/client_golang/prometheus"
)

type fieldLogger struct {
	unsupportedFieldMap map[string]bool
	sync.RWMutex
}

func NewFieldLogger() *fieldLogger {
	return &fieldLogger{
		unsupportedFieldMap: make(map[string]bool),
		RWMutex:             sync.RWMutex{},
	}
}

func (fl *fieldLogger) checkUnsupportedFields(fieldName string) bool {
	fl.RLock()
	defer fl.RUnlock()
	if fl.unsupportedFieldMap == nil {
		return false
	}
	_, exists := fl.unsupportedFieldMap[fieldName]
	return exists
}

// logUnsupportedField logs the unsupported field name
// and adds it to the map of unsupported fields
// to avoid logging it again
func (fl *fieldLogger) logUnsupportedField(fieldName string) {
	fl.Lock()
	defer fl.Unlock()
	if fl.unsupportedFieldMap == nil {
		fl.unsupportedFieldMap = make(map[string]bool)
	}
	if _, exists := fl.unsupportedFieldMap[fieldName]; !exists {
		logger.Log.Printf("Platform doesn't support field name: %s", fieldName)
		fl.unsupportedFieldMap[fieldName] = true
	}
}

func (fl *fieldLogger) logWithValidateAndExport(metrics prometheus.GaugeVec, fieldName string,
	labels map[string]string, value interface{}) {

	if fl.checkUnsupportedFields(fieldName) {
		return
	}
	err := utils.ValidateAndExport(metrics, fieldName, labels, value)
	if err != utils.ErrorNone {
		if err == utils.ErrorNotApplicable {
			fl.logUnsupportedField(fieldName)
		} else {
			logger.Log.Printf("Failed to export metric %s: %v", fieldName, err)
		}
	}
}
