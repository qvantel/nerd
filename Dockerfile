# 1) BUILD DEPENDENCIES
FROM golang:1.16.0-alpine3.13 AS build-go
RUN apk --no-cache add git
ENV D=/go/src/github.com/qvantel/nerd
ADD ./ $D/
WORKDIR $D
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' ./cmd/nerd/ && \
    cp nerd /tmp/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' ./cmd/fcollect/ && \
    cp fcollect /tmp/

# 2) BUILD FINAL IMAGE
FROM scratch
WORKDIR /opt/docker
COPY --from=build-go /tmp/nerd /tmp/fcollect /opt/docker/
ENV GIN_MODE=release \
    VERSION=0.4.0
EXPOSE 5400
ENTRYPOINT [ "./nerd" ]