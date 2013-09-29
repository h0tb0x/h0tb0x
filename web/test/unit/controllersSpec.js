'use strict';

describe('H0tb0x controllers', function() {

	beforeEach(function(){
		this.addMatchers({
			toEqualData: function(expected) {
				return angular.equals(this.actual, expected);
			}
		});
	});

	beforeEach(module('App'));

	describe('FriendListCtrl', function() {
		var scope, ctrl, $httpBackend;
		var friends = [
			{ id: '1' },
			{ id: '2' }
		];
		var self = {};

		beforeEach(inject(function(_$httpBackend_, $rootScope, $controller) {
			$httpBackend = _$httpBackend_;
			$httpBackend.expectGET('/api/self').respond(self);
			$httpBackend.expectGET('/api/friends').respond(friends);
			scope = $rootScope.$new();
			ctrl = $controller('FriendListCtrl', {$scope: scope});
		}));

		it('should have 2 items from xhr', function() {
			expect(scope.friends).toEqual([]);
			$httpBackend.flush();

			expect(scope.friends).toEqualData(friends);
		});
	});

	describe('CollectionListCtrl', function() {
		var scope, ctrl, $httpBackend;
		var self = {};
		var data = [
			{ id: '1' },
			{ id: '2' }
		];

		beforeEach(inject(function(_$httpBackend_, $rootScope, $controller) {
			$httpBackend = _$httpBackend_;
			$httpBackend.expectGET('/api/self').respond(self);
			$httpBackend.expectGET('/api/collections').respond(data);
			scope = $rootScope.$new();
			ctrl = $controller('CollectionListCtrl', {$scope: scope});
		}));

		it('should have 2 items from xhr', function() {
			expect(scope.data).toEqual([]);
			$httpBackend.flush();

			expect(scope.data).toEqualData(data);
		});
	});

});
