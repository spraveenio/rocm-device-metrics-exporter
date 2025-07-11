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

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	k8sclient "github.com/ROCm/device-metrics-exporter/pkg/client"
	"github.com/ROCm/device-metrics-exporter/pkg/exporter/logger"
	exputils "github.com/ROCm/device-metrics-exporter/pkg/exporter/utils"
	"github.com/ROCm/device-metrics-exporter/tools/amd-gpu-health/cli/utils"
)

var (
	metricName             string
	counterMetricThreshold int64
	gaugeMetricThreshold   float64
	metricDuration         string
	nodeName               string
)

var QueryMetricCmd = &cobra.Command{
	Use:   "query",
	Short: "query a metric",
	Long:  "query a metric",
}

var counterMetricCmd = &cobra.Command{
	Use:   "counter-metric",
	Short: "metric of type counter",
	Long:  "metric of type counter",
	Run:   counterMetricCmdHandler,
}

var gaugeMetricCmd = &cobra.Command{
	Use:   "gauge-metric",
	Short: "metric of type gauge",
	Long:  "metric of type gauge",
	Run:   gaugeMetricCmdHandler,
}

func counterMetricCmdHandler(cmd *cobra.Command, args []string) {
	//TODO: pass configfile namespace info values from config file
	configPath := viper.GetString("kubeConfigPath")
	ns := viper.GetString("namespace")
	//get node name from env variable
	nodeName, err := exputils.GetHostName()
	if err != nil {
		logger.Log.Printf("unable to fetch node name. error:%v", err)
		os.Exit(2)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	k8sc, err := k8sclient.NewClient(ctx, configPath, nodeName)
	if err != nil {
		logger.Log.Printf("unable to instantiate k8s client. error:%v", err)
		os.Exit(2)
	}
	metricsEndpoint := k8sc.GetGPUMetricsEndpointURL(ns, nodeName)
	// if endpoint is not found, we should not exit with error code 1.
	if metricsEndpoint == "" {
		os.Exit(2)
	}
	// check if the below mandatory args are provided or not
	if !cmd.Flags().Changed("metric") || !cmd.Flags().Changed("threshold") {
		os.Exit(2)
	}
	// query metrics endpoint
	response := utils.QueryMetricsEndpoint(metricsEndpoint)

	logger.Log.Printf("counter metrics response=%s", response)
	metrics := utils.ParseGPUMetricsResponse(response, metricName)
	if len(metrics) == 0 {
		os.Exit(2)
	}
	for _, metric := range metrics {
		if int64(metric) > counterMetricThreshold {
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func gaugeMetricCmdHandler(cmd *cobra.Command, args []string) {
	//TODO: pass configfile namespace info values from config file
	configPath := viper.GetString("configPath")
	ns := viper.GetString("namespace")
	//get node name from env variable
	nodeName, err := exputils.GetHostName()
	if err != nil {
		logger.Log.Printf("unable to fetch node name. error:%v", err)
		os.Exit(2)
	}
	//validate all the args
	if !cmd.Flags().Changed("metric") || !cmd.Flags().Changed("threshold") {
		os.Exit(2)
	}

	var metrics []float64
	// if duration is specified, query from prometheus endpoint
	// else query the latest value from local exporter endpoint.
	if !cmd.Flags().Changed("duration") {
		prometheusEndpoint := os.Getenv("PROMETHEUS_ENDPOINT")
		promQuery := fmt.Sprintf(`avg_over_time(%s{hostname="%s"}[%s])`, metricName, nodeName, metricDuration)
		response, err := utils.QueryPrometheusEndpoint(prometheusEndpoint, promQuery)
		if err != nil {
			os.Exit(2)
		}
		logger.Log.Printf("gauge metrics prometheus endpoint response=%v", response)
		metrics = utils.ParsePromethuesResponse(*response)
	} else {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		k8sc, err := k8sclient.NewClient(ctx, configPath, nodeName)
		if err != nil {
			logger.Log.Printf("unable to instantiate k8s client. error:%v", err)
			os.Exit(2)
		}
		metricsEndpoint := k8sc.GetGPUMetricsEndpointURL(ns, nodeName)
		// if endpoint is not found, we should not exit with error code 1.
		if metricsEndpoint == "" {
			os.Exit(2)
		}
		response := utils.QueryMetricsEndpoint(metricsEndpoint)

		logger.Log.Printf("gauge metrics exporter endpoint response=%s", response)
		metrics = utils.ParseGPUMetricsResponse(response, metricName)
	}

	//loop over all the items and check value against threshold
	// if crossed threshold, exit(1), else exit(0)
	for _, metric := range metrics {
		if metric > gaugeMetricThreshold {
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func init() {
	RootCmd.AddCommand(QueryMetricCmd)
	QueryMetricCmd.AddCommand(counterMetricCmd)
	QueryMetricCmd.AddCommand(gaugeMetricCmd)

	counterMetricCmd.Flags().StringVarP(&nodeName, "node", "n", "", "Specify node name")
	counterMetricCmd.Flags().StringVarP(&metricName, "metric", "m", "", "Specify metric name")
	counterMetricCmd.Flags().Int64VarP(&counterMetricThreshold, "threshold", "t", 0, "Specify threshold value for the metric")

	gaugeMetricCmd.Flags().StringVarP(&nodeName, "node", "n", "", "Specify node name")
	gaugeMetricCmd.Flags().StringVarP(&metricName, "metric", "m", "", "Specify metric name")
	gaugeMetricCmd.Flags().Float64VarP(&gaugeMetricThreshold, "threshold", "t", 0, "Specify threshold value for the metric")
	gaugeMetricCmd.Flags().StringVarP(&metricDuration, "duration", "d", "", "Specify duration of query. Ex: 5s, 5m, etc.")
}
