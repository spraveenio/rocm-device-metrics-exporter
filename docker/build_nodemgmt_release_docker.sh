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
# script to generate tarball with node management docker, entrypoint script
# and docker_run script to create the docker container

print_help () {
    echo "This script can be used to build a nodemgmt container"
    echo
    echo "Syntax: $0 [-s -n]"
    echo "options:"
    echo "-h    print help"
    echo "-s    prepare a release tarball image"
    echo "-n    docker image name"
}

VER=v1
DOCKER_IMAGE_NAME="amd/nodemgmt:$VER"
SAVE_IMAGE=0

while getopts ":hsn:" option; do
    case $option in
        h)
            print_help
            exit ;;
        s)
            SAVE_IMAGE=1 ;;
        n)
            DOCKER_IMAGE_NAME=$OPTARG ;;
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
gunzip -c $TOP_DIR/asset/gpuagent_static.bin.gz > $TOP_DIR/docker/gpuagent
ln -f $TOP_DIR/asset/gpuctl.gobin $TOP_DIR/docker/gpuctl
ln -f $TOP_DIR/internal/bin/amd-metrics-exporter $TOP_DIR/docker/amd-metrics-exporter

# build docker image tarball from the docker file
cd $TOP_DIR/docker
rm -rf nodemgmt-docker*.tgz
docker build -t $DOCKER_IMAGE_NAME . -f Dockerfile.nodemgmt-release && docker save -o nodemgmt-docker-$VER.tar $DOCKER_IMAGE_NAME
if [ $? -eq 0 ]; then
    gzip nodemgmt-docker-$VER.tar
    mv nodemgmt-docker-$VER.tar.gz nodemgmt-docker-$VER.tgz
else
    echo "Failed to build docker image"
    exit $?
fi

# prepare the final tar ball now
if [ "$SAVE_IMAGE" == 1 ]; then
    echo "Preparing final image ..."
    tar cvzf $IMAGE_DIR/nodemgmt-release-$VER.tgz nodemgmt-docker-$VER.tgz start_nodemgmt.sh
    echo "Image ready in $IMAGE_DIR"
fi

# remove the symlinks we created for the docker image
rm -rf gpuagent gpuctl amd-metrics-exporter

exit 0
