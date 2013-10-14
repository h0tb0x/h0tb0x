export GOPATH=${PWD}
OS := $(shell uname)

ifeq ($(OS),Darwin)
	export CC=gcc-4.2
	SUDO=
else
	SUDO=sudo
endif

.PHONY: deps schema go test web 
.PHONY: clean clean_go clean_web 
.PHONY: docs deps_docs clean_docs

all: deps go web docs

bin/gpm:
	rm -rf /tmp/gpm
	git clone git@github.com:pote/gpm.git /tmp/gpm
	cd /tmp/gpm && ./configure --prefix=${PWD} && make install

deps: bin/gpm
	go get -d h0tb0x
	bin/gpm
	go install h0tb0x/db/embed

go: deps quick

quick:
	bin/embed src/h0tb0x/db/h0tb0x
	bin/embed src/h0tb0x/db/rendezvous
	go fmt h0tb0x/...
	go install h0tb0x

test: go
	go test h0tb0x/db
	go test h0tb0x/transfer
	go test h0tb0x/crypto
	go test h0tb0x/link
	go test h0tb0x/sync
	go test h0tb0x/meta
	go test h0tb0x/data
	go test h0tb0x/api
	go test h0tb0x/rendezvous
	make -C web test

test_web:
	make -C web test

web:
	make -C web

clean: clean_go clean_web clean_docs

clean_go:
	rm -rf bin
	rm -rf pkg

clean_web:
	make -C web clean

clean_docs:
	make -C doc clean

deps_docs:
	$(SUDO) pip install Sphinx sphinxcontrib-httpdomain

docs: deps_docs
	cd doc && grunt build
