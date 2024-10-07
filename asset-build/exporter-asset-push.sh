#!/bin/bash

if [ -z $RELEASE ]
then
  echo "RELEASE is not set, return"
  exit 0
fi

echo "Copying device-metrics-exporter artifacts..."

setup_dir () {
    ls -al /device-metrics-exporter/
    BUNDLE_DIR=/device-metrics-exporter/output/
    mkdir -p $BUNDLE_DIR
    UPLOAD_DIR=/device-metrics-exporter/upload/
    mkdir -p $UPLOAD_DIR
}

copy_artifacts () {
    # copy docker image
    cp /device-metrics-exporter/docker/obj/exporter-release-*.tgz  $BUNDLE_DIR/
    cp /device-metrics-exporter/docker/obj/exporter-release-v1.tgz  $UPLOAD_DIR/
    # copy docker mock image
    cp /device-metrics-exporter/docker/obj/exporter-release-mock-*.tgz  $BUNDLE_DIR/
    # copy debian package
    cp /device-metrics-exporter/bin/amdgpu-exporter_*_amd64.deb  $BUNDLE_DIR/
    # list the artifacts copied out
    ls -la $BUNDLE_DIR
}

docker_build_push () {
    cd $UPLOAD_DIR/
    EXPORTER_IMAGE_URL=registry.test.pensando.io:5000/device-metrics-exporter/exporter:latest
    tar xzf exporter-release-v1.tgz
    ls -la $UPLOAD_DIR/
    docker load -i exporter-docker-v1.tgz
    docker tag amd/exporter:v1 $EXPORTER_IMAGE_URL
    echo "FROM $EXPORTER_IMAGE_URL" | docker build --label HOURLY_TAG=$RELEASE -t "$EXPORTER_IMAGE_URL" -
    docker push $EXPORTER_IMAGE_URL
}

setup () {
    setup_dir
    copy_artifacts
    docker_build_push
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
exit 0
