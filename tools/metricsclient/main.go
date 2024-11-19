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
	"flag"
	"fmt"
	"log"

	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/metricssvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/globals"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func prettyPrintGPUState(resp *metricssvc.GPUStateResponse) {
	fmt.Println("ID\tHealth\tAssociated Workload\t")
	fmt.Println("------------------------------------------------")
	for _, gs := range resp.GPUState {
		fmt.Printf("%v\t%v\t%+v\t\r\n", gs.ID, gs.Health, gs.AssociatedWorkload)
	}
	fmt.Println("------------------------------------------------")
}

func send() error {
	socketPath := globals.MetricsSocketPath
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

func set(id, state string) error {
	socketPath := globals.MetricsSocketPath
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
	gpuUpdate := &metricssvc.GPUUpdateRequest{
		ID:     []string{id},
		Health: []string{state},
	}
	_, err = client.SetGPUHealth(context.Background(), gpuUpdate)
	if err != nil {
		return err
	}

	// send an metricssvcrequest
	resp, err := client.GetGPUState(context.Background(),
		&metricssvc.GPUStateRequest{ID: gpuUpdate.ID})
	if err != nil {
		return err
	}
	prettyPrintGPUState(resp)

	return nil
}

func main() {
	var (
		setOpt   = flag.Bool("set", false, "send set req")
		setId    = flag.String("id", "1", "send gpu id")
		setState = flag.String("state", "healthy", "[healthy, unhealthy, unknown]")
	)
	flag.Parse()

	if *setOpt {
		err := set(*setId, *setState)
		if err != nil {
			log.Fatalf("send failed")
		}
	}

	err := send()
	if err != nil {
		log.Fatalf("send failed")
	}
}
