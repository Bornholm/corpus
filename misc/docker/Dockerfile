FROM golang:1.24 AS build

RUN apt-get update \
  && apt-get install -y make git

RUN --mount=type=cache,target=/go/pkg/mod/ \
  --mount=type=bind,source=go.sum,target=go.sum \
  --mount=type=bind,source=go.mod,target=go.mod \
  go mod download -x

WORKDIR /src

COPY . /src

ARG GOCACHE=/root/.cache/go-build

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target="/root/.cache/go-build" \
    --mount=type=cache,target=./tools \
    make goreleaser GORELEASER_ARGS="build --clean --auto-snapshot --single-target"

FROM alpine:3.21 AS runtime

RUN apk add \
    ca-certificates \
    openssl \
    pandoc \
    gcompat \
  && update-ca-certificates

COPY --from=build /src/dist/corpus-server_linux_amd64_v1/corpus-server /usr/local/bin/corpus-server
COPY --from=build /src/dist/corpus-client_linux_amd64_v1/corpus-client /usr/local/bin/corpus-client

CMD ["/usr/local/bin/corpus-server"]