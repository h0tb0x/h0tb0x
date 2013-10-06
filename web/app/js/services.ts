/// <reference path="reference.ts" />

module App {
	'use strict';

	export class AppService {

		private _profiles: { [fp: string]: IFriend; } = {};
		private _pending = {};

		public injection(): any[] {
			return [
				'$log',
				'$rootScope',
				'SelfResource',
				'ProfileResource',
				'CollectionResource',
				'CollectionInviteResource',
				AppService
			]
		}

		constructor(
			private $log: ng.ILogService,
			private $rootScope: IRootScope,
			private Self: IResourceClass,
			private Profile: IResourceClass,
			private Collection: IResourceClass,
			private Invite: IResourceClass
		) {
		}

		public load() {
			this.Self.get((self: ISelf) => {
				this.$rootScope.self = self;
				this.getPublicCid();
			});
		}

		private getPublicCid() {
			this.Profile.get({cid: this.$rootScope.self.selfCid}, (data: IProfileRef) => {
				this.savePublicCid(data.publicCid);
			}, (data, status, headers, config) => {
				this.newPrivate();
			});
		}

		private newPrivate() {
			var collection = <ICollection> new this.Collection();
			collection.$save(() => {
				var data = <IProfileRef> new this.Profile();
				data.publicCid = collection.id;
				data.$save({cid: this.$rootScope.self.selfCid});
				this.savePublicCid(data.publicCid);
			});
		}

		private savePublicCid(cid: string) {
			this.$rootScope.publicCid = cid;
		}

		public resolveProfile(friend: IFriend) {
			if (friend.id in this._profiles) {
				var profile = this._profiles[friend.id];
				friend.name = profile.name;
				friend.pictureUrl = profile.pictureUrl;
				return;
			}

			this.Profile.get({
				cid: friend.recvCid
			}, (ref: IProfileRef) => { // success
				var now = new Date().getTime();
				friend.pictureUrl = '/api/collections/'+ref.publicCid+'/data/picture'+'#'+now;
				var invite = <ICollectionInvite> new this.Invite();
				invite.cid = friend.publicCid = ref.publicCid;
				invite.friend = friend.id;
				invite.$save(() => {
					this.loadProfile(friend);
				});
			}, (result) => { // failure
				this.$log.info(result);
			});
		}

		private loadProfile(friend: IFriend) {
			this.Profile.get({
				cid: friend.publicCid
			}, (profile: IPublicProfile) => {
				friend.name = profile.name;
				this._profiles[friend.id] = friend;
			});
		}
	}
}
