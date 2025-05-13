#!/bin/sh
set -ex 
set -euo pipefail
dir=/usr/src/github.com/pensando/device-metrics-exporter
netns=/var/run/netns

term() {
    killall dockerd
    wait
}

dockerd -s vfs &

trap term INT TERM

mkdir -p ${dir}
mkdir -p ${netns}
mount -o bind /device-metrics-exporter ${dir}
rm -f $dir/.container_ready
export GOFLAGS=-mod=vendor
sysctl -w vm.max_map_count=262144

touch $dir/.container_ready
exec "$@"
