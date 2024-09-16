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
