FROM golang

# Simplify making releases
RUN apt-get update \
	&& apt-get install -yq zip bzip2
RUN wget -O github-release.bz2 https://github.com/aktau/github-release/releases/download/v0.6.2/linux-amd64-github-release.tar.bz2 \
        && tar jxvf github-release.bz2 \
        && mv bin/linux/amd64/github-release /usr/local/bin/ \
        && rm github-release.bz2

ENV TARGET desktop
ENV GOPATH /go
ENV USER root

WORKDIR /go/src/github.com/SvenDowideit/${TARGET}

RUN go get github.com/Sirupsen/logrus \
    && go get github.com/urfave/cli \
    && go get github.com/cloudfoundry-incubator/candiedyaml \
    && go get github.com/google/go-github/github \
    && go get golang.org/x/oauth2 \
    && go get github.com/miekg/mmark \
    && go get github.com/blang/semver \
    && go get github.com/kardianos/osext

ADD . /go/src/github.com/SvenDowideit/${TARGET}
RUN go get -d -v
RUN go test -v ./...

ARG RELEASE_DATE="developer build"
ARG COMMIT_HASH="unknown"

RUN go build -o ${TARGET} -ldflags "-X main.Version=${RELEASE_DATE} -X main.CommitHash=${COMMIT_HASH}" main.go \
	&& GOOS=windows GOARCH=amd64 go build -o ${TARGET}.exe -ldflags "-X main.Version=${RELEASE_DATE} -X main.CommitHash=${COMMIT_HASH}" main.go \
	&& GOOS=darwin GOARCH=amd64 go build -o ${TARGET}.app -ldflags "-X main.Version=${RELEASE_DATE} -X main.CommitHash=${COMMIT_HASH}" main.go \
	&& zip ${TARGET}.zip ${TARGET} ${TARGET}.exe ${TARGET}.app
