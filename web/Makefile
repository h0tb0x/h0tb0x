NPM_BIN=node_modules/.bin

.PHONY: all clean deps unit-test e2e-test build watch test

all: deps build

clean:
	rm -rf node_modules
	rm -rf app/lib
	rm -rf test/lib
	rm -rf d.ts
	rm -f app/app.js*

deps:
	npm install
	$(NPM_BIN)/bower install
	cd test && ../$(NPM_BIN)/bower install

unit-test:
	$(NPM_BIN)/karma start etc/karma.conf.js --auto-watch

e2e-test:
	$(NPM_BIN)/karma start etc/karma-e2e.conf.js

build:
	$(NPM_BIN)/grunt 

watch:
	$(NPM_BIN)/grunt watch

test:
	$(NPM_BIN)/karma start etc/karma.conf.js --single-run
	# TODO: launch h0tb0x and then run this
#	$(NPM_BIN)/karma start etc/karma-e2e.conf.js
