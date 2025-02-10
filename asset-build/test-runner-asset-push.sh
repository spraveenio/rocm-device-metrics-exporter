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

echo "Copying test-runner artifacts and pushing docker image with tag:$tag"

setup_dir () {
    ls -al /device-metrics-exporter/
    BUNDLE_DIR=/device-metrics-exporter/output/
    mkdir -p $BUNDLE_DIR
}

copy_artifacts () {
    # copy docker mock image
    cp /device-metrics-exporter/docker/testrunner/test-runner-latest.tar.gz $BUNDLE_DIR/test-runner-latest-$RELEASE.tar.gz
    # list the artifacts copied out
    ls -la $BUNDLE_DIR
}

docker_push () {
    TEST_RUNNER_IMAGE_URL=registry.test.pensando.io:5000/test-runner/test-runner
    docker load -i /device-metrics-exporter/docker/testrunner/test-runner-latest.tar.gz
    docker inspect $TEST_RUNNER_IMAGE_URL:latest | grep "HOURLY"
    docker tag $TEST_RUNNER_IMAGE_URL:latest $TEST_RUNNER_IMAGE_URL:$tag
    docker push $TEST_RUNNER_IMAGE_URL:$tag

    if [ -z $DOCKERHUB_TOKEN ]
    then
      echo "DOCKERHUB_TOKEN is not set"
    else
      docker login --username=shreyajmeraamd --password-stdin <<< $DOCKERHUB_TOKEN
      docker tag $TEST_RUNNER_IMAGE_URL:$tag amdpsdo/test-runner:$RELEASE
      docker push amdpsdo/test-runner:$RELEASE
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
