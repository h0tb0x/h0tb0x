export GOPATH=${PWD}
OS := $(shell uname)

ifeq ($(OS),Darwin)
	export CC=gcc-4.2
endif

.PHONY: deps go test web clean clean_go clean_web

all: go web

deps:
	go get github.com/jteeuwen/go-bindata
	go install github.com/jteeuwen/go-bindata
	go get h0tb0x

go: deps
	bin/go-bindata -pkg db -func schema_sql -out src/h0tb0x/db/schema_sql.go src/h0tb0x/db/schema.sql
	go fmt h0tb0x/...
	go install h0tb0x

test: build
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

clean: clean_go clean_web

clean_go:
	rm -rf bin
	rm -rf pkg
	rm -rf src/code.google.com
	rm -rf src/github.com

clean_web:
	make -C web clean
