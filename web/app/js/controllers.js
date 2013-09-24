'use strict';

angular.module('App.Controllers', ['App.Resources', 'App.Services'])
	
	.controller('BreadcrumbCtrl', ['$rootScope', '$scope', 'BreadcrumbService',
		function($rootScope, $scope, BreadcrumbService) {
			$scope.crumbs = BreadcrumbService;
		}
	])

	.controller('MainCtrl', ['$rootScope', '$scope', 'Self', 'SelfInfo',
		function($rootScope, $scope, Self, SelfInfo) {
			$rootScope.self = Self.get();
			$scope.info = SelfInfo.get();
		}
	])

	.controller('FriendListCtrl', ['$scope', 'Friend', 'FriendInfo', '$http',
		function($scope, Friend, FriendInfo, $http) {
			$scope.load = function() {
				$scope.friends = Friend.query(function(result) {
					angular.forEach(result, function(friend, key) {
						FriendInfo(friend.id).get(function(info) {
							friend.name = info.name;
						})
					})
				})
			}

			$scope.load();
			$http.get('/api/self').success(function(data, status, headers, config) {
				$scope.sendBlob = data;
			});

			$scope.onAddFriend = function() {
				var error = function(data, status, headers, config) {
					$scope.recvBlobStatus = 'has-error';
					$scope.recvBlobError = data;
				}

				try {
					var json = angular.fromJson($scope.recvBlob);
				}
				catch (e) {
					error(e);
					return;
				}

				$http.post('/api/friends', json).success(function() {
					$scope.load();
					$scope.recvBlob = "";
				}).error(error);
			}
		}
	])

	.controller('FriendDetailCtrl', ['$scope', '$routeParams', 'Friend', 'FriendInfo',
		function($scope, $routeParams, Friend, FriendInfo) {
			$scope.id = $routeParams.id;
			$scope.data = Friend.get({id: $scope.id});
			$scope.info = FriendInfo($scope.id).get();
		}
	])

	.controller('CollectionListCtrl', ['$scope', 'Collection', 'CollectionInfo',
		function($scope, Collection, CollectionInfo) {
			$scope.collections = Collection.query(function(result) {
				angular.forEach(result, function(collection, key) {
					CollectionInfo(collection.id).get(function(info) {
						collection.name = info.name;
					})
				})
			})
		}
	])

	.controller('CollectionDetailCtrl', 
		['$scope', '$routeParams', 'Collection', 'CollectionData', 'CollectionWriter', 'CollectionInfo',
		function($scope, $routeParams, Collection, CollectionData, CollectionWriter, CollectionInfo) {
			$scope.id = $routeParams.id;
			$scope.collection = Collection.get({cid: $scope.id});
			$scope.writers = CollectionWriter.query({cid: $scope.id});
			$scope.data = CollectionData.query({cid: $scope.id});
			$scope.info = CollectionInfo($scope.id).get();
		}
	])
