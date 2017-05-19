# XXX no versioning of the docker image
IMAGE_NAME=planitar/fakes3

ifneq ($(NOCACHE),)
  NOCACHEFLAG=--no-cache
endif

.PHONY: build push clean test

build:
	docker build ${NOCACHEFLAG} -t ${IMAGE_NAME} .

push:
ifneq (${IMAGE_TAG},)
	docker tag ${IMAGE_NAME} ${IMAGE_NAME}:${IMAGE_TAG}
	docker push ${IMAGE_NAME}:${IMAGE_TAG}
else
	docker push ${IMAGE_NAME}
endif

clean:
	docker rmi -f ${IMAGE_NAME} || true
	rm -rf bin

test: bin/test
	IMAGE_NAME=${IMAGE_NAME} ./test.sh

bin/test: test.go
	mkdir -p bin
	docker run --rm \
	  -v `pwd`:/src \
	  -v `pwd`/bin:/out \
	  planitar/dev-go /bin/bash -lc ' \
	    pkg="github.com/PlanitarInc/docker-image-fakes3"; \
	    gobldcp $$pkg docker-image-fakes3 /out/test; \
	'
