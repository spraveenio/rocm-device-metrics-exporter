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
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/metricssvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/utils"
	"github.com/pensando/device-metrics-exporter/pkg/testrunner"
	types "github.com/pensando/device-metrics-exporter/pkg/testrunner/interface"
)

var (
	Version                string
	BuildDate              string
	GitCommit              string
	rvsTestRunner          types.TestRunner
	globalTestRunnerConfig = &testrunner.TestRunnerConfig{
		RunnerLogPath: globals.DefaultRunnerLogPath,
		ResultLogDir:  globals.DefaultResultLogDir,
		StatusDBPath:  globals.DefaultStatusDBPath,
		GPUTestTriggers: map[testrunner.TestTriggerCondition]testrunner.TestConfig{
			testrunner.UnhealthyGPU: {
				Name: globals.DefaultUnhealthyGPUTestName,
				Parameters: types.TestParams{
					Iterations:    globals.DefaultUnhealthyGPUTestIterations,
					StopOnFailure: globals.DefaultUnhealthyGPUTestStopOnFailure,
					Timeout:       globals.DefaultUnhealthyGPUTestTimeoutSeconds,
				},
			},
			testrunner.PreJobCheck: {
				Name: globals.DefaultPreJobCheckTestName,
				Parameters: types.TestParams{
					Iterations:    globals.DefaultPreJobCheckTestIterations,
					StopOnFailure: globals.DefaultPreJobCheckTestStopOnFailure,
					Timeout:       globals.DefaultPreJobCheckTestTimeoutSeconds,
				},
			},
		},
	}
)

// validateArgs validate argument to make sure the mandatory tools/configs are available
func validateArgs(rvsPath, rocmSMIPath, testCaseDir, exporterSocketPath string) {
	statOrExit(rvsPath, false)
	statOrExit(rocmSMIPath, false)
	statOrExit(exporterSocketPath, false)
	statOrExit(testCaseDir, true)
	dryRunBinary(rvsPath, "-g")     // run rvs to list GPU to make sure rvs is working
	dryRunBinary(rocmSMIPath, "-i") // run rocm-smi to list GPU IDs to make sure GPU info is available
}

// dryRunBinary dry run the executable binary to make sure it is working, otherwise exit
func dryRunBinary(binPath, arg string) {
	cmd := exec.Command(binPath, arg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error executing %+v %+v: %+v\n", binPath, arg, err)
		fmt.Printf("Output: %+v\n", string(output))
		os.Exit(1)
	}
}

// statOrExit look given file/dir exists otherwise exit
func statOrExit(path string, isFolder bool) {
	if info, err := os.Stat(path); err != nil {
		fmt.Printf("Failed to find %+v, err: %+v\n", path, err)
		os.Exit(1)
	} else if info != nil && info.IsDir() != isFolder {
		fmt.Printf("Expect %+v IsDir %+v got %+v\n", path, isFolder, info.IsDir())
		os.Exit(1)
	}
}

// readTestRunnerConfig try to user provided customized test runner config from given file
func readTestRunnerConfig(configPath string) (*testrunner.TestRunnerConfig, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("error opening test runner config %+v: %+v", configPath, err)
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file %+v: %+v", configPath, err)
	}
	var config testrunner.TestRunnerConfig
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling test runner config at %+v: %+v", configPath, err)
	}
	return &config, nil
}

func initConfig(config *testrunner.TestRunnerConfig, testCaseDir string) {
	// test runner log
	err := os.MkdirAll(filepath.Dir(config.RunnerLogPath), 0755)
	if err != nil {
		fmt.Printf("Failed to create dir for test runner logs %+v, err: %+v\n", filepath.Dir(config.RunnerLogPath), err)
		os.Exit(1)
	}
	logger.SetLogDir(filepath.Dir(config.RunnerLogPath))
	logger.SetLogFile(filepath.Base(config.RunnerLogPath))
	logger.SetLogPrefix(globals.LogPrefix)
	logger.Init(utils.IsKubernetes())
	// test reuslt log dir
	err = os.MkdirAll(config.ResultLogDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create dir for test result logs %+v, err: %+v\n", config.ResultLogDir, err)
		os.Exit(1)
	}
	// status db
	// don't try to create when status db already exists
	// test runner needs to read the existing db and rerun incomplete test before crash/restart
	if _, err := os.Stat(config.StatusDBPath); err != nil && os.IsNotExist(err) {
		_, err = os.Create(config.StatusDBPath)
		if err != nil {
			fmt.Printf("Failed to create test status db %+v, err: %+v\n", config.StatusDBPath, err)
			os.Exit(1)
		}
		runnerStatus := &testrunner.TestRunnerStatus{
			Status: map[string]string{},
		}
		err = testrunner.SaveRunnerStatus(runnerStatus, config.StatusDBPath)
		if err != nil {
			fmt.Printf("Failed to init test runner status db %+v, err: %+v\n", config.StatusDBPath, err)
			os.Exit(1)
		}
	}
	// GPU test triggers map
	for trigger, testCfg := range config.GPUTestTriggers {
		testCfgPath := filepath.Join(testCaseDir, testCfg.Name+".conf")
		if _, err := os.Stat(testCfgPath); err != nil {
			fmt.Printf("Trigger %+v cannot find corresponding test config file %+v, err: %+v\n", trigger, testCfgPath, err)
			os.Exit(1)
		}
	}
}

