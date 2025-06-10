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

package metricsutil

import (
	"fmt"
	"testing"

	"github.com/ROCm/device-metrics-exporter/pkg/exporter/gen/exportermetrics"
	"gotest.tools/assert"
)

func TestMetricsHandler(t *testing.T) {
	teardownSuite := setupTest(t)
	defer teardownSuite(t)

	// expected test suite config default amd string
	prefix := mh.GetPrefix()
	assert.Equal(t, prefix, "amd", fmt.Sprintf("expected configured prefix amd but got %v", prefix))

	// udpate prefix to invalid prefix
	invalidPrefixList := []string{"amd-", "-amd"}
	for _, ipre := range invalidPrefixList {
		invalidPrefixConfig := &exportermetrics.MetricConfig{
			CommonConfig: &exportermetrics.CommonConfig{
				MetricsFieldPrefix: ipre,
			},
		}
		UpdateConfFile(t, invalidPrefixConfig)
		newPref := mh.GetPrefix()
		assert.Equal(t, newPref, "", fmt.Sprintf("expected empty prefix but got %v", newPref))
	}

	// Test with valid prefixes
	validPrefixList := []string{"amd", "gpu", "metrics"}
	for _, vpre := range validPrefixList {
		validPrefixConfig := &exportermetrics.MetricConfig{
			CommonConfig: &exportermetrics.CommonConfig{
				MetricsFieldPrefix: vpre,
			},
		}
		UpdateConfFile(t, validPrefixConfig)
		newPref := mh.GetPrefix()
		assert.Equal(t, newPref, vpre, fmt.Sprintf("expected prefix %v but got %v", vpre, newPref))
	}

	// Test with empty prefix
	emptyPrefixConfig := &exportermetrics.MetricConfig{
		CommonConfig: &exportermetrics.CommonConfig{
			MetricsFieldPrefix: "",
		},
	}
	UpdateConfFile(t, emptyPrefixConfig)
	newPref := mh.GetPrefix()
	assert.Equal(t, newPref, "", fmt.Sprintf("expected empty prefix but got %v", newPref))

	// Test with prefix containing only invalid characters
	invalidOnlyPrefixList := []string{"-", "--", "?", "#", "@"}
	for _, ipre := range invalidOnlyPrefixList {
		invalidPrefixConfig := &exportermetrics.MetricConfig{
			CommonConfig: &exportermetrics.CommonConfig{
				MetricsFieldPrefix: ipre,
			},
		}
		UpdateConfFile(t, invalidPrefixConfig)
		newPref := mh.GetPrefix()
		assert.Equal(t, newPref, "", fmt.Sprintf("expected empty prefix but got %v", newPref))
	}
}
