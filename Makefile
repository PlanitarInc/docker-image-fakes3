# XXX no versioning of the docker image

ifneq ($(NOCACHE),)
  NOCACHEFLAG=--no-cache
endif

.PHONY: build push clean test

build:
	docker build ${NOCACHEFLAG} -t planitar/fake-s3 .

push:
	docker push planitar/fake-s3

clean:
	docker rmi -f planitar/fake-s3 || true
	rm -rf bin

test: bin/test
	docker run -d --name test-fakes3 -v `pwd`:/s3 -p 4567:4567 \
	  planitar/fake-s3
	sleep 3s
	./bin/test; \
	res=$$?; \
	if [ $$res -ne 0 ]; then \
	  docker logs test-fakes3; \
	  docker rm -f test-fakes3; \
	  false; \
	fi
	docker rm -f test-fakes3

bin/test:
	mkdir -p bin
	docker run --rm -v `pwd`/bin:/out planitar/dev-go /bin/bash -lc ' \
	  go get "github.com/PlanitarInc/docker-image-fake-s3" && \
	  cp $$GOPATH/bin/docker-image-fake-s3 /out/test \
	'