// initTestRunner uses existing toolkits to collect info before running any test
func initTestRunner(testCaseDir, testRunnerConfigPath string) {
	// init test runner config
	// testRunnerConfigPath file existence has been verified
	config, err := readTestRunnerConfig(testRunnerConfigPath)
	if config != nil {
		initConfig(config, testCaseDir)
		globalTestRunnerConfig = config
	} else {
		initConfig(globalTestRunnerConfig, testCaseDir)
		logger.Log.Printf("failed to read provided test runner config at %+v, err: %+v, using default test runner config", testRunnerConfigPath, err)
	}
	logger.Log.Printf("Test runner config: %+v", globalTestRunnerConfig)
}

func watchGPUState(socketPath, rocmSMIPath string) {
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
			conn, err = grpc.NewClient("unix:"+socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				logger.Log.Printf("testrunner cannot connect to %v: %v", "unix:"+socketPath, err)
				continue
			}
			defer conn.Close()
		case <-ctx.Done():
			logger.Log.Fatalf("retry exhausted: testrunner cannot connect to %v", "unix:"+socketPath)
			return
		}
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

func removeIDsWithExistingTest(trigger testrunner.TestTriggerCondition, ids []string, config testrunner.TestConfig) ([]string, *testrunner.TestRunnerStatus) {
	// load ongoing test status
	// avoid run multiple test on the same device
	statusObj, err := testrunner.LoadRunnerStatus(globalTestRunnerConfig.StatusDBPath)
	if err != nil {
		logger.Log.Printf("failed to load test runner status %+v, err: %+v", globalTestRunnerConfig.StatusDBPath, err)
		if os.IsNotExist(err) {
			os.Create(globalTestRunnerConfig.StatusDBPath)
		}
		// TODO: add more error handling when failed to load runner running status
	}
	validIDs := []string{}
	for _, id := range ids {
		if testName, ok := statusObj.Status[id]; ok {
			logger.Log.Printf("trigger %+v is trying to run test %+v on device %+v but found existing running test %+v, skip for now",
				trigger, config.Name, id, testName)
		} else {
			validIDs = append(validIDs, id)
		}
	}
	return validIDs, statusObj
}

