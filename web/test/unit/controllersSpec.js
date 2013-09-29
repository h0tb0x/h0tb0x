'use strict';

describe('App', function() {
	var mockInitService;
	var httpBackend;

	beforeEach(function() {
		this.addMatchers({
			toEqualData: function(expected) {
				return angular.equals(this.actual, expected);
			}
		});
	});

	beforeEach(function() {
		module('App', function($provide) {
			mockInitService = {
				load: function() {
					console.log('hi');
				}
			}
			spyOn(mockInitService, 'load');
			$provide.value('InitService', mockInitService);
		});

		inject(function($httpBackend) {
			httpBackend = $httpBackend;
		});
	});

	afterEach(function() {
		httpBackend.verifyNoOutstandingExpectation();
		httpBackend.verifyNoOutstandingRequest();
	});

	it('should load self at startup', function() {
		expect(mockInitService.load).toHaveBeenCalled();
	});

	describe('controllers', function() {

		describe('FriendListCtrl', function() {
			var scope, ctrl;
			var friends = [
				{ id: '1' },
				{ id: '2' }
			];

			beforeEach(inject(function($rootScope, $controller) {
				httpBackend.expectGET('/api/friends').respond(friends);
				scope = $rootScope.$new();
				ctrl = $controller('FriendListCtrl', {$scope: scope});
			}));

			it('should have 2 items from xhr', function() {
				expect(scope.friends).toEqual([]);
				httpBackend.flush();
				expect(scope.friends).toEqualData(friends);
			});
		});

		describe('CollectionListCtrl', function() {
			var scope, ctrl;
			var collections = [
				{ id: '1' },
				{ id: '2' }
			];

			beforeEach(inject(function($rootScope, $controller) {
				httpBackend.expectGET('/api/collections').respond(collections);
				scope = $rootScope.$new();
				ctrl = $controller('CollectionListCtrl', {$scope: scope});
			}));

			it('should have 2 items from xhr', function() {
				expect(scope.collections).toEqual([]);
				httpBackend.flush();
				expect(scope.collections).toEqualData(collections);
			});
		});

	});

});
