.PHONY: build shell docker-build docker run release

TARGET=desktop

RELEASE_DATE=$(shell date +%F)
COMMIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null)
GITSTATUS=$(git status --porcelain --untracked-files=no)
ifneq ($(GITSTATUS),)
  DIRTY=-dirty
endif

LDFLAGS=-ldflags "-X main.Version=${RELEASE_DATE} -X main.CommitHash=${COMMIT_HASH}${DIRTY}"

AWSTOKENSFILE ?= ~/aws.env
-include $(AWSTOKENSFILE)
export GITHUB_USERNAME GITHUB_TOKEN

build:
	go build $(LDFLAGS) -o ${TARGET} main.go

shell: docker-build
	docker run --rm -it -v $(CURDIR):/go/src/github.com/SvenDowideit/${TARGET} ${TARGET} bash

docker-build:
	rm -f ${TARGET}.gz
	docker build \
		--build-arg RELEASE_DATE=$(RELEASE_DATE) \
		--build-arg COMMIT_HASH=$(COMMIT_HASH) \
		-t ${TARGET} .

docker: docker-build
	docker run --name ${TARGET}-build ${TARGET}
	docker cp ${TARGET}-build:/go/src/github.com/SvenDowideit/${TARGET}/${TARGET}.zip .
	docker rm ${TARGET}-build
	rm -f ${TARGET}
	unzip -o ${TARGET}.zip

run:
	./${TARGET} .

trash:
	go get github.com/Shopify/logrus-bugsnag
	go get github.com/Sirupsen/logrus
	go get github.com/bugsnag/bugsnag-go
	go get github.com/urfave/cli
	go get github.com/docker/machine/
	go get github.com/rancher/cli/
	go get github.com/rancher/go-rancher/
	go get github.com/blang/semver

release: docker
	# TODO: check that we have upstream master, bail if not
	docker run --rm -it -e GITHUB_TOKEN ${TARGET} \
		github-release release --user SvenDowideit --repo ${TARGET} --tag $(RELEASE_DATE)
	docker run --rm -it -e GITHUB_TOKEN ${TARGET} \
		github-release upload --user SvenDowideit --repo ${TARGET} --tag $(RELEASE_DATE) \
			--name ${TARGET} \
			--file ${TARGET}
	docker run --rm -it -e GITHUB_TOKEN ${TARGET} \
		github-release upload --user SvenDowideit --repo ${TARGET} --tag $(RELEASE_DATE) \
			--name ${TARGET}-osx \
			--file ${TARGET}.app
	docker run --rm -it -e GITHUB_TOKEN ${TARGET} \
		github-release upload --user SvenDowideit --repo ${TARGET} --tag $(RELEASE_DATE) \
			--name ${TARGET}.exe \
			--file ${TARGET}.exe

fmt:
	docker run --rm -it -v $(shell pwd):/data -w /data golang go fmt
	docker run --rm -it -v $(shell pwd):/data -w /data/commands golang go fmt
	docker run --rm -it -v $(shell pwd):/data -w /data/allprojects golang go fmt
	docker run --rm -it -v $(shell pwd):/data -w /data/render golang go fmt
