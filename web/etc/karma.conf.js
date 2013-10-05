module.exports = function (config) {
	config.set({
		basePath: '../',

		files: [
			'app/lib/angular/angular.js',
			'app/lib/angular-resource/angular-resource.js',
			'app/lib/angularjs-file-upload/angular-file-upload.js',
			'test/lib/angular-mocks/angular-mocks.js',
			'app/app.js',
			'test/unit/**/*.js'
		],

		frameworks: ['jasmine'],

		browsers: ['PhantomJS'],

		junitReporter: {
			outputFile: 'test_out/unit.xml',
			suite: 'unit'
		}
	});
};
