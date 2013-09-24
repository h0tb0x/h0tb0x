module.exports = function (config) {
	config.set({
		basePath: '../',

		files: [
			'app/lib/angular/angular.js',
			'app/lib/angular-resource/angular-resource.js',
			'test/lib/angular-mocks/angular-mocks.js',
			'app/js/**/*.js',
			'test/unit/**/*.js'
		],

		frameworks: ['jasmine'],

		autoWatch: true,

		browsers: ['Chrome'],

		junitReporter: {
			outputFile: 'test_out/unit.xml',
			suite: 'unit'
		}
	});
};
