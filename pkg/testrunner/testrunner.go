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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/metricssvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/utils"
	testrunnerGen "github.com/pensando/device-metrics-exporter/pkg/testrunner/gen/testrunner"
	types "github.com/pensando/device-metrics-exporter/pkg/testrunner/interface"
)

var (
	defaultGlobalTestRunnerConfig = &testrunnerGen.TestRunnerConfig{
		SystemConfig: &testrunnerGen.TestRunnerSysConfig{
			RunnerLogPath: globals.DefaultRunnerLogPath,
			ResultLogDir:  globals.DefaultResultLogDir,
			StatusDBPath:  globals.DefaultStatusDBPath,
		},
		TestConfig: map[string]*testrunnerGen.TestCategoryConfig{
			testrunnerGen.TestCategory_GPU_HEALTH_CHECK.String(): {
				TestLocationTrigger: map[string]*testrunnerGen.TestLocationTrigger{
					globals.GlobalTestTriggerKeyword: {
						TestParameters: map[string]*testrunnerGen.TestParameters{
							testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH.String(): {
								TestCases: []*testrunnerGen.TestParameter{
									{
										Recipe:         globals.DefaultUnhealthyGPUTestName,
										Iterations:     globals.DefaultUnhealthyGPUTestIterations,
										StopOnFailure:  globals.DefaultUnhealthyGPUTestStopOnFailure,
										TimeoutSeconds: globals.DefaultUnhealthyGPUTestTimeoutSeconds,
									},
								},
							},
							testrunnerGen.TestTrigger_PRE_START_JOB_CHECK.String(): {
								TestCases: []*testrunnerGen.TestParameter{
									{
										Recipe:         globals.DefaultPreJobCheckTestName,
										Iterations:     globals.DefaultPreJobCheckTestIterations,
										StopOnFailure:  globals.DefaultPreJobCheckTestStopOnFailure,
										TimeoutSeconds: globals.DefaultPreJobCheckTestTimeoutSeconds,
									},
								},
							},
							testrunnerGen.TestTrigger_MANUAL.String(): {
								TestCases: []*testrunnerGen.TestParameter{
									{
										Recipe:         globals.DefaultManualTestName,
										Iterations:     globals.DefaultManualTestIterations,
										StopOnFailure:  globals.DefaultManualTestStopOnFailure,
										TimeoutSeconds: globals.DefaultManualTestTimeoutSeconds,
									},
								},
							},
						},
					},
				},
			},
		},
	}
)

type TestRunner struct {
	rvsPath                string
	rocmSMIPath            string
	exporterSocketPath     string
	testCategory           string
	testLocation           string
	testTrigger            string
	rvsTestCaseDir         string
	globalTestRunnerConfig *testrunnerGen.TestRunnerConfig
	rvsTestRunner          types.TestRunner
}

// initTestRunner init the test runner and related configs
// return the test location, either global or specific host name
func NewTestRunner(rvsPath, rvsTestCaseDir, rocmSMIPath, exporterSocketPath, testRunnerConfigPath, testCategory, testTrigger string) *TestRunner {
	runner := &TestRunner{
		rvsPath:            rvsPath,
		rocmSMIPath:        rocmSMIPath,
		exporterSocketPath: exporterSocketPath,
		testCategory:       testCategory,
		testTrigger:        testTrigger,
		rvsTestCaseDir:     rvsTestCaseDir,
	}
	// init test runner config
	// testRunnerConfigPath file existence has been verified
	runner.readTestRunnerConfig(testRunnerConfigPath)
	runner.validateTestTrigger()
	runner.initTestRunnerConfig()
	logger.Log.Printf("Test runner config: %+v", runner.globalTestRunnerConfig)
	return runner
}

