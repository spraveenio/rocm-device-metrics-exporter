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
# script to generate tarball with exporter docker, entrypoint script
# and docker_run script to create the docker container

print_help () {
    echo "This script can be used to build a exporter container"
    echo
    echo "Syntax: $0 [-s -n]"
    echo "options:"
    echo "-h    print help"
    echo "-s    prepare a release tarball image"
    echo "-p    publish to registry"
    echo "-n    docker image name"
}

VER=v1
SAVE_IMAGE=0
PUBLISH_IMAGE=0
DOCKER_REGISTRY="registry.test.pensando.io:5000/device-metrics-exporter/"
#TODO : rename to official exporter later
EXPORTER_IMAGE="exporter"

IMAGE_URL="${DOCKER_REGISTRY}${EXPORTER_IMAGE}:${VER}"

while getopts ":h:sn:p" option; do
    case $option in
        h)
            print_help
            exit ;;
        s)
            echo "saving image option set"
            SAVE_IMAGE=1
            ;;
        n)
            DOCKER_IMAGE_NAME=$OPTARG ;;
        p)
            echo "publish image option set"
            PUBLISH_IMAGE=1
            ;;
        \?)
            echo "Error: Invalid argument"
            exit ;;
    esac
done

IMAGE_DIR=$(pwd)/obj

rm -rf $IMAGE_DIR
mkdir -p $IMAGE_DIR

# create symlinks for gpuagent and gpuctl binaries and librocm_smi64.so.2 in the
# docker directory so that they can be added to the container
if [ "$MOCK" == "1" ]; then
    gunzip -c $TOP_DIR/assets/gpuagent_mock.bin.gz > $TOP_DIR/docker/gpuagent
else
    gunzip -c $TOP_DIR/assets/gpuagent_static.bin.gz > $TOP_DIR/docker/gpuagent
fi
chmod +x $TOP_DIR/docker/gpuagent
cp -r $TOP_DIR/assets/amd_smi_lib/x86_64/lib $TOP_DIR/docker/smilib
ln -f $TOP_DIR/assets/gpuctl.gobin $TOP_DIR/docker/gpuctl
ln -f $TOP_DIR/bin/amd-metrics-exporter $TOP_DIR/docker/amd-metrics-exporter
ln -f $TOP_DIR/bin/metricsclient $TOP_DIR/docker/metricsclient

# build docker image tarball from the docker file
cd $TOP_DIR/docker
rm -rf exporter-docker*.tgz
if [ $PUBLISH_IMAGE == 1 ]; then
    echo "publishing exporter image to $IMAGE_URL"
    docker build --build-arg BASE_IMAGE=registry.test.pensando.io:5000/ubi9/ubi-minimal:9.4 -t $IMAGE_URL . -f Dockerfile.exporter-release && docker push $IMAGE_URL
    if [ $? -eq 0 ]; then
        echo "Successfully published image $IMAGE_URL"
    else
        echo "Failed to publish docker image"
        exit $?
    fi
else
    echo "building exporter image to $IMAGE_URL"
    docker build --build-arg BASE_IMAGE=registry.test.pensando.io:5000/ubi9/ubi-minimal:9.4 -t $IMAGE_URL . -f Dockerfile.exporter-release && docker save -o exporter-docker-$VER.tar $IMAGE_URL
    if [ $? -eq 0 ]; then
        gzip exporter-docker-$VER.tar
        mv exporter-docker-$VER.tar.gz exporter-docker-$VER.tgz
    else
        echo "Failed to build docker image"
        exit $?
    fi
fi

# prepare the final tar ball now
if [ "$SAVE_IMAGE" == 1 ]; then
    echo "Preparing final image ..."
    if [ "$MOCK" == "1" ]; then
        mv exporter-docker-$VER.tgz exporter-mock-latest.tgz 
    else
        mv exporter-docker-$VER.tgz exporter-latest.tgz
    fi
    echo "Image ready in $IMAGE_DIR"
fi

# remove the symlinks we created for the docker image
rm -rf gpuagent gpuctl amd-metrics-exporter smilib metricsclient

exit 0
