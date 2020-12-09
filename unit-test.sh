#!/bin/bash

mkdir -p test-reports
docker run --rm -v $PWD:/go/src/github.com/qvantel/nerd golang:1.15.6-alpine3.12 \
  sh -c 'apk add --no-cache git gcc musl-dev && \
  go get gotest.tools/gotestsum && \
  gotestsum --junitfile /go/src/github.com/qvantel/nerd/test-reports/unit-tests.xml /go/src/github.com/qvantel/nerd/...'