all: install

install:
	npm install
	rm -rf app/lib
	node_modules/.bin/bower install
	rm -rf test/lib
	cd test && ../node_modules/.bin/bower install

unit-test:
	node_modules/.bin/karma start etc/karma.conf.js

e2e-test:
	node_modules/.bin/karma start etc/karma-e2e.conf.js

watch:
	node_modules/.bin/grunt
