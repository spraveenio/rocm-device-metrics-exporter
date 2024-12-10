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
	// metrics exporter default server port
	AMDListenPort = 5000

	// metrics exporter configuraiton file path
	AMDMetricsFile = "/etc/metrics/config.json"

	// GPUAgent internal clien port
	GPUAgentPort = 50061

	ZmqPort = "6601"

	SlurmDir = "/var/run/exporter/"

	MetricsSocketPath = "/var/lib/amd-metrics-exporter/amdgpu_device_metrics_exporter_grpc.socket"

	// rvs binary path
	RVSPath = "/opt/rocm/bin/rvs"

	// rocm-smi binary path
	ROCmSMIPath = "/opt/rocm/bin/rocm-smi"

	// gpu test runner configuration file path
	AMDTestCaseDir = "/opt/rocm/share/rocm-validation-suite/conf/"

	//PodResourceSocket - k8s pod grpc socket
	PodResourceSocket = "/var/lib/kubelet/pod-resources/kubelet.sock"

	// AMDGPUResourceLabel - k8s AMD gpu resource label
	AMDGPUResourceLabel = "amd.com/gpu"

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
	DefaultUnhealthyGPUTestName           = "rvs"
	DefaultUnhealthyGPUTestIterations     = 1
	DefaultUnhealthyGPUTestStopOnFailure  = true
	DefaultUnhealthyGPUTestTimeoutSeconds = 600

	DefaultPreJobCheckTestName           = "rvs"
	DefaultPreJobCheckTestIterations     = 1
	DefaultPreJobCheckTestStopOnFailure  = true
	DefaultPreJobCheckTestTimeoutSeconds = 600
)
