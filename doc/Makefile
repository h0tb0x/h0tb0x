NPM_BIN=node_modules/.bin
OS := $(shell uname)


ifeq ($(OS),Darwin)
	SUDO=
	PIP_INSTALL=brew install python
else
	SUDO=sudo
	PIP_INSTALL=sudo apt-get install python-pip
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
