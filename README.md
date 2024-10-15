# device-metrics-exporter
Device Metrics Exporter exports metrics from AMD GPUs to collectors like Prometheus.

#help
```
Usage of bin/amd-metrics-exporter:
  -agent-grpc-port int
      Agent GRPC port (default 50061)
  -amd-metrics-config string
      AMD metrics exporter config file (default "/etc/metrics/config.json")
```

## Build and Run Instructions

### 1. Build amdexporter application binary

-  Run the following make target in the TOP directory. This will also generate the required protos to build the amdexporter application
   	binary.
   		`make all`
   	

### 2. Build exporter container
-  Run the following make target in the TOP directory:
   
   	`make docker`
### 3. Run exporter
  - docker environment
    - To run the exporter container after building the container in $TOPDIR/docker, run:
      
    ```
    cd $TOPDIR
    ./docker/start_exporter.sh -d docker/exporter-docker-v1.tgz
    ```
      
    - To run the exporter from docker registery
    ```
    docker run --rm -itd --privileged --mount type=bind,source=./,target=/var/run -e PATH=$PATH:/home/amd/bin/ -p 5000:5000 --name exporter 		registry.test.pensando.io:5000/device-metrics-exporter/rocm-metrics-exporter:v1 bash
    ```
 - ubuntu linux debian package
   - Supported ROCM versions : 6.2.0 and up
   - prerequistes
     - dkms installated on the system
     - rdc service is expected to be up and running with supported versions
       only
       - sample rdc.service is available in example/rdc.service

  - Services run on following default ports. These can be changed by updating
    the respective service file with the below option
    
    gpuagent - default port 50061 : changing this port would require amd-metrics-exporter
    to be configured with the port as these services are dependent
    ```
    gpuagent -p <grpc_port>
    ```

    exporter http port is configurable through the config file ServerPort
    filed in /etc/metrics/config.json : please refer to the example/export_configs.json
    ```
    amd-metrics-exporter - defualt port 5000
        -agent-grpc-port <grpc_port>
    ```
        

  - if running unsupported rocm then the behavior is undefined and some metric fields
    may not work as intended
    update the LD_LIBRARY_PATH in '/usr/local/etc/metrics/gpuagent.conf' to
    proper library location after installation and before starting the
    services. the following libraries must be installed onto the new Library
    path or the system with below command
        `apt-get install -y libdrm libdrm-amdgpu1`

  - installation package
   `$ dpkg -i amdgpu-exporter_0.1_amd64.deb`

  - default config file path /etc/metrics/config.json
  - to change to a custom file, update
    /lib/systemd/system/amd-metrics-exporter.service
    ExecStart=/usr/local/bin/amd-metrics-exporter -f <custom_config_path>


  - enable on system bootup (Optional)
    ```
    systemctl enable gpuagent.service
    systemctl enable amd-metrics-exporter.service
    ```

  - starting services
    ```
    systemctl start gpuagent.service
    systemctl start amd-metrics-exporter.service
    ```

  - stopping service
    ```
    systemctl stop gpuagent.service
    systemctl stop amd-metrics-exporter.service
    ```

  - uninstall package
    ```
    apt-get remove amdgpu-exporter
    ```

  - slurm lua plugin file for metrics job id integrations, this can be copied
    onto the slurm plugin directory to job labels on metrics.
    path : `/usr/local/etc/metrics/pensando.lua`
    proto : `/usr/local/etc/metrics/plugin.proto`

### 4. Custom metrics config
- To run the exporter with config mount the /etc/metrics/config.json on the
  exporter container 
	- create your config in config.json
	- start docker container
   ```
  	#docker run --rm -itd --privileged --mount type=bind,source=./,target=/var/run -e PATH=$PATH:/home/amd/bin/ -p 5000:5000 -v ./config.json:/etc/metrics/config.json --name exporter registry.test.pensando.io:5000/device-metrics-exporter/exporter:latest bash
   ```
### 5. Metrics Config formats
- a json file with the following keys are expected
    - Field
        array of string specifying what field to be exported
        present in internal/amdgpu/proto/fields.proto:GPUMetricField
    - Label
        GPU_UUID and SERIAL_NUMBER are always set and cannot be removed 
        array of optional label info can be specified in
        internal/amdgpu/proto/fields.proto:GPUMetricLabel
### 6. Slurm integration
Metrics exporter uses SPANK((Slurm Plug-in Architecture for Node and job (K)control)  plugin to collect job metrics
- Configure SPANK config, plugstack.conf(default) on  worker nodes
- Copy metrics exporter plugin files from /etc/metrics/slurm to slurm config (/etc/slurm)
- Restart slurmd service
- Include JOB_ID in exported labels (config.json)

metrics will be reported with slurm JOB_IDs, example
```
gpu_edge_temperature{CARD_MODEL="0xc34",DRIVER_VERSION="6.8.5",GPU_ID="0",GPU_UUID="0beb0a09-4200-4242-0e05-67bf583b4c72",JOB_ID="32",SERIAL_NUMBER="692251001124"} 32
```



### 7. Run prometheus (Testing)
   ```
	docker run -p 9090:9090 -v ./example/prometheus.yml:/etc/prometheus/prometheus.yml -v prometheus-data:/prometheus prom/prometheus
   ```
### 8. Install Grafana (Testing)
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
