'use strict';

angular.module('App.Resources', ['ngResource'])
	.factory('Self', ['$resource', 
		function($resource) {
			return $resource('/api/self');
		}
	])

	.factory('SelfInfo', ['$rootScope', '$resource', 'CollectionInfo',
		function($rootScope, $resource, CollectionInfo) {
			return CollectionInfo($rootScope.self.selfCid);
		}
	])

	.factory('Friend', ['$resource', 
		function($resource) {
			return $resource('/api/friends/:id');
		}
	])

	.factory('FriendInfo', ['$rootScope', '$resource', 
		function($rootScope, $resource) {
			return function(fp) {
				return $resource('/api/collections/:cid/data/.friends/:fp', {
					cid: $rootScope.self.selfCid,
					fp: fp
				}, {
					save: { method: 'PUT' }
				})
			}
		}
	])

	.factory('Collection', ['$resource', 
		function($resource) {
			return $resource('/api/collections/:cid');
		}
	])

	.factory('CollectionData', ['$resource', 
		function($resource) {
			return $resource('/api/collections/:cid/data');
		}
	])

	.factory('CollectionInfo', ['$resource', 
		function($resource) {
			return function(cid) {
				return $resource('/api/collections/:cid/data/.info', {
					cid: cid
				}, {
					save: { method: 'PUT' }
				})
			}
		}
	])

	.factory('CollectionWriter', ['$resource', 
		function($resource) {
			return $resource('/api/collections/:cid/writers/:wid');
		}
	])
