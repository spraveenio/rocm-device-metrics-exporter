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

echo "Copying device-metrics-exporter artifacts..."
setup_dir () {
    ls -al /device-metrics-exporter/
    BUNDLE_DIR=/device-metrics-exporter/output/
    mkdir -p $BUNDLE_DIR
}

copy_artifacts () {
    # copy docker image
    cp /device-metrics-exporter/docker/exporter-latest.tar.gz $BUNDLE_DIR/
    # copy docker mock image
    cp /device-metrics-exporter/docker/exporter-mock-latest.tgz $BUNDLE_DIR/
    # copy debian package
    cp /device-metrics-exporter/bin/amdgpu-exporter_*_amd64.deb  $BUNDLE_DIR/
    # copy helm charts
    cp /device-metrics-exporter/helm-charts/amdgpu-metrics-exporter-charts-*.tgz $BUNDLE_DIR/
    # copy techsupport scripts
    cp /device-metrics-exporter/tools/techsupport_dump.sh $BUNDLE_DIR/
    # list the artifacts copied out
    ls -la $BUNDLE_DIR
}

docker_push () {
    EXPORTER_IMAGE_URL=registry.test.pensando.io:5000/device-metrics-exporter/exporter:latest
    docker load -i /device-metrics-exporter/docker/exporter-latest.tar.gz
    docker inspect $EXPORTER_IMAGE_URL | grep "HOURLY"
    docker push $EXPORTER_IMAGE_URL

    if [ -z $DOCKERHUB_TOKEN ]
    then
      echo "DOCKERHUB_TOKEN is not set"
    else
      docker tag $EXPORTER_IMAGE_URL amdpsdo/device-metrics-exporter:latest
      docker login --username=shreyajmeraamd --password-stdin <<< $DOCKERHUB_TOKEN
      docker push amdpsdo/device-metrics-exporter:latest
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
