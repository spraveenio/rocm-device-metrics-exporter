#!/bin/bash

if [ -z $RELEASE ]
then
  echo "RELEASE is not set, return"

  if [ -z ${DOCKERHUB_TOKEN-} ]
  then
      echo "DOCKERHUB_TOKEN is not set"
  else
      echo "DOCKERHUB_TOKEN is set"
  fi

  exit 0
fi

tag_prefix="${RELEASE%-*}"

if [ "$tag_prefix" == "exporter-0.0.1" ]; then
  tag="latest"
else
  tag="$tag_prefix"
fi

echo "Copying device-metrics-exporter artifacts and pushing docker image with tag:$tag"

setup_dir () {
    ls -al /device-metrics-exporter/
    BUNDLE_DIR=/device-metrics-exporter/output/
    mkdir -p $BUNDLE_DIR
}

copy_artifacts () {
    # copy docker image ubi9.4
    cp /device-metrics-exporter/docker/device-metrics-exporter-latest.tar.gz $BUNDLE_DIR/device-metrics-exporter-latest-$RELEASE.tar.gz
    # copy docker image azure coreos 3
    cp /device-metrics-exporter/docker/exporter-latest-azure.tar.gz $BUNDLE_DIR/device-metrics-exporter-latest-azure-$RELEASE.tar.gz
    # copy docker mock image
    cp /device-metrics-exporter/docker/device-metrics-exporter-mock-latest.tgz $BUNDLE_DIR/device-metrics-exporter-mock-latest-$RELEASE.tar.gz
    # copy debian ubuntu 22.04 package
    cp /device-metrics-exporter/bin/amdgpu-exporter_1.2.0_ubuntu_22.04_amd64.deb  $BUNDLE_DIR/amdgpu-exporter-$RELEASE-1.2.0_ubuntu_22.04_amd64.deb
    # copy debian ubuntu 24.04 package
    cp /device-metrics-exporter/bin/amdgpu-exporter_1.2.0_ubuntu_24.04_amd64.deb  $BUNDLE_DIR/amdgpu-exporter-$RELEASE-1.2.0_ubuntu_24.04_amd64.deb
    # copy helm charts
    cp /device-metrics-exporter/helm-charts/device-metrics-exporter-charts-*.tgz $BUNDLE_DIR/device-metrics-exporter-charts-$RELEASE-v1.2.0.tgz
    # copy techsupport scripts
    cp /device-metrics-exporter/tools/techsupport_dump.sh $BUNDLE_DIR/
    # list the artifacts copied out
    ls -la $BUNDLE_DIR
}

docker_push () {
    EXPORTER_IMAGE_URL=registry.test.pensando.io:5000/device-metrics-exporter

    # rhel 9.4 image push
    docker load -i /device-metrics-exporter/docker/device-metrics-exporter-latest.tar.gz
    docker inspect $EXPORTER_IMAGE_URL:latest | grep "HOURLY"
    docker tag $EXPORTER_IMAGE_URL:latest $EXPORTER_IMAGE_URL:$tag
    docker push $EXPORTER_IMAGE_URL:$tag

    # azurelinux3 image push
    azuretag="$tag-azl3"
    docker load -i /device-metrics-exporter/docker/device-metrics-exporter-latest-azure.tar.gz
    docker inspect $EXPORTER_IMAGE_URL:latest | grep "HOURLY"
    docker tag $EXPORTER_IMAGE_URL:latest $EXPORTER_IMAGE_URL:$azuretag
    docker push $EXPORTER_IMAGE_URL:$azuretag

    if [ -z $DOCKERHUB_TOKEN ]
    then
      echo "DOCKERHUB_TOKEN is not set"
    else
      docker login --username=shreyajmeraamd --password-stdin <<< $DOCKERHUB_TOKEN
      # rhel 9.4
      docker tag $EXPORTER_IMAGE_URL:$tag amdpsdo/device-metrics-exporter:$RELEASE
      docker push amdpsdo/device-metrics-exporter:$RELEASE
      # azure linux3
      docker tag $EXPORTER_IMAGE_URL:$azuretag amdpsdo/device-metrics-exporter:$RELEASE-azl3
      docker push amdpsdo/device-metrics-exporter:$RELEASE-azl3
    fi
}

setup () {
    setup_dir
    copy_artifacts
    docker_push
}

upload () {
    cd $BUNDLE_DIR
    find . -type f -print0 | while IFS= read -r -d $'\0' file;
      do asset-push builds hourly-device-metrics-exporter $RELEASE "$file" ;
      if [ $? -ne 0 ]; then
        exit 1
      fi
    done
}

main () {
  setup
  upload
}

main
