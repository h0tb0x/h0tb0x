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

	export interface IPrivateProfile extends ng.resource.IResource {
		publicCid: string;
	}

	export interface IPublicProfile extends ng.resource.IResource {
		name: string;
		created: number;
	}

	export interface IFriend extends ISelf {
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
		id: string;
		fp: string;
		remove: boolean;
	}
}