// validateTestTrigger validates the test category/location/trigger existence
// return test locaiton, either global or specific hostname
func (tr *TestRunner) validateTestTrigger() {
	// 1. verify test category
	// given category config should exist
	if tr.globalTestRunnerConfig.TestConfig == nil {
		fmt.Printf("failed to find any test category config from %+v\n", tr.globalTestRunnerConfig)
		os.Exit(1)
	}
	if _, ok := tr.globalTestRunnerConfig.TestConfig[tr.testCategory]; !ok {
		fmt.Printf("failed to find category %+v from config %+v\n", tr.testCategory, tr.globalTestRunnerConfig)
		os.Exit(1)
	}
	// 2. verify test location
	// global config or given hostname's config should exist
	categoryConfig := tr.globalTestRunnerConfig.TestConfig[tr.testCategory]
	hostName, err := os.Hostname()
	if err != nil {
		logger.Log.Printf("failed to get hostname, err: %+v", err)
	}
	logger.Log.Printf("HostName: %v", hostName)
	if categoryConfig.TestLocationTrigger == nil {
		fmt.Printf("failed to find any global or host specific test config under category %+v: %+v\n", tr.testCategory, categoryConfig)
		os.Exit(1)
	}
	_, foundHostSpecifcTest := categoryConfig.TestLocationTrigger[hostName]
	_, foundGlobalTest := categoryConfig.TestLocationTrigger[globals.GlobalTestTriggerKeyword]
	if !foundGlobalTest && !foundHostSpecifcTest {
		fmt.Printf("cannot find neither global test config nor host specific config under category %+v: %+v\n", tr.testCategory, categoryConfig)
		os.Exit(1)
	}
	// 3. validate test trigger's config
	// if host specifc config was found
	// validate host specific config's trigger
	if foundHostSpecifcTest {
		if categoryConfig.TestLocationTrigger[hostName].TestParameters == nil {
			fmt.Printf("failed to get any test trigger under category %+v config: %+v\n", categoryConfig, categoryConfig.TestLocationTrigger[hostName])
			os.Exit(1)
		}
		if params, ok := categoryConfig.TestLocationTrigger[hostName].TestParameters[tr.testTrigger]; !ok {
			fmt.Printf("failed to get test trigger %+v under category %+v config: %+v\n", tr.testTrigger, categoryConfig, categoryConfig.TestLocationTrigger[hostName])
			os.Exit(1)
		} else if len(params.TestCases) == 0 {
			fmt.Printf("failed to get test case under category %+v trigger %+v config: %+v\n", categoryConfig, tr.testTrigger, categoryConfig.TestLocationTrigger[hostName])
			os.Exit(1)
		}
		tr.testLocation = hostName
		return
	}
	// if host specific config was not found
	// validate global config's trigger
	if categoryConfig.TestLocationTrigger[globals.GlobalTestTriggerKeyword].TestParameters == nil {
		fmt.Printf("failed to get any test trigger under category %+v global config: %+v\n", categoryConfig, categoryConfig.TestLocationTrigger[hostName])
		os.Exit(1)
	}
	if params, ok := categoryConfig.TestLocationTrigger[globals.GlobalTestTriggerKeyword].TestParameters[tr.testTrigger]; !ok {
		fmt.Printf("failed to get test trigger %+v under category %+v global config: %+v\n", tr.testTrigger, categoryConfig, categoryConfig.TestLocationTrigger[hostName])
		os.Exit(1)
	} else if len(params.TestCases) == 0 {
		fmt.Printf("failed to get test case under category %+v trigger %+v global config: %+v\n", categoryConfig, tr.testTrigger, categoryConfig.TestLocationTrigger[hostName])
		os.Exit(1)
	}
	tr.testLocation = globals.GlobalTestTriggerKeyword
}

func (tr *TestRunner) initLogger() {
	logger.SetLogDir(filepath.Dir(tr.globalTestRunnerConfig.SystemConfig.RunnerLogPath))
	logger.SetLogFile(filepath.Base(tr.globalTestRunnerConfig.SystemConfig.RunnerLogPath))
	logger.SetLogPrefix(globals.LogPrefix)
	logger.Init(utils.IsKubernetes())
}

