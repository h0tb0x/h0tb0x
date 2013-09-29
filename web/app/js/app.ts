/// <reference path="reference.ts" />

module App {
	'use strict';

	export interface IRootScope extends ng.IScope {
		self: ISelf;
	}

	export interface IHttpService extends ng.IHttpService {
		uploadFile(any);
	}

	var app = angular.module('App', ['ngResource', 'angularFileUpload'])

		// configuration
		.config(['$routeProvider', function($routeProvider: ng.IRouteProvider) {
			$routeProvider
				.when('/', {
					templateUrl: 'html/main.html',
					controller: 'MainCtrl'
				})
				.when('/friends', {
					templateUrl: 'html/friend-list.html',
					controller: 'FriendListCtrl'
				})
				.when('/friends/:fp', {
					templateUrl: 'html/friend-detail.html',
					controller: 'FriendDetailCtrl'
				})
				.when('/collections', {
					templateUrl: 'html/collection-list.html',
					controller: 'CollectionListCtrl'
				})
				.when('/collections/:cid', {
					templateUrl: 'html/collection-detail.html',
					controller: 'CollectionDetailCtrl'
				})
				.otherwise({ redirectTo: '/' })
		}])

		// controllers
		.controller('MainCtrl', MainCtrl.prototype.injection())
		.controller('CollectionListCtrl', CollectionListCtrl.prototype.injection())
		.controller('CollectionDetailCtrl', CollectionDetailCtrl.prototype.injection())
		.controller('FriendListCtrl', FriendListCtrl.prototype.injection())
		.controller('FriendDetailCtrl', FriendDetailCtrl.prototype.injection())

		// resources
		.factory('SelfResource', SelfResource())
		.factory('PrivateResource', PrivateResource())
		.factory('ProfileResource', ProfileResource())
		.factory('CollectionResource', CollectionResource())
		.factory('CollectionWriterResource', CollectionWriterResource())
		.factory('CollectionDataResource', CollectionDataResource())
		.factory('FriendResource', FriendResource())

		// initialization
		.run(['$log', '$rootScope', 'SelfResource',
			function(
				$log: ng.ILogService, 
				$rootScope: IRootScope, 
				Self: ng.resource.IResourceClass
			) {
				$rootScope.self = <ISelf> Self.get();
			}
		])
}
