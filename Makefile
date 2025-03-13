SHELL := /bin/bash

GIT_SHORT_VERSION ?= $(shell git describe --tags --abbrev=8 --always)
GIT_LONG_VERSION ?= $(shell git describe --tags --abbrev=8 --dirty --always --long)
LDFLAGS ?= -w -s \
	-X 'github.com/bornholm/corpus/internal/build.ShortVersion=$(GIT_SHORT_VERSION)' \
	-X 'github.com/bornholm/corpus/internal/build.LongVersion=$(GIT_LONG_VERSION)'

GCFLAGS ?= -trimpath=$(PWD)
ASMFLAGS ?= -trimpath=$(PWD) \

CI_EVENT ?= push

watch: tools/modd/bin/modd
	tools/modd/bin/modd

run-with-env: .env
	( set -o allexport && source .env && set +o allexport && $(value CMD))

build:
	CGO_ENABLED=0 \
		go build \
			-ldflags "$(LDFLAGS)" \
			-gcflags "$(GCFLAGS)" \
			-asmflags "$(ASMFLAGS)" \
			-o ./bin/corpus ./cmd/corpus


purge:
	rm -rf data.sqlite bleve.index

tools/modd/bin/modd:
	mkdir -p tools/modd/bin
	GOBIN=$(PWD)/tools/modd/bin go install github.com/cortesi/modd/cmd/modd@latest

tools/act/bin/act:
	mkdir -p tools/act/bin
	cd tools/act && curl https://raw.githubusercontent.com/nektos/act/master/install.sh | bash -

ci: tools/act/bin/act
	tools/act/bin/act $(CI_EVENT)

.env:
	cp .env.dist .env

include misc/*/*.mk

