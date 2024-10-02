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
}

copy_artifacts () {
    # copy amd-metrics-exporter binary
    cp /device-metrics-exporter/amd-metrics-exporter $BUNDLE_DIR/amd-metrics-exporter.gobin
    # copy docker image
    cp /device-metrics-exporter/docker/obj/exporter-release-*.tgz  $BUNDLE_DIR/
    # copy docker mock image
    cp /device-metrics-exporter/docker/obj/exporter-release-mock-*.tgz  $BUNDLE_DIR/
    # copy debian package
    cp /device-metrics-exporter/docker/bin/amdgpu-exporter_*_amd64.deb  $BUNDLE_DIR/
    # list the artifacts copied out
    ls -la $BUNDLE_DIR
}

setup () {
    setup_dir
    copy_artifacts
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
