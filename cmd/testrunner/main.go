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

	c := metricssvc.NewMetricsServiceClient(conn)
	watchTicker := time.NewTicker(globals.GPUStateWatchFreq)
	defer watchTicker.Stop()
	unhealthyGPUTestCfg := globalTestRunnerConfig.GPUTestTriggers[testrunner.UnhealthyGPU]

	// handle test runner crash or restart
	// read existing test runner status db
	// immediately start test on interrupted test before restarting
	statusObj, _ := testrunner.LoadRunnerStatus(globalTestRunnerConfig.StatusDBPath)
	ids := []string{}
	if statusObj != nil && len(statusObj.Status) > 0 {
		for deviceID := range statusObj.Status {
			ids = append(ids, deviceID)
		}
		logger.Log.Printf("found GPU %+v with incomplete test before restart %+v, start to rerun test", ids, statusObj)
		go testGPU(testrunner.UnhealthyGPU, ids, unhealthyGPUTestCfg, rocmSMIPath, true)
	}

	for {
		select {
		case <-watchTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), globals.GPUStateReqTimeout)
			r, err := c.List(ctx, &emptypb.Empty{})
			if err != nil {
				logger.Log.Printf("could not list GPU state: %v", err)
				cancel()
				continue
			}
			logger.Log.Printf("GPU State: %s", r.String())
			cancel()

			unHealthyGPUIDs := []string{}
			if r != nil {
				for _, state := range r.GPUState {
					// if any GPU is not healthy, start a test against those GPUs
					if state.Health != metricssvc.GPUHealth_HEALTHY.String() {
						// TODO: currently exporter with gpuagent just returns GPU index number
						// we need to convert it to GUID per rvs's request
						// modify this after rvs starts to accept index number as ID
						id, err := getGUIDFromIndex(state.ID, rocmSMIPath)
						if err != nil {
							logger.Log.Printf("failed to fetch GUID for GPU card%v, err: %+v", state.ID, err)
							continue
						}
						unHealthyGPUIDs = append(unHealthyGPUIDs, id)
					}
				}
			}

			// start test on unhealthy GPU
			if len(unHealthyGPUIDs) > 0 {
				logger.Log.Printf("found GPU with unhealthy state %+v", unHealthyGPUIDs)
				go testGPU(testrunner.UnhealthyGPU, unHealthyGPUIDs, unhealthyGPUTestCfg, rocmSMIPath, false)
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
