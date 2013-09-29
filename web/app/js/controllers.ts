/// <reference path="reference.ts" />

module App {
	'use strict';

	export interface IMainScope extends ng.IScope {
		self: ISelf;
		private: IPrivateProfile;
		public: IPublicProfile;
		savePublic: Function;
		onFileSelect: Function;
		pictureUrl: string;
	}

	export class MainCtrl {
		publicCid: string;
		pictureUrl: string = '/api/collections/:cid/data/picture';

		public injection(): any[] {
			return [
				'$log',
				'$scope',
				'$http',
				'SelfResource',
				'PrivateResource',
				'ProfileResource',
				'CollectionResource',
				MainCtrl,
			]
		}

		constructor(
			private $log: ng.ILogService,
			private $scope: IMainScope,
			private $http: IHttpService,
			private Self: IResourceClass,
			private Private: IResourceClass,
			private Profile: IResourceClass,
			private Collection: IResourceClass
		) {
			$scope.self = <ISelf> Self.get();
			$scope.private = <IPrivateProfile> Private.get(() => {
				this.publicCid = $scope.private.publicCid;
				if (this.publicCid) {
					this.loadPublic();
				} else {
					this.newPrivate();
				}
			}, (data, status, headers, config) => {
				this.newPrivate();
			});

			$scope.savePublic = () => this.savePublic();
			$scope.onFileSelect = ($files) => this.onFileSelect($files);
		}

		newPrivate() {
			var collection = <ICollection> new this.Collection();
			collection.$save(() => {
				this.$scope.private.publicCid = this.publicCid = collection.id;
				this.$scope.private.$save();
				this.loadPublic();
			});
		}

		loadPublic() {
			this.updatePicture();
			this.$scope.public = <IPublicProfile> this.Profile.get({cid: this.publicCid});
		}

		savePublic() {
			this.$scope.public.$save({cid: this.publicCid});
		}

		onFileSelect($files) {
			angular.forEach($files, (file) => {
				this.$http.uploadFile({
					url: this.$scope.pictureUrl,
					file: file
				}).then((data, status, headers, config) => {
					this.updatePicture();
				});
			});
		}

		updatePicture() {
			this.$scope.pictureUrl = this.pictureUrl.replace(':cid', this.publicCid) + '#' + new Date().getTime();
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

	export interface IFriendListScope extends ng.IScope {
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
				'$http',
				'SelfResource',
				'FriendResource',
				FriendListCtrl
			]
		}

		constructor(
			private $scope: IFriendListScope, 
			private $http: IHttpService,
			private Self: IResourceClass,
			private Friend: IResourceClass
		) {
			this.load();
			$scope.onAddFriend = () => this.onAddFriend();
		}

		load() {
			this.$scope.friends = this.Friend.query();
		}

		onAddFriend() {
			var error = (data, status?, headers?, config?) => {
				this.$scope.recvBlobStatus = 'has-error';
				this.$scope.recvBlobError = data;
			}

			this.$http.post('/api/friends', this.$scope.recvBlob).success(() => {
				this.load();
				this.$scope.recvBlob = "";
			}).error(error);
		}
	}

	export interface IFriendDetailScope extends ng.IScope {
		fp: string;
		friend: ng.resource.IResource;
	}

	export class FriendDetailCtrl {
		public injection(): any[] {
			return [
				'$scope',
				'$routeParams',
				'FriendResource',
				FriendDetailCtrl
			]
		}

		constructor(
			$scope: IFriendDetailScope, 
			$routeParams: any, 
			Friend: IResourceClass) {
			$scope.fp = $routeParams.fp;
			$scope.friend = Friend.get({fp: $scope.fp});
		}
	}
}