// readTestRunnerConfig try to user provided customized test runner config from given file
func (tr *TestRunner) readTestRunnerConfig(configPath string) {
	file, err := os.Open(configPath)
	if err != nil {
		tr.globalTestRunnerConfig = defaultGlobalTestRunnerConfig
		tr.initLogger()
		logger.Log.Printf("cannot read provided test runner config at %+v, err: %+v, using default test runner config", configPath, err)
		return
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		tr.globalTestRunnerConfig = defaultGlobalTestRunnerConfig
		tr.initLogger()
		logger.Log.Printf("cannot read provided test runner config at %+v, err: %+v, using default test runner config", configPath, err)
		return
	}
	var config *testrunnerGen.TestRunnerConfig
	err = json.Unmarshal(bytes, config)
	if err != nil {
		tr.globalTestRunnerConfig = defaultGlobalTestRunnerConfig
		tr.initLogger()
		logger.Log.Printf("cannot read provided test runner config at %+v, err: %+v, using default test runner config", configPath, err)
		return
	}
	tr.globalTestRunnerConfig = config
	tr.initLogger()
}

func (tr *TestRunner) initTestRunnerConfig() {
	// init test runner log
	err := os.MkdirAll(filepath.Dir(tr.globalTestRunnerConfig.SystemConfig.RunnerLogPath), 0755)
	if err != nil {
		fmt.Printf("Failed to create dir for test runner logs %+v, err: %+v\n", filepath.Dir(tr.globalTestRunnerConfig.SystemConfig.RunnerLogPath), err)
		os.Exit(1)
	}

	// init test reuslt log dir
	err = os.MkdirAll(tr.globalTestRunnerConfig.SystemConfig.ResultLogDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create dir for test result logs %+v, err: %+v\n", tr.globalTestRunnerConfig.SystemConfig.ResultLogDir, err)
		os.Exit(1)
	}
	// init status db
	// don't try to create when status db already exists
	// test runner needs to read the existing db and rerun incomplete test before crash/restart
	if _, err := os.Stat(tr.globalTestRunnerConfig.SystemConfig.StatusDBPath); err != nil && os.IsNotExist(err) {
		_, err = os.Create(tr.globalTestRunnerConfig.SystemConfig.StatusDBPath)
		if err != nil {
			fmt.Printf("Failed to create test status db %+v, err: %+v\n", tr.globalTestRunnerConfig.SystemConfig.StatusDBPath, err)
			os.Exit(1)
		}
		runnerStatus := &testrunnerGen.TestRunnerStatus{
			RunningTest: map[string]string{},
		}
		err = SaveRunnerStatus(runnerStatus, tr.globalTestRunnerConfig.SystemConfig.StatusDBPath)
		if err != nil {
			fmt.Printf("Failed to init test runner status db %+v, err: %+v\n", tr.globalTestRunnerConfig.SystemConfig.StatusDBPath, err)
			os.Exit(1)
		}
	}
	// the validation has been done previously by validateTestTrigger()
	testParams := tr.globalTestRunnerConfig.TestConfig[tr.testCategory].TestLocationTrigger[tr.testLocation].TestParameters[tr.testTrigger]
	testCfgPath := filepath.Join(tr.rvsTestCaseDir, testParams.TestCases[0].Recipe+".conf")
	if _, err := os.Stat(testCfgPath); err != nil {
		fmt.Printf("Trigger %+v cannot find corresponding test config file %+v, err: %+v\n", tr.testTrigger, testCfgPath, err)
		os.Exit(1)
	}
	logger.Log.Printf("applied test config for %+v", tr.testLocation)
}

