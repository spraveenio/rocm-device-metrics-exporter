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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ROCm/device-metrics-exporter/pkg/amdgpu/globals"
	"github.com/ROCm/device-metrics-exporter/pkg/amdgpu/logger"
	"github.com/ROCm/device-metrics-exporter/pkg/testrunner"
)

const (
	testCategoryEnv = "TEST_CATEGORY"
	testTriggerEnv  = "TEST_TRIGGER"
	logDirEnv       = "LOG_MOUNT_DIR"
)

var (
	Version   string
	BuildDate string
	GitCommit string
)

func getStrFromEnvOrDefault(env string, defaultVal string) string {
	if val, exists := os.LookupEnv(env); exists {
		return val
	}
	return defaultVal
}

func main() {
	var (
		rvsPath              = flag.String("rvs-path", globals.RVSPath, "Path to ROCmValidationSuite rvs binary file")
		rocmSMIPath          = flag.String("rocm-path", globals.ROCmSMIPath, "Path to rocm-smi binary file")
		rvsTestCaseDir       = flag.String("rvs-test-case-dir", globals.AMDTestCaseDir, "Directory of test suite config files")
		testRunnerConfigPath = flag.String("test-runner-cfg-path", globals.AMDTestRunnerCfgPath, "Path to test runner config file")
		exporterSocketPath   = flag.String("exporter-socket-path", globals.MetricsSocketPath, "Path to exporter metrics server socket")
		versionOpt           = flag.Bool("version", false, "show version")
	)

	flag.Parse()

	if *versionOpt {
		fmt.Printf("Version : %v\n", Version)
		fmt.Printf("BuildDate: %v\n", BuildDate)
		fmt.Printf("GitCommit: %v\n", GitCommit)
		os.Exit(0)
	}
	defer conn.Close()
	c := metricssvc.NewMetricsServiceClient(conn)

	timer := time.NewTicker(gpuStateWatchFreq)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			ctx, cancel := context.WithTimeout(context.Background(), gpuStateReqTimeout)
			r, err := c.List(ctx, &emptypb.Empty{})
			if err != nil {
				logger.Log.Fatalf("could not list GPU state: %v", err)
				cancel()
				return
			}
			logger.Log.Printf("GPU State: %s", r.String())
			cancel()

			unHealthyGPUIDs := []string{}
			if r != nil {
				for _, state := range r.GPUState {
					// if any GPU is not healthy, start a test against those GPUs
					if state.Health != metricssvc.GPUHealth_HEALTHY.String() {
						unHealthyGPUIDs = append(unHealthyGPUIDs, state.ID)
					}
				}
			}

			// start test on unhealthy GPU
			if len(globalTestConfig.DeviceIDs) > 0 {
				logger.Log.Printf("test config force to run test for GPU %+v", globalTestConfig.DeviceIDs)
				go testGPU(globalTestConfig.DeviceIDs)
			} else if len(unHealthyGPUIDs) > 0 {
				logger.Log.Printf("found GPU with unhealthy state %+v", unHealthyGPUIDs)
				go testGPU(unHealthyGPUIDs)
			}
		}
	}
}

	testCategory := strings.ToUpper(getStrFromEnvOrDefault(testCategoryEnv, globals.DefaultTestCategory))
	testTrigger := strings.ToUpper(getStrFromEnvOrDefault(testTriggerEnv, globals.DefaultTestTrigger))
	logDir := getStrFromEnvOrDefault(logDirEnv, globals.DefaultRunnerLogDir)

	testrunner.ValidateArgs(testCategory, testTrigger, *rvsPath, *rocmSMIPath, *rvsTestCaseDir, *exporterSocketPath)
	runner := testrunner.NewTestRunner(*rvsPath, *rvsTestCaseDir, *rocmSMIPath, *exporterSocketPath, *testRunnerConfigPath, testCategory, testTrigger, logDir)
	logger.Log.Printf("Version : %v", Version)
	logger.Log.Printf("BuildDate: %v", BuildDate)
	logger.Log.Printf("GitCommit: %v", GitCommit)
	runner.TriggerTest()
}
