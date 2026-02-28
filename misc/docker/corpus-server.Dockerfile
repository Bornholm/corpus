FROM alpine:3.23 AS runtime

RUN apk add \
    ca-certificates \
    openssl \
    pandoc \
    gcompat \
    libreoffice \
  && update-ca-certificates

COPY corpus-server /usr/local/bin/corpus-server

ENV CORPUS_STORAGE_DATABASE_DSN=/data/data.sqlite CORPUS_STORAGE_SQLITEVEC_DSN=/data/index.sqlite CORPUS_STORAGE_BLEVE_DSN=/data/index.bleve

VOLUME /data

EXPOSE 3002

CMD ["/usr/local/bin/corpus-server"]