// the validation functions have make sure that the given category/location/trigger config exists and valid within runnerConfig
// this function will be responsible to trigger the test
func (tr *TestRunner) TriggerTest() {
	switch tr.testCategory {
	case testrunnerGen.TestCategory_GPU_HEALTH_CHECK.String():
		switch tr.testTrigger {
		case testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH.String():
			// init rvs test runner
			// and start to listen for unix socket to receive the event
			// for triggering the test run on unhealthy GPU
			rvsTestRunner, err := NewRvsTestRunner(tr.rvsPath, tr.rvsTestCaseDir, tr.globalTestRunnerConfig.SystemConfig.ResultLogDir)
			if err != nil || rvsTestRunner == nil {
				logger.Log.Printf("failed to create rvs test runner, runner: %+v, err: %+v", rvsTestRunner, err)
				os.Exit(1)
			}
			tr.rvsTestRunner = rvsTestRunner
			testParams := tr.globalTestRunnerConfig.TestConfig[tr.testCategory].TestLocationTrigger[tr.testLocation].TestParameters[tr.testTrigger]
			tr.watchGPUState(testParams)
		case testrunnerGen.TestTrigger_MANUAL.String(),
			testrunnerGen.TestTrigger_PRE_START_JOB_CHECK.String():
			rvsTestRunner, err := NewRvsTestRunner(tr.rvsPath, tr.rvsTestCaseDir, tr.globalTestRunnerConfig.SystemConfig.ResultLogDir)
			if err != nil || rvsTestRunner == nil {
				logger.Log.Printf("failed to create rvs test runner, runner: %+v, err: %+v", rvsTestRunner, err)
				os.Exit(1)
			}
			tr.rvsTestRunner = rvsTestRunner
			testParams := tr.globalTestRunnerConfig.TestConfig[tr.testCategory].TestLocationTrigger[tr.testLocation].TestParameters[tr.testTrigger]
			tr.manualTestGPU(testParams)
		default:
			logger.Log.Printf("unsupported test trigger %+v for category %+v", tr.testTrigger, tr.testCategory)
			os.Exit(1)
		}
	}
}

func (tr *TestRunner) watchGPUState(parameters *testrunnerGen.TestParameters) {
	ticker := time.NewTicker(globals.GPUStateConnRetryFreq)
	defer ticker.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), globals.GPUStateConnREtryTimeout)
	defer cancel()

	var err error
	var conn *grpc.ClientConn
	connected := false

	for !connected {
		select {
		case <-ticker.C:
			conn, err = grpc.NewClient("unix:"+tr.exporterSocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				logger.Log.Printf("testrunner cannot connect to %v: %v", "unix:"+tr.exporterSocketPath, err)
				continue
			}
			connected = true
			defer conn.Close()
		case <-ctx.Done():
			logger.Log.Fatalf("retry exhausted: testrunner cannot connect to %v", "unix:"+tr.exporterSocketPath)
			return
		}
	}

	c := metricssvc.NewMetricsServiceClient(conn)
	watchTicker := time.NewTicker(globals.GPUStateWatchFreq)
	defer watchTicker.Stop()

	// handle test runner crash or restart
	// read existing test runner status db
	// immediately start test on interrupted test before restarting
	statusObj, _ := LoadRunnerStatus(tr.globalTestRunnerConfig.SystemConfig.StatusDBPath)
	ids := []string{}
	if statusObj != nil && len(statusObj.RunningTest) > 0 {
		for deviceID := range statusObj.RunningTest {
			ids = append(ids, deviceID)
		}
		logger.Log.Printf("found GPU %+v with incomplete test before restart %+v, start to rerun test", ids, statusObj)
		go tr.testGPU(testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH, ids, parameters, true)
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
					if !strings.EqualFold(state.Health, metricssvc.GPUHealth_HEALTHY.String()) {
						// TODO: currently exporter with gpuagent just returns GPU index number
						// we need to convert it to GUID per rvs's request
						// modify this after rvs starts to accept index number as ID
						id, err := GetGUIDFromIndex(state.ID, tr.rocmSMIPath)
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
				go tr.testGPU(testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH, unHealthyGPUIDs, parameters, false)
			} else {
				logger.Log.Printf("all GPUs are healthy, skip testing")
			}
		}
	}
}