func testGPU(trigger testrunner.TestTriggerCondition, ids []string, config testrunner.TestConfig, rocmSMIPath string, isRerun bool) {
	// load ongoing test status
	// avoid run multiple test on the same device
	validIDs, statusObj := removeIDsWithExistingTest(trigger, ids, config)
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
	// if len(ids) and len(validIDs) are 0
	// that means all devices were selected

	handler, err := rvsTestRunner.GetTestHandler(config.Name, types.TestParams{
		Iterations:    config.Parameters.Iterations,
		StopOnFailure: config.Parameters.StopOnFailure,
		DeviceIDs:     validIDs,
		Timeout:       config.Parameters.Timeout,
		ExtraArgs:     config.Parameters.ExtraArgs,
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
		ids, err := getAllGUIDs(rocmSMIPath)
		if err != nil {
			// TODO: add more error handling when failed to get all GUIDs
		}
		validIDs = ids
	}
	for _, id := range validIDs {
		statusObj.Status[id] = config.Name
	}

	err = testrunner.SaveRunnerStatus(statusObj, globalTestRunnerConfig.StatusDBPath)
	if err != nil {
		//TODO: add error handling here if new running status failed to be saved
	}

	select {
	case <-time.After(time.Duration(config.Parameters.Timeout) * time.Second):
		logger.Log.Printf("Trigger: %v Test: %v GPU IDs: %v timeout", trigger, config.Name, ids)
	case <-handler.Done():
		// TODO: this has to change later based on result logs parsing.
		// for now updating same result in all GPU
		result := handler.Result()
		logger.Log.Printf("Trigger: %v Test: %v GPU IDs: %v completed. Result: %v", trigger, config.Name, ids, result)
		// save log into gzip file
		if stdout := handler.Stdout(); stdout != "" {
			gzipStringToFile(stdout, getLogFilePath(string(trigger), config.Name, "stdout"))
		}
		if stderr := handler.Stderr(); stderr != "" {
			gzipStringToFile(stderr, getLogFilePath(string(trigger), config.Name, "stderr"))
		}
	}

	// remove the running test status from db
	for _, id := range validIDs {
		delete(statusObj.Status, id)
	}
	testrunner.SaveRunnerStatus(statusObj, globalTestRunnerConfig.StatusDBPath)
}

func getLogFilePath(trigger, testName, suffix string) string {
	now := time.Now().UTC()
	ts := now.Format("20060102_150405")
	fileName := ts + "_" + trigger + "_" + testName + "_" + suffix + ".gz"
	return filepath.Join(globalTestRunnerConfig.ResultLogDir, fileName)
}

func gzipStringToFile(output, path string) {
	// Create the file
	file, err := os.Create(path)
	if err != nil {
		logger.Log.Printf("failed to create gzip file %v, err: %v", path, err)
	}
	defer file.Close()

	// Create a gzip writer
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	// Write the string data to the gzip writer
	_, err = gzipWriter.Write([]byte(output))
	if err != nil {
		logger.Log.Printf("failed to write to gzip writer %v, err: %v", path, err)
	}
}

// getAllGUIDs list all GUIDs from rocm-smi
func getAllGUIDs(rocmSMIPath string) ([]string, error) {
	cmd := exec.Command(rocmSMIPath, "-i", "--json")
	output, err := cmd.Output()
	if err != nil {
		return []string{}, err
	}

	// Parse the JSON response
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	if err != nil {
		return []string{}, err
	}

	var guids []string
	for _, cardInfo := range result {
		if cardInfoMap, ok := cardInfo.(map[string]interface{}); ok {
			if guid, ok := cardInfoMap["GUID"].(string); ok {
				guids = append(guids, guid)
			}
		}
	}

	return guids, nil
}

// getGUIDFromIndex use rocm-smi to get GUID from index number
func getGUIDFromIndex(index, rocmSMIPath string) (string, error) {
	cmd := exec.Command(rocmSMIPath, "-i", "--json")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse the JSON response
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	if err != nil {
		return "", err
	}

	// Retrieve the GUID
	cardKey := fmt.Sprintf("card%s", index)
	if cardInfo, exists := result[cardKey]; exists {
		if cardInfoMap, ok := cardInfo.(map[string]interface{}); ok {
			if guid, ok := cardInfoMap["GUID"].(string); ok {
				return guid, nil
			}
		}
	}

	return "", fmt.Errorf("failed to GUID from 'rocm-smi -i --json' output: %+v", result)
}

func main() {
	var (
		rvsPath              = flag.String("rvs-path", globals.RVSPath, "Path to ROCmValidationSuite rvs binary file")
		rocmSMIPath          = flag.String("rocm-path", globals.ROCmSMIPath, "Path to rocm-smi binary file")
		testCaseDir          = flag.String("test-case-dir", globals.AMDTestCaseDir, "Directory of test suite config files")
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

	validateArgs(*rvsPath, *rocmSMIPath, *testCaseDir, *exporterSocketPath)
	initTestRunner(*testCaseDir, *testRunnerConfigPath)

	logger.Log.Printf("Version : %v", Version)
	logger.Log.Printf("BuildDate: %v", BuildDate)
	logger.Log.Printf("GitCommit: %v", GitCommit)

	// init rvs test runner
	// and start to listen for unix socket to receive the event
	// for triggering the test run on unhealthy GPU
	runner, err := testrunner.NewRvsTestRunner(*rvsPath, *testCaseDir, *globalTestRunnerConfig)
	if err != nil || runner == nil {
		logger.Log.Printf("failed to create rvs test runner, runner: %+v, err: %+v", runner, err)
		return
	}
	rvsTestRunner = runner
	watchGPUState(*exporterSocketPath, *rocmSMIPath)
}
