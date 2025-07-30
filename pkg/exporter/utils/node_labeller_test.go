/*
Copyright (c) Advanced Micro Devices, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the \"License\");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an \"AS IS\" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"strings"
	"testing"

	"github.com/ROCm/device-metrics-exporter/pkg/amdnic/gen/nicmetricssvc"
	"github.com/ROCm/device-metrics-exporter/pkg/exporter/logger"
	"gotest.tools/assert"
)

func TestNewNodeHealthLabellerConfig(t *testing.T) {
	prefix := "test.prefix"
	cfg := NewNodeHealthLabellerConfig(prefix)
	assert.Assert(t, cfg != nil, "Expected non-nil config")
	assert.Equal(t, cfg.LabelPrefix, prefix, "Expected LabelPrefix to match input prefix")
}

func TestExtractDeviceID(t *testing.T) {
	logger.Init(true)
	cfg := NewNodeHealthLabellerConfig("test.prefix")

	tests := []struct {
		label       string
		expectedID  string
		description string
	}{
		{"test.prefix.device1.state", "device1", "Standard label"},
		{"test.prefix.dev-42.state", "dev-42", "Label with hyphen and number"},
		{"test.prefix.device.name.state", "device.name", "Label with dots in device ID"},
		{"test.prefix..state", "", "Empty device ID"},
		{"wrongprefix.device1.state", "", "Wrong prefix"},
		{"prefix.device1.statenot", "", "Label not ending with .state"},
		{"prefixdevice1.state", "", "Prefix without dot"},
		{"prefix.device1.state.extra", "", "Label with extra suffix"},
	}

	for _, test := range tests {
		got := cfg.extractDeviceID(test.label)
		assert.Equal(t, got, test.expectedID, test.description)
	}
}

func TestParseNodeHealthLabel(t *testing.T) {
	logger.Init(true)
	cfg := NewNodeHealthLabellerConfig("test.prefix")

	testCases := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "Standard labels",
			input: map[string]string{
				"test.prefix.nic1.state": strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),
				"test.prefix.nic2.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"test.prefix.nic3.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
			expected: map[string]string{
				"nic1": strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),
				"nic2": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"nic3": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
		},
		{
			name: "Labels with empty deviceID and unrelated labels",
			input: map[string]string{
				"test.prefix..state":     strings.ToLower(nicmetricssvc.Health_UNKNOWN.String()),
				"other.label":            "value",
				"test.prefix.nic4.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
			expected: map[string]string{
				"nic4": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
		},
		{
			name:     "Empty input",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "No matching labels",
			input: map[string]string{
				"unrelated.label": "foo",
				"another.one":     "bar",
			},
			expected: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := cfg.ParseNodeHealthLabel(tc.input)
			assert.Equal(t, len(got), len(tc.expected), "Expected map size to match")
			for k, v := range tc.expected {
				assert.Equal(t, got[k], v, "Expected state for deviceID %q to match", k)
			}
			// Ensure no empty deviceID key
			if _, exists := got[""]; exists {
				t.Error("Expected empty deviceID key to be skipped")
			}
		})
	}
}

func TestRemoveNodeHealthLabel(t *testing.T) {
	logger.Init(true)
	cfg := NewNodeHealthLabellerConfig("test.prefix")

	testCases := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "Remove only health labels",
			input: map[string]string{
				"test.prefix.dev1.state":  strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),
				"test.prefix.dev2.state":  strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"other.label":             "value",
				"test.prefix.dev3.state":  strings.ToLower(nicmetricssvc.Health_UNKNOWN.String()),
				"test.prefix.dev4.other":  "somevalue",
				"test.prefix.dev5.statex": "shouldstay",
			},
			expected: map[string]string{
				"other.label":             "value",
				"test.prefix.dev4.other":  "somevalue",
				"test.prefix.dev5.statex": "shouldstay",
			},
		},
		{
			name:     "No health labels present",
			input:    map[string]string{"foo": "bar", "baz": "qux"},
			expected: map[string]string{"foo": "bar", "baz": "qux"},
		},
		{
			name:     "Empty input",
			input:    map[string]string{},
			expected: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			labels := make(map[string]string)
			for k, v := range tc.input {
				labels[k] = v
			}
			cfg.RemoveNodeHealthLabel(labels)

			if len(labels) != len(tc.expected) {
				t.Errorf("Expected %d labels remaining, got %d", len(tc.expected), len(labels))
			}
			for k, v := range tc.expected {
				if val, ok := labels[k]; !ok || val != v {
					t.Errorf("Expected label %q with value %q, got %q", k, v, val)
				}
			}
			// Confirm no health labels remain
			for k := range labels {
				if strings.HasPrefix(k, cfg.LabelPrefix) && strings.HasSuffix(k, ".state") {
					t.Errorf("Health label %q should have been removed", k)
				}
			}
		})
	}
}

func TestAddNodeHealthLabel(t *testing.T) {
	logger.Init(true)
	cfg := NewNodeHealthLabellerConfig("test.prefix")

	testCases := []struct {
		name        string
		startLabels map[string]string
		healthMap   map[string]string
		expected    map[string]string
	}{
		{
			name: "Add unhealthy and unknown, skip healthy",
			startLabels: map[string]string{
				"existing.label": "foo",
			},
			healthMap: map[string]string{
				"dev1": strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),   // should be skipped
				"dev2": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()), // should be added
				"dev3": strings.ToLower(nicmetricssvc.Health_UNKNOWN.String()),   // should be added
				"dev4": strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),   // skipped
			},
			expected: map[string]string{
				"existing.label":         "foo",
				"test.prefix.dev2.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"test.prefix.dev3.state": strings.ToLower(nicmetricssvc.Health_UNKNOWN.String()),
			},
		},
		{
			name:        "Empty health map",
			startLabels: map[string]string{"label1": "val1"},
			healthMap:   map[string]string{},
			expected:    map[string]string{"label1": "val1"},
		},
		{
			name:        "All healthy devices",
			startLabels: map[string]string{},
			healthMap: map[string]string{
				"devA": strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),
				"devB": strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),
			},
			expected: map[string]string{},
		},
		{
			name:        "All unhealthy devices",
			startLabels: map[string]string{},
			healthMap: map[string]string{
				"devX": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"devY": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
			expected: map[string]string{
				"test.prefix.devX.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"test.prefix.devY.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nodeLabels := make(map[string]string)
			for k, v := range tc.startLabels {
				nodeLabels[k] = v
			}
			cfg.AddNodeHealthLabel(nodeLabels, tc.healthMap)

			assert.Equal(t, len(nodeLabels), len(tc.expected), "Expected label count to match")
			for k, v := range tc.expected {
				assert.Equal(t, nodeLabels[k], v, "Expected label %q to have value %q", k, v)
			}
			// Ensure no healthy device label is present
			for k, v := range nodeLabels {
				if strings.HasSuffix(k, ".state") && v == strings.ToLower(nicmetricssvc.Health_HEALTHY.String()) {
					t.Errorf("Label %q should not be present for healthy device", k)
				}
			}
		})
	}
}

func TestIntegration_AddRemoveParse(t *testing.T) {
	logger.Init(true)
	cfg := NewNodeHealthLabellerConfig("test.prefix")

	testCases := []struct {
		name           string
		healthMap      map[string]string
		expectedLabels map[string]string
		expectedParsed map[string]string
	}{
		{
			name: "Add unhealthy and unknown, skip healthy",
			healthMap: map[string]string{
				"dev1": strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),   // should be skipped
				"dev2": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()), // should be added
				"dev3": strings.ToLower(nicmetricssvc.Health_UNKNOWN.String()),   // should be added
			},
			expectedLabels: map[string]string{
				"test.prefix.dev2.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"test.prefix.dev3.state": strings.ToLower(nicmetricssvc.Health_UNKNOWN.String()),
			},
			expectedParsed: map[string]string{
				"dev2": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"dev3": strings.ToLower(nicmetricssvc.Health_UNKNOWN.String()),
			},
		},
		{
			name: "All healthy devices",
			healthMap: map[string]string{
				"devA": strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),
				"devB": strings.ToLower(nicmetricssvc.Health_HEALTHY.String()),
			},
			expectedLabels: map[string]string{},
			expectedParsed: map[string]string{},
		},
		{
			name: "All unhealthy devices",
			healthMap: map[string]string{
				"devX": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"devY": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
			expectedLabels: map[string]string{
				"test.prefix.devX.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"test.prefix.devY.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
			expectedParsed: map[string]string{
				"devX": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
				"devY": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
		},
		{
			name: "Mixed and empty device IDs",
			healthMap: map[string]string{
				"":     strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()), // should be skipped, as deviceID is empty
				"devZ": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
			expectedLabels: map[string]string{
				"test.prefix.devZ.state": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
			expectedParsed: map[string]string{
				"devZ": strings.ToLower(nicmetricssvc.Health_UNHEALTHY.String()),
			},
		},
		{
			name:           "Empty health map",
			healthMap:      map[string]string{},
			expectedLabels: map[string]string{},
			expectedParsed: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nodeLabels := map[string]string{}

			// Add
			cfg.AddNodeHealthLabel(nodeLabels, tc.healthMap)
			assert.Equal(t, len(nodeLabels), len(tc.expectedLabels), "Expected label count to match after add")
			for k, v := range tc.expectedLabels {
				assert.Equal(t, nodeLabels[k], v, "Expected label %q to have value %q after add", k, v)
			}

			// Parse
			parsed := cfg.ParseNodeHealthLabel(nodeLabels)
			assert.Equal(t, len(parsed), len(tc.expectedParsed), "Expected parsed map size to match")
			for k, v := range tc.expectedParsed {
				assert.Equal(t, parsed[k], v, "Expected parsed deviceID %q to have value %q", k, v)
			}

			// Remove
			cfg.RemoveNodeHealthLabel(nodeLabels)
			assert.Equal(t, len(nodeLabels), 0, "Expected all health labels to be removed after remove")
		})
	}
}
