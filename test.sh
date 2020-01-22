#!/bin/bash

set -e

docker run -d --name test-fakes3 -p :4567 ${IMAGE_NAME}

sleep 3s

export HOST=$(docker run --rm --net host planitar/base ip -4 a show docker0 | \
  sed 's@^\s*inet \([0-9][0-9.]*\)/.*$@\1@p' -n)

export PORT=$(docker inspect \
  -f '{{ (index (index .NetworkSettings.Ports "4567/tcp") 0).HostPort }}' \
  test-fakes3)

docker run --rm -ti -v `pwd`/bin:/in --net host \
  -e HOST=${HOST} -e PORT=${PORT} planitar/base /in/test
res=$?; \
if [ ${res} -ne 0 ]; then
  docker logs test-fakes3
  docker rm -f test-fakes3
  exit 1
fi

docker rm -f test-fakes3
