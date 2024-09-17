# device-metrics-exporter
Device Metrics Exporter exports metrics from AMD GPUs to collectors like Prometheus.

Build and Run Instructions

1. Build amdexporter application binary
-  Run the following make target in the TOP directory:
   
   make all

   This will also generate the required protos to build the amdexporter application
   binary.

2. Build exporter container
-  Run the following make target in the TOP directory:
   
   make docker

3. Run exporter container
- To run the exporter container after building the container in $TOPDIR/docker, run:
  
   cd $TOPDIR
  ./docker/start_exporter.sh -d docker/exporter-docker-v1.tgz

4. Custom metrics config
- To run the exporter with config mount the /etc/metrics/config.json on the
  exporter container 
	- create log directories
	#mkdir -p  exporter/var/run
	- create your config in exporter/configs/config.json
	- start docker container
  	#docker run --rm -itd --privileged --mount type=bind,source=./exporter/var/run,target=/var/run -e PATH=$PATH:/home/amd/bin/ -p 5000:5000 -v ./exporter/configs:/etc/metrics/ --name exporter registry.test.pensando.io:5000/device-metrics-exporter/rocm-metrics-exporter:v1 bash

5. Metrics Config formats
- a json file with the following keys are expected
    - Field
        array of string specifying what field to be exported
        present in internal/amdgpu/proto/fields.proto:GPUMetricField
    - Label
        GPU_UUID and SERIAL_NUMBER are always set and cannot be removed 
        array of optional label info can be specified in
        internal/amdgpu/proto/fields.proto:GPUMetricLabel
