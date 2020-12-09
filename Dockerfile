# 1) BUILD DEPENDENCIES
FROM golang:1.15.6-alpine3.12 AS build-go
RUN apk --no-cache add git
ENV D=/go/src/github.com/qvantel/nerd
ADD ./ $D/
RUN cd $D && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' ./cmd/nerd/ && cp nerd /tmp/

# 2) BUILD FINAL IMAGE
FROM scratch
WORKDIR /opt/docker
COPY --from=build-go /tmp/nerd /opt/docker/
ENV GIN_MODE=release \
    VERSION=0.2.1
EXPOSE 5400
ENTRYPOINT [ "./nerd" ]