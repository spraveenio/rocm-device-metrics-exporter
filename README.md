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
   ```
	# mkdir -p  exporter/var/run
   	# mkdir -p exporter/configs
   ```
	- create your config in exporter/configs/config.json
	- start docker container
   ```
  	#docker run --rm -itd --privileged --mount type=bind,source=./exporter/var/run,target=/var/run -e PATH=$PATH:/home/amd/bin/ -p 5000:5000 -v ./exporter/configs:/etc/metrics/ --name exporter registry.test.pensando.io:5000/device-metrics-exporter/rocm-metrics-exporter:v1 bash
   ```
5. Metrics Config formats
- a json file with the following keys are expected
    - Field
        array of string specifying what field to be exported
        present in internal/amdgpu/proto/fields.proto:GPUMetricField
    - Label
        GPU_UUID and SERIAL_NUMBER are always set and cannot be removed 
        array of optional label info can be specified in
        internal/amdgpu/proto/fields.proto:GPUMetricLabel

6. Run prometheus (Testing)
   ```
	docker run -p 9090:9090 -v ./example/prometheus.yml:/etc/prometheus/prometheus.yml -v prometheus-data:/prometheus prom/prometheus

7. Install Grafana (Testing)
    - installation
    ```
    https://grafana.com/docs/grafana/latest/setup-grafana/installation/debian/
    #sudo apt-get install -y apt-transport-https software-properties-common wget
    #sudo mkdir -p /etc/apt/keyrings/
    #wget -q -O - https://apt.grafana.com/gpg.key | gpg --dearmor | sudo tee /etc/apt/keyrings/grafana.gpg > /dev/null
    #echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" | sudo tee -a /etc/apt/sources.list.d/grafana.list
    #sudo apt-get update
    #sudo apt-get install grafana

    ```
    - running
    ```
    sudo systemctl daemon-reload
    sudo systemctl start grafana-server
    sudo systemctl status grafana-server
    ```

	```
