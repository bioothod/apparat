#!/usr/bin/env make

GOROOT=/usr/local/go
#GOROOT=/usr/local/go1.6
#GOROOT=/usr/local/go1.5.3

apparat=github.com/bioothod/apparat
apparat_common=${apparat}/services/common

BUILD_DATE=$(shell date "+%Y-%m-%d/%H:%M:%S/%z")

GO_LDFLAGS=-ldflags "-X ${apparat_common}.BuildDate=${BUILD_DATE} \
	-X ${apparat_common}.LastCommit=$(shell git rev-parse --short HEAD) \
	-X ${apparat_common}.EllipticsGoLastCommit=$(shell GIT_DIR=${GOPATH}/src/github.com/bioothod/elliptics-go/.git git rev-parse --short HEAD)"

.DEFAULT: build
.PHONY: build

APPARAT_BINARIES := auth_server index_server io_server aggregator_server

all: build

build:
	rm -f ${APPARAT_BINARIES}
	for server in ${APPARAT_BINARIES}; do \
		base=`echo $${server} | awk -F "_server" {'print $$1'}` ; \
		${GOROOT}/bin/go build -o $${server} $${GO_LDFLAGS} servers/$${base}/$${base}.go; \
	done

install: build
	cp -rf ${APPARAT_BINARIES} ${GOPATH}/bin/
