#!/bin/bash -e



#
# Copyright(C) Advanced Micro Devices, Inc. All rights reserved.
#
# You may not use this software and documentation (if any) (collectively,
# the "Materials") except in compliance with the terms and conditions of
# the Software License Agreement included with the Materials or otherwise as
# set forth in writing and signed by you and an authorized signatory of AMD.
# If you do not have a copy of the Software License Agreement, contact your
# AMD representative for a copy.
#
# You agree that you will not reverse engineer or decompile the Materials,
# in whole or in part, except as allowed by applicable law.
#
# THE MATERIALS ARE DISTRIBUTED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OR
# REPRESENTATIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED.
#


#
# script to be run on the x86 host server to start a exporter container

# function to print help string
print_help () {
    echo "This script can be used to start a exporter container"
    echo
    echo "Syntax: $0 -d <docker-image-tarball-location> | -s [-n <docker-image-name>] [-p <external-port>]"
    echo "options:"
    echo "-h    print help"
    echo "-d    path for exporter docker image tarball"
    echo "-s    skip loading of exporter docker image tarball"
    echo "-n    name to be used for docker instance"
    echo "-p    external port to map to the exporter's internal port (default: 5000)"
}

VER=v1
LOAD_IMAGE=1
EXPORTER_EXTERNAL_LISTENER_PORT=5000
EXPORTER_INTERNAL_LISTENER_PORT=5000
NODE_MGMT_RUN_DIR=$PWD
DOCKER_IMAGE_NAME="exporter:$VER"
DOCKER_INSTANCE_NAME="exporter"

while getopts ":hd:p:sn:" option; do
    case $option in
        h)
            print_help
            exit ;;
        d)
            DOCKER_IMAGE=$OPTARG ;;
        s)
            LOAD_IMAGE=0 ;;
        n)
            DOCKER_INSTANCE_NAME=$OPTARG ;;
        p)
            EXPORTER_EXTERNAL_LISTENER_PORT=$OPTARG ;;
        \?)
            echo "Error: Invalid argument"
            exit ;;
    esac
done

if [ "$LOAD_IMAGE" == 1 ]; then
    if test -z "$DOCKER_IMAGE"; then
        echo "Error: Unable to load exporter docker image, path not specified"
        exit
    fi
    echo "Cleaning up docker image from registry..."
    docker rmi amd/$DOCKER_IMAGE_NAME -f || true
    echo "Loading up docker image specified into registry..."
    docker load -i $DOCKER_IMAGE
fi

# directory on host that is mounted to the container
HOST_DIR=$NODE_MGMT_RUN_DIR/exporter/
# create a dir to hold gpuagent logs
mkdir -p $HOST_DIR/var/run
# mount options to mount the host dir to the container
MOUNT_OPTS=" --mount type=bind,source=$HOST_DIR/var/run,target=/var/run"
# bind gpuagent grpc ports to the container
PORT_OPTS=" -p $EXPORTER_EXTERNAL_LISTENER_PORT:$EXPORTER_INTERNAL_LISTENER_PORT"
echo "Creating docker container..."
docker run --rm -itd --privileged --name $DOCKER_INSTANCE_NAME $PORT_OPTS $MOUNT_OPTS -e PATH=$PATH:/home/amd/bin/ amd/$DOCKER_IMAGE_NAME
exit 0
