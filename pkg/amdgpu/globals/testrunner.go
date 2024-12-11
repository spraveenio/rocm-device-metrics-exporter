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

package globals

import "time"

const (
	// TestRunner related constants
	DefaultTestCategory      = "GPU_HEALTH_CHECK"
	DefaultTestTrigger       = "AUTO_UNHEALTHY_GPU_WATCH"
	GlobalTestTriggerKeyword = "global"

	// rvs binary path
	RVSPath = "/opt/rocm/bin/rvs"

	// rocm-smi binary path
	ROCmSMIPath = "/opt/rocm/bin/rocm-smi"

	// gpu test runner configuration file path
	AMDTestCaseDir = "/opt/rocm/share/rocm-validation-suite/conf/"

	// AMDTestRunnerCfgPath path to the test runner config
	AMDTestRunnerCfgPath = "/etc/test-runner/config.json"

	LogPrefix = "test-runner "

	GPUStateWatchFreq        = 30 * time.Second // frequency to watch GPU health state from exporter
	GPUStateReqTimeout       = 10 * time.Second // timeout for gRPC request sending to exporter socket
	GPUStateConnRetryFreq    = 5 * time.Second
	GPUStateConnREtryTimeout = 60 * time.Second

	DefaultRunnerLogPath = "/var/run/test-runner/test-runner.log"
	DefaultResultLogDir  = "/var/run/test-runner/results/"
	DefaultStatusDBPath  = "/var/run/test-runner/status.db"

	// TODO: rvs is one of offical ROCm RVS test suite names
	// revisit after deciding the pre-defined default test case
	DefaultUnhealthyGPUTestName           = "mem"
	DefaultUnhealthyGPUTestIterations     = 1
	DefaultUnhealthyGPUTestStopOnFailure  = true
	DefaultUnhealthyGPUTestTimeoutSeconds = 600
	DefaultPreJobCheckTestName            = "tst_single"
	DefaultPreJobCheckTestIterations      = 1
	DefaultPreJobCheckTestStopOnFailure   = true
	DefaultPreJobCheckTestTimeoutSeconds  = 600
	DefaultManualTestName                 = "mem"
	DefaultManualTestIterations           = 1
	DefaultManualTestStopOnFailure        = true
	DefaultManualTestTimeoutSeconds       = 600
)
