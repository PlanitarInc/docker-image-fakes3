#!/bin/bash

set -e

cleanup() {
  docker rm -f test-fakes3 test-bin || true
}
report() {
  echo ""
  echo "##### FAKES3 LOGS #####"
  docker logs test-fakes3
  echo ""
  echo "##### TEST LOGS #####"
  docker logs test-bin
}
trap report ERR
trap cleanup EXIT


docker run -d --name test-fakes3 -p :4567 ${IMAGE_NAME}

sleep 3s

export HOST=$(docker run --rm --net host planitar/base ip -4 a show docker0 | \
  sed 's@^\s*inet \([0-9][0-9.]*\)/.*$@\1@p' -n)

export PORT=$(docker inspect \
  -f '{{ (index (index .NetworkSettings.Ports "4567/tcp") 0).HostPort }}' \
  test-fakes3)

docker run \
  --name test-bin \
  -v `pwd`/bin:/in \
  --net host \
  -e HOST=${HOST} \
  -e PORT=${PORT} \
  planitar/base \
    /in/test
