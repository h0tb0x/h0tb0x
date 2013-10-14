module.exports = function(grunt) {
	grunt.loadNpmTasks('grunt-exec');
	grunt.loadNpmTasks('grunt-express');
	grunt.loadNpmTasks('grunt-contrib-watch');

	grunt.initConfig({
		exec: {
			sphinx: {
				command: 'sphinx-build -b html -d _build/doctrees src _build/html'
			},
		},
		express: {
			server: {
				options: {
					port: 9000,
					hostname: '0.0.0.0',
					bases: ['_build/html'],
					livereload: true
				}
			}
		},
		watch: {
			sphinx: {
				files: ['src/**/*'],
				tasks: ['exec']
			},
			options: {
				livereload: true
			}
		}
	});

	grunt.registerTask('default', [
		'exec',
		'express',
		'watch'
	]);

	grunt.registerTask('build', [
		'exec'
	]);
}
