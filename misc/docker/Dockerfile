FROM golang:1.23 AS build

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
    make build

FROM alpine:3.21 AS runtime

RUN apk add \
    ca-certificates \
    openssl \
    pandoc \
  && update-ca-certificates

COPY --from=build /src/bin/corpus /usr/local/bin/corpus

CMD ["/usr/local/bin/corpus"]