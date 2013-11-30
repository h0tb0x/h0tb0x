/// <reference path="reference.ts" />

module App {
	'use strict';

	export interface IMainScope extends IRootScope {
		profile: IPublicProfile;
		saveProfile: Function;
		onFileSelect: Function;
		pictureUrl: string;
	}

	export class MainCtrl {
		public injection(): any[] {
			return [
				'$log',
				'$scope',
				'$upload',
				'$http',
				'SelfResource',
				'ProfileResource',
				MainCtrl,
			]
		}

		constructor(
			private $log: ng.ILogService,
			private $scope: IMainScope,
			private $upload: any,
			private $http: IHttpService,
			private Self: IResourceClass,
			private Profile: IResourceClass
		) {
			$scope.self = <ISelf> Self.get();
			$scope.saveProfile = () => this.saveProfile();
			$scope.onFileSelect = ($files) => this.onFileSelect($files);
			$scope.$watch('publicCid', () => {
				if ($scope.publicCid) {
					$scope.profile = <IPublicProfile> Profile.get({cid: $scope.publicCid});
					this.updatePicture();
				}
			});
		}

		saveProfile() {
			this.$scope.profile.$save({cid: this.$scope.publicCid});
		}

		onFileSelect($files) {
			angular.forEach($files, (file) => {
				this.$upload.upload({
					url: this.$scope.pictureUrl,
					file: file
				}).then((data, status, headers, config) => {
					this.updatePicture();
				});
			});
		}

		updatePicture() {
			var now = new Date().getTime();
			this.$scope.pictureUrl = '/api/collections/'+this.$scope.publicCid+'/data/picture'+'#'+now;
		}
	}

	export interface ICollectionListScope extends ng.IScope {
		collections: ng.resource.IResource;
	}

	export class CollectionListCtrl {
		public injection(): any[] {
			return [ 
				'$log', 
				'$scope', 
				'CollectionResource', 
				CollectionListCtrl 
			]
		}

		constructor(
			private $log: ng.ILogService,
			private $scope: ICollectionListScope,
			private Collection: IResourceClass
		) {
			$scope.collections = Collection.query();
		}
	}

	export interface ICollectionDetailScope extends ng.IScope {
		cid: string;
		collection: ng.resource.IResource;
		writers: ng.resource.IResource;
		data: ng.resource.IResource;
	}

	export class CollectionDetailCtrl {
		public injection(): any[] {
			return [
				'$scope',
				'$routeParams',
				'CollectionResource', 
				'CollectionDataResource', 
				'CollectionWriterResource', 
				CollectionDetailCtrl
			]
		}

		constructor(
			$scope: ICollectionDetailScope, 
			$routeParams: any,
			Collection: IResourceClass, 
			CollectionData: IResourceClass, 
			CollectionWriter: IResourceClass
		) {
			$scope.cid = $routeParams.cid;
			$scope.collection = Collection.get({cid: $scope.cid});
			$scope.writers = CollectionWriter.query({cid: $scope.cid});
			$scope.data = CollectionData.query({cid: $scope.cid});
		}
	}

	export interface IFriendListScope extends IRootScope {
		friends: ng.resource.IResource;
		recvBlob: string;
		recvBlobStatus: string;
		recvBlobError: string;
		onAddFriend: Function;
	}

	export class FriendListCtrl {
		public injection(): any[] {
			return [
				'$scope',
				'AppService',
				'SelfResource',
				'FriendResource',
				'CollectionDataResource',
				'CollectionInviteResource',
				'ProfileResource',
				FriendListCtrl
			]
		}

		constructor(
			private $scope: IFriendListScope, 
			private app: AppService, 
			private Self: IResourceClass,
			private Friend: IResourceClass,
			private CollectionData: IResourceClass,
			private Invite: IResourceClass,
			private Profile: IResourceClass
		) {
			this.load();
			$scope.onAddFriend = () => this.onAddFriend();
		}

		load() {
			this.$scope.friends = this.Friend.query((friends: IFriend[]) => {
				angular.forEach(friends, (friend: IFriend) => {
					this.app.resolveProfile(friend);
				});
			});
		}

		onAddFriend() {
			var friend = <IFriend> new this.Friend();
			friend.passport = this.$scope.recvBlob;
			friend.$save((friend: IFriend) => {
				this.load();
				this.$scope.recvBlob = "";
				this.shareCollection(friend);
			}, (result) => {
				this.$scope.recvBlobStatus = 'has-error';
				this.$scope.recvBlobError = result.data;
			});
		}
		
		shareCollection(friend: IFriend) {
			console.log(friend);
			var invite = <ICollectionInvite> new this.Invite();
			invite.cid = this.$scope.publicCid;
			invite.friend = friend.id;
			invite.$save();

			var sendRef = <IProfileRef> new this.Profile();
			sendRef.publicCid = this.$scope.publicCid;
			sendRef.$save({cid: friend.sendCid});
		}
	}

	export interface IFriendDetailScope extends ng.IScope {
		fp: string;
		friend: ng.resource.IResource;
		onDeleteFriend: Function;
	}

	export class FriendDetailCtrl {
		public injection(): any[] {
			return [
				'$scope',
				'$routeParams',
				'AppService',
				'FriendResource',
				FriendDetailCtrl
			]
		}

		constructor(
			private $scope: IFriendDetailScope, 
			private $routeParams: any, 
			private app: AppService, 
			private Friend: IResourceClass) {
			$scope.fp = $routeParams.fp;
			$scope.friend = Friend.get({fp: $scope.fp}, (friend: IFriend) => {
				this.app.resolveProfile(friend);
			});
			$scope.onDeleteFriend = () => this.onDeleteFriend();
		}
		
		onDeleteFriend() {
			var friend = <IFriend> this.$scope.friend;
			var fname = friend.name;
			if (!friend.name) {
				fname = "Anonymous (" + this.$scope.fp + ")";
			}
			var ready = confirm("Are you sure you want to delete this friend " + fname + " permanently?");
			if(!ready) {
				return;
			}
			friend.$delete({fp: this.$scope.fp}, (friend: IFriend) => {
				// Unknown
				alert("Deleted");
				window.location.href = "#/friends";
			}, (result) => {
				alert("Error: " + result.data);
			});
		}
	}
}
