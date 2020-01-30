# XXX no versioning of the docker image
IMAGE_NAME=planitar/fakes3
PLNTR_FAKES3_VERSION=1.0.0.pre.11

ifneq ($(NOCACHE),)
  NOCACHEFLAG=--no-cache
endif

.PHONY: build push clean test

build:
	docker build ${NOCACHEFLAG} \
	  --build-arg PLNTR_FAKES3_VERSION=${PLNTR_FAKES3_VERSION} \
	  -t ${IMAGE_NAME} .

push:
ifneq (${IMAGE_TAG},)
	docker tag ${IMAGE_NAME} ${IMAGE_NAME}:${IMAGE_TAG}
	docker push ${IMAGE_NAME}:${IMAGE_TAG}
else
	docker push ${IMAGE_NAME}
endif
	docker tag ${IMAGE_NAME} ${IMAGE_NAME}:${PLNTR_FAKES3_VERSION}
	docker push ${IMAGE_NAME}:${PLNTR_FAKES3_VERSION}

clean:
	docker rmi -f ${IMAGE_NAME} || true
	rm -rf bin

test: bin/test
	IMAGE_NAME=${IMAGE_NAME} ./test.sh

bin/test: test-bin/main.go
	mkdir -p bin
	docker run --rm \
	  -v `pwd`:/src \
	  -v `pwd`/bin:/out \
	  planitar/dev-go bash -c ' \
	    cd /src/test-bin/; \
	    go build -o /out/test .; \
	  '
