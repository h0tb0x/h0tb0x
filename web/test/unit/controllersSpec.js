'use strict';

describe('App', function() {
	var mockAppService;
	var httpBackend;
	var rootScope;

	beforeEach(function() {
		this.addMatchers({
			toEqualData: function(expected) {
				return angular.equals(this.actual, expected);
			}
		});
	});

	beforeEach(function() {
		module('App', function($provide) {
			mockAppService = {
				load: function() {},
				resolveProfile: function() {}
			}
			spyOn(mockAppService, 'load');
			$provide.value('AppService', mockAppService);
		});

		inject(function($httpBackend, $rootScope) {
			httpBackend = $httpBackend;
			rootScope = $rootScope;
			rootScope.publicCid = 'PUB_CID';
		});
	});

	afterEach(function() {
		httpBackend.verifyNoOutstandingExpectation();
		httpBackend.verifyNoOutstandingRequest();
	});

	it('should load self at startup', function() {
		expect(mockAppService.load).toHaveBeenCalled();
	});

	describe('controllers', function() {

		describe('FriendListCtrl', function() {
			var scope, ctrl;
			var friends = [
				{ id: '1', recvCid: 'R1', sendCid: 'S1' },
				{ id: '2', recvCid: 'R2', sendCid: 'S2' }
			];

			var profiles = {
				R1: { publicCid: 'PUB_CID_1' },
				R2: { publicCid: 'PUB_CID_2' }
			}

			beforeEach(inject(function($rootScope, $controller) {
				scope = $rootScope.$new();
				httpBackend.whenGET('/api/friends').respond(friends);
				angular.forEach(profiles, function(value, key) {
					httpBackend.whenGET('/api/collections/'+key+'/data/profile').respond(value);
				});
				httpBackend.expectGET('/api/friends');
				ctrl = $controller('FriendListCtrl', {$scope: scope});
				expect(scope.friends).toEqual([]);
				httpBackend.flush();
			}));

			it('should have 2 items from xhr', function() {
				expect(scope.friends).toEqualData(friends);
			});

			it('should handle adding new friends', function() {
				var newFriend = {
					id: 'NEW_FP',
					recvCid: 'RECV_CID',
					sendCid: 'SEND_CID'
				};
				scope.recvBlob = 'RECV_BLOB';

				// send passport to initiate friending process
				httpBackend.expectPOST('/api/friends', {
					passport: scope.recvBlob
				}).respond(newFriend);
				friends.push(newFriend);

				// page refresh: reload friends list
				httpBackend.expectGET('/api/friends').respond(friends);

				// begin invite to friend for profile collection
				httpBackend.expectPOST('/api/invites', {
					cid: rootScope.publicCid, 
					friend: newFriend.id
				}).respond(200);

				// put a profile reference into the friend's outbox
				httpBackend.expectPUT('/api/collections/'+newFriend.sendCid+'/data/profile', {
					publicCid: rootScope.publicCid
				}).respond(200);

				ctrl.onAddFriend();
				httpBackend.flush();
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
