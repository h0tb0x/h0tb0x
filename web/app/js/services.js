'use strict';

angular.module('App.Services', [])

	.factory('BreadcrumbService', ['$rootScope', '$location',
		function($rootScope, $location) {
			var crumbs = [];
			var service = {};

			$rootScope.$on('$routeChangeSuccess', function(event, current) {
				var parts = $location.path().split('/'), result = [], i;
				var breadcrumbPath = function(index) {
					return '/' + (parts.slice(0, index + 1)).join('/');
				}
				parts.shift();
				angular.forEach(parts, function(part, i){
					result.push({name: part, path: breadcrumbPath(i)});
				});
				crumbs = result;
			});

			service.get = function() {
				return crumbs;
			}

			return service;
		}
	])
