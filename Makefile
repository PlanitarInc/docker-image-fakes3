# XXX no versioning of the docker image
IMAGE_NAME=planitar/fakes3

ifneq ($(NOCACHE),)
  NOCACHEFLAG=--no-cache
endif

.PHONY: build push clean test

build:
	docker build ${NOCACHEFLAG} -t ${IMAGE_NAME} .

push:
	docker push ${IMAGE_NAME}

clean:
	docker rmi -f ${IMAGE_NAME} || true
	rm -rf bin

test: bin/test
	docker run -d --name test-fakes3 -v `pwd`:/s3 -p :4567 ${IMAGE_NAME}
	sleep 3s
	export PORT=`docker inspect \
	  -f '{{ (index (index .NetworkSettings.Ports "4567/tcp") 0).HostPort }}' \
	  test-fakes3`; \
	./bin/test; \
	res=$$?; \
	if [ $$res -ne 0 ]; then \
	  docker logs test-fakes3; \
	  docker rm -f test-fakes3; \
	  false; \
	fi
	docker rm -f test-fakes3

bin/test: test.go
	mkdir -p bin
	docker run --rm -v `pwd`/bin:/out planitar/dev-go /bin/bash -lc ' \
	  go get "github.com/PlanitarInc/docker-image-fakes3" && \
	  cp $$GOPATH/bin/docker-image-fakes3 /out/test \
	'
