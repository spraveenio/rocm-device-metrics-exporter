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

// client/client.go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/metricssvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/testsvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/k8sclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func prettyPrintGPUState(resp *metricssvc.GPUStateResponse) {
	if *jout {
		jsonData, err := json.Marshal(resp)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println(string(jsonData))
		return
	}
	fmt.Println("ID\tHealth\tAssociated Workload\t")
	fmt.Println("------------------------------------------------")
	for _, gs := range resp.GPUState {
		fmt.Printf("%v\t%v\t%+v\t\r\n", gs.ID, gs.Health, gs.AssociatedWorkload)
	}
	fmt.Println("------------------------------------------------")
}

func prettyPrint(resp interface{}) {
	jsonData, err := json.Marshal(resp)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(string(jsonData))
	return
}

func prettyPrintErrResponse(resp *metricssvc.GPUErrorResponse) {
	jsonData, err := json.Marshal(resp)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(string(jsonData))
	return
}

func send(socketPath string) error {
	conn, err := grpc.Dial(
		"unix:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Use insecure credentials for simplicity
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	// create a new gRPC echo client through the compiled stub
	client := metricssvc.NewMetricsServiceClient(conn)

	resp, err := client.List(context.Background(), &emptypb.Empty{})
	if err != nil {
		return err
	}

	prettyPrintGPUState(resp)
	return nil
}

func get(socketPath, id string) error {
	conn, err := grpc.Dial(
		"unix:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Use insecure credentials for simplicity
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	// create a new gRPC echo client through the compiled stub
	client := metricssvc.NewMetricsServiceClient(conn)

	// send an metricssvcrequest
	gpuReq := &metricssvc.GPUGetRequest{
		ID: []string{id},
	}
	_, err = client.GetGPUState(context.Background(), gpuReq)
	if err != nil {
		return err
	}

	// send an metricssvcrequest
	resp, err := client.GetGPUState(context.Background(),
		&metricssvc.GPUGetRequest{ID: gpuReq.ID})
	if err != nil {
		return err
	}
	prettyPrintGPUState(resp)

	return nil
}

func sendTestResult(socketPath string) error {
	conn, err := grpc.Dial(
		"unix:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Use insecure credentials for simplicity
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	// create a new gRPC echo client through the compiled stub
	client := testsvc.NewTestServiceClient(conn)

	req := &testsvc.TestPostRequest{
		ID:   "uuid",
		Name: "mock_test",
	}
	resp, err := client.SubmitTestResult(context.Background(), req)
	if err != nil {
		return err
	}
	prettyPrint(resp)

	return nil

}

func listTestResult(socketPath string) error {
	conn, err := grpc.Dial(
		"unix:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Use insecure credentials for simplicity
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	// create a new gRPC echo client through the compiled stub
	client := testsvc.NewTestServiceClient(conn)

	resp, err := client.List(context.Background(), &emptypb.Empty{})
	if err != nil {
		return err
	}
	prettyPrint(resp)

	return nil

}
func setError(socketPath, filepath string) error {

	// send an metricssvcrequest
	gpuUpdate := &metricssvc.GPUErrorRequest{}
	eccConfigs, err := ioutil.ReadFile(filepath)
	if err != nil {
		fmt.Printf("err: %+v", err)
		return err
	} else {
		err = json.Unmarshal(eccConfigs, gpuUpdate)
		if err != nil {
			fmt.Printf("err: %+v", err)
			return err
		}
	}

	conn, err := grpc.Dial(
		"unix:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Use insecure credentials for simplicity
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	// create a new gRPC echo client through the compiled stub
	client := metricssvc.NewMetricsServiceClient(conn)

	resp, err := client.SetError(context.Background(), gpuUpdate)
	if err != nil {
		return err
	}

	prettyPrintErrResponse(resp)

	return nil
}

var jout = flag.Bool("json", false, "output in json format")

func main() {
	var (
		socketPath   = flag.String("socket", globals.MetricsSocketPath, "metrics grpc socket path")
		getOpt       = flag.Bool("get", false, "get health status of gpu")
		setId        = flag.String("id", "1", "gpu id")
		getNodeLabel = flag.Bool("label", false, "get k8s node label")
		//sendTest     = flag.Bool("test", false, "send mock test result")
		//listTest     = flag.Bool("list", false, "list all test results from server")
		setEcc       = flag.Bool("ecc", false, "set mock ecc error")
		eccFile      = flag.String("ecc-file-path", "", "json ecc err file")
	)
	flag.Parse()

	if *getOpt {
		err := get(*socketPath, *setId)
		if err != nil {
			log.Fatalf("request failed :%v", err)
		}
	} else {
		err := send(*socketPath)
		if err != nil {
			log.Fatalf("request failed :%v", err)
		}
	}

	if *getNodeLabel {
		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			fmt.Println("not a k8s deployment")
			return
		}
		kc := k8sclient.NewClient()
		labels, err := kc.GetNodelLabel(nodeName)
		if err != nil {
			fmt.Printf("err: %+v", err)
			return
		}
		fmt.Printf("node[%v] labels[%+v]", nodeName, labels)
	}

	/*
	if *sendTest {
		sendTestResult(*socketPath)
	}

	if *listTest {
		listTestResult(*socketPath)
	}
	*/

	if *setEcc {
		if *eccFile == "" {
			fmt.Println("invalid ecc error file path")
			return

		}
		setError(*socketPath, *eccFile)
	}
}
