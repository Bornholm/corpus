FROM alpine:3.21 AS runtime

RUN apk add \
    ca-certificates \
    openssl \
    gcompat \
  && update-ca-certificates

COPY corpus-client /usr/local/bin/corpus-client

CMD ["/usr/local/bin/corpus-client"]