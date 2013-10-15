NPM_BIN=node_modules/.bin
OS := $(shell uname)

ifeq ($(OS),Darwin)
	SUDO=
else
	SUDO=sudo
endif

.PHONY: all clean deps build watch

all: deps build

clean:
	rm -rf node_modules
	rm -rf _build

deps:
	$(SUDO) pip install Sphinx sphinxcontrib-httpdomain
	npm install

build:
	$(NPM_BIN)/grunt

watch:
	$(NPM_BIN)/grunt monitor
