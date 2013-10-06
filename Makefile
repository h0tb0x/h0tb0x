export GOPATH=${PWD}
OS := $(shell uname)

ifeq ($(OS),Darwin)
	export CC=gcc-4.2
endif

.PHONY: deps go test web clean clean_go clean_web 

all: deps go web

bin/gpm:
	rm -rf /tmp/gpm
	git clone git@github.com:pote/gpm.git /tmp/gpm
	cd /tmp/gpm && ./configure --prefix=${PWD} && make install

deps: bin/gpm
	go get github.com/jteeuwen/go-bindata
	go get h0tb0x
	bin/gpm
	go install github.com/jteeuwen/go-bindata

go: 
	bin/go-bindata -pkg db -func schema_sql -out src/h0tb0x/db/schema_sql.go src/h0tb0x/db/schema.sql
	go fmt h0tb0x/...
	go install h0tb0x

test: go
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

clean_web:
	make -C web clean
