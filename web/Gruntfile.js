module.exports = function(grunt) {
	grunt.loadNpmTasks('grunt-ts');

	grunt.initConfig({
		pkg: grunt.file.readJSON('package.json'),

		ts: {
			app: {
				src: ['app/js/**/*.ts'],
				reference: 'app/js/reference.ts',
				out: 'app/app.js',
				watch: 'app/js'
			}
		}
	});

	grunt.registerTask('default', ['tsd-pkg', 'ts']);

	grunt.registerTask('tsd-pkg', 'Install TypeScript definitions specified in package.json', function() {
		this.requiresConfig('pkg');
		var pkg = grunt.config('pkg');
		pkg.tsd.forEach(function(tsd) {
			grunt.task.run('tsd:' + tsd);
		})
	});

	grunt.registerTask('tsd', 'Install a TypeScript definition', function(tsd) {
		var done = this.async();
		grunt.util.spawn({cmd: 'node_modules/.bin/tsd', args: [ 'install*', tsd ]}, function(error, result, code) {
			grunt.log.writeln(result);
			done(error);
		});
	})
};
