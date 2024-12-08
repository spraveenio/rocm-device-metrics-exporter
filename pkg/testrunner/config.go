/*
*
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
*
*/

package testrunner

import (
	"encoding/json"
	"os"

	types "github.com/pensando/device-metrics-exporter/pkg/testrunner/interface"
)

// TestTriggerCondition defines various types of condition to trigger the test
type TestTriggerCondition string

const (
	// UnhealthyGPU when GPU turned to unhealthy reported by exporter
	UnhealthyGPU TestTriggerCondition = "unhealthy_gpu"
	// PreJobCheck when test runner needs to do a pre-check before starting the job
	PreJobCheck TestTriggerCondition = "pre_job_check"
)

// TestConfig per trigger's test config, including test config name and related parameters
type TestConfig struct {
	// Name could be pre-defined test config: level-1, level-2, level-3
	// could also be user customized config name
	Name string `json:"name,omitempty"`
	// Parameters more specific parameters for the test run
	Parameters types.TestParams `json:"parameters,omitempty"`
}

// TestRunnerConfig the comprehensive config struct for test runner
type TestRunnerConfig struct {
	// RunnerLogPath path for storing the runner's log
	RunnerLogPath string `json:"runner_log_path,omitempty"`
	// ResultLogDir directory for saving test log and result
	ResultLogDir string `json:"result_log_dir,omitempty"`
	// StatusDBPath path for saving a tiny db to maintain the status of running test
	StatusDBPath string `json:"status_db_path,omitempty"`
	// GPUTestTriggers key is the TestTriggerCondition and value is per trigger's test config
	GPUTestTriggers map[TestTriggerCondition]TestConfig `json:"gpu_test_triggers,omitempty"`
}

// TestRunnerStatus saves the testrunner running status
// 1. saved into a temporary file as db
// 2. record the currently ongoing test and related device IDs
// 3. avoid running multiple test on the same device
// 4. when test runner restart, read the status db file and rerun interruptted test
type TestRunnerStatus struct {
	// Status key is the device ID and value is the test config file name
	Status map[string]string `json:"status"`
}

func SaveRunnerStatus(statusObj *TestRunnerStatus, path string) error {
	data, err := json.Marshal(statusObj)
	if err != nil {
		return err
	}
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func LoadRunnerStatus(path string) (*TestRunnerStatus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var status TestRunnerStatus
	err = json.Unmarshal(data, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}
