/// <reference path="reference.ts" />

module App {
	'use strict';

	export interface ISelf extends ng.resource.IResource {
		id: string;
		host: string;
		port: number;
		selfCid: string;
		passport: string;
		rendezvous: string;
		publicKey: string;
	}

	export interface IProfileRef extends ng.resource.IResource {
		publicCid: string;
	}

	export interface IPublicProfile extends ng.resource.IResource {
		name: string;
	}

	export interface IFriend extends ISelf {
		name: string;
		publicCid: string;
		pictureUrl: string;
		recvCid: string;
		sendCid: string;
	}

	export interface ICollection extends ng.resource.IResource {
		id: string;
		owner: string;
	}

	export interface ICollectionWriter extends ng.resource.IResource {
		id: string;
		pubkey: string;
	}

	export interface ICollectionInvite extends ng.resource.IResource {
		cid: string;
		friend: string;
		remove: boolean;
	}

	export interface IForum extends ng.resource.IResource {
	}

	export interface IForumPost extends ng.resource.IResource {
		uuid: string;
		fp: string;
		date: Date;
		body: string;
	}
}
