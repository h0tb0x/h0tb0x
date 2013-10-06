/// <reference path="reference.ts" />

module App {
	'use strict';

	export interface IRootScope extends ng.IScope {
		self: ISelf;
		publicCid: string;
	}

	export interface IHttpService extends ng.IHttpService {
		uploadFile(any);
	}

	var app = angular.module('App', ['ngResource', 'angularFileUpload', 'ui.bootstrap'])

		// configuration
		.config(['$routeProvider',
			function(
				$routeProvider: ng.IRouteProvider
			) {
			$routeProvider
				.when('/', {
					templateUrl: 'html/main.html',
					controller: 'MainCtrl'
				})
				.when('/wall', {
					templateUrl: 'html/wall.html',
					controller: 'WallCtrl'
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
		.controller('WallCtrl', WallCtrl.prototype.injection())
		.controller('CollectionListCtrl', CollectionListCtrl.prototype.injection())
		.controller('CollectionDetailCtrl', CollectionDetailCtrl.prototype.injection())
		.controller('FriendListCtrl', FriendListCtrl.prototype.injection())
		.controller('FriendDetailCtrl', FriendDetailCtrl.prototype.injection())

		// services
		.service('AppService', AppService.prototype.injection())

		// resources
		.factory('SelfResource', SelfResource())
		.factory('WallResource', WallResource())
		.factory('ProfileResource', ProfileResource())
		.factory('CollectionResource', CollectionResource())
		.factory('CollectionWriterResource', CollectionWriterResource())
		.factory('CollectionDataResource', CollectionDataResource())
		.factory('CollectionInviteResource', CollectionInviteResource())
		.factory('FriendResource', FriendResource())

		// initialization
		.run(['$log', 'AppService',
			function(
				$log: ng.ILogService, 
				app: AppService
			) {
				app.load();
			}
		])
}