func (tr *TestRunner) testGPU(trigger testrunnerGen.TestTrigger, ids []string, parameters *testrunnerGen.TestParameters, isRerun bool) {
	// load ongoing test status
	// avoid run multiple test on the same device
	validIDs, statusObj := removeIDsWithExistingTest(trigger, tr.globalTestRunnerConfig.SystemConfig.StatusDBPath, ids, parameters, isRerun)
	if isRerun {
		// for rerun after test runner restart
		// we need to force to run the incomplete test
		// ignore the status db temporarily
		validIDs = ids
	}
	if len(ids) > 0 && len(validIDs) == 0 {
		// all original target devices have existing running test, skip for now
		return
	}
	// if both len(ids) and len(validIDs) are 0
	// that means all devices were selected

	handler, err := tr.rvsTestRunner.GetTestHandler(parameters.TestCases[0].Recipe, types.TestParams{
		Iterations:    uint(parameters.TestCases[0].Iterations),
		StopOnFailure: parameters.TestCases[0].StopOnFailure,
		DeviceIDs:     validIDs,
		Timeout:       uint(parameters.TestCases[0].TimeoutSeconds),
	})
	if err != nil {
		logger.Log.Printf("failed to get test run handler, err: %+v", err)
		return
	}

	err = handler.StartTest()
	if err != nil {
		logger.Log.Printf("failed to start test run, err: %+v", err)
		return
	}

	if len(validIDs) == 0 {
		// all devices were selected
		ids, err := GetAllGUIDs(tr.rocmSMIPath)
		if err != nil {
			// TODO: add more error handling when failed to get all GUIDs
		}
		validIDs = ids
	}
	for _, id := range validIDs {
		statusObj.RunningTest[id] = parameters.TestCases[0].Recipe
	}

	err = SaveRunnerStatus(statusObj, tr.globalTestRunnerConfig.SystemConfig.StatusDBPath)
	if err != nil {
		//TODO: add error handling here if new running status failed to be saved
	}

	select {
	case <-time.After(time.Duration(parameters.TestCases[0].TimeoutSeconds) * time.Second):
		logger.Log.Printf("Trigger: %v Test: %v GPU IDs: %v timeout", trigger.String(), parameters.TestCases[0].Recipe, ids)
	case <-handler.Done():
		// TODO: this has to change later based on result logs parsing.
		// for now updating same result in all GPU
		result := handler.Result()
		logger.Log.Printf("Trigger: %v Test: %v GPU IDs: %v completed. Result: %v", trigger.String(), parameters.TestCases[0].Recipe, ids, result)
		// save log into gzip file
		if stdout := handler.Stdout(); stdout != "" {
			SaveTestResultToGz(stdout, GetLogFilePath(tr.globalTestRunnerConfig.SystemConfig.ResultLogDir, trigger.String(), parameters.TestCases[0].Recipe, "stdout"))
		}
		if stderr := handler.Stderr(); stderr != "" {
			SaveTestResultToGz(stderr, GetLogFilePath(tr.globalTestRunnerConfig.SystemConfig.ResultLogDir, trigger.String(), parameters.TestCases[0].Recipe, "stderr"))
		}
	}

	// remove the running test status from db
	for _, id := range validIDs {
		delete(statusObj.RunningTest, id)
	}
	SaveRunnerStatus(statusObj, tr.globalTestRunnerConfig.SystemConfig.StatusDBPath)
}

func (tr *TestRunner) manualTestGPU(parameters *testrunnerGen.TestParameters) {
	// handle test runner crash or restart
	// read existing test runner status db
	// immediately start test on interrupted test before restarting
	statusObj, _ := LoadRunnerStatus(tr.globalTestRunnerConfig.SystemConfig.StatusDBPath)
	ids := []string{}
	if statusObj != nil && len(statusObj.RunningTest) > 0 {
		for deviceID := range statusObj.RunningTest {
			ids = append(ids, deviceID)
		}
		logger.Log.Printf("found GPU %+v with incomplete test before restart %+v, start to rerun test", ids, statusObj)
		tr.testGPU(testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH, ids, parameters, true)
	} else {
		tr.testGPU(testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH, ids, parameters, false)
	}
}
