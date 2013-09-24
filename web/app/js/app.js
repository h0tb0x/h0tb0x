'use strict';

angular.module('App', ['App.Controllers', 'App.Resources'])
	.config(['$routeProvider', function($routeProvider) {
		$routeProvider
			.when('/', {
				templateUrl: 'html/main.html',
				controller: 'MainCtrl'
			})
			.when('/friends', {
				templateUrl: 'html/friend-list.html',
				controller: 'FriendListCtrl'
			})
			.when('/friends/:id', {
				templateUrl: 'html/friend-detail.html',
				controller: 'FriendDetailCtrl'
			})
			.when('/collections', {
				templateUrl: 'html/collection-list.html',
				controller: 'CollectionListCtrl'
			})
			.when('/collections/:id', {
				templateUrl: 'html/collection-detail.html',
				controller: 'CollectionDetailCtrl'
			})
			.otherwise({ redirectTo: '/' })
	}])

	.run(['$rootScope', 'Self', function($rootScope, Self) {
		$rootScope.self = Self.get();
	}])
