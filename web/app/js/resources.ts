/// <reference path="reference.ts" />

module App {
	'use strict';

	export interface IResourceClass extends ng.resource.IResourceClass {
		new (): ng.resource.IResource;
	}

	export function SelfResource(): any[] {
		return [ '$resource',
			function(
				$resource: ng.resource.IResourceService
			): ng.resource.IResourceClass {
				return $resource('/api/self');	
			}
		]
	}

	export function ProfileResource(): any[] {
		return [ '$resource',
			function(
				$resource: ng.resource.IResourceService
			): ng.resource.IResourceClass {
				return $resource('/api/collections/:cid/data/profile', {
				}, {
					save: { method: 'PUT' }
				})
			}
		]
	}

	export function CollectionResource(): any[] {
		return [ '$resource',
			function(
				$resource: ng.resource.IResourceService
			): ng.resource.IResourceClass {
				return $resource('/api/collections/:cid');
			}
		]
	}

	export function CollectionWriterResource(): any[] {
		return [ '$resource', 
			function(
				$resource: ng.resource.IResourceService
			): ng.resource.IResourceClass {
				return $resource('/api/collections/:cid/writers/:wid');
			}
		]
	}

	export function CollectionDataResource(): any[] {
		return [ '$resource', 
			function(
				$resource: ng.resource.IResourceService
			): ng.resource.IResourceClass {
				return $resource('/api/collections/:cid/data/:path');
			}
		]
	}

	export function CollectionInviteResource(): any[] {
		return [ '$resource',
			function(
				$resource: ng.resource.IResourceService
			): ng.resource.IResourceClass {
				return $resource('/api/invites');
			}
		]
	}

	export function FriendResource(): any[] {
		return [ '$resource',
			function(
				$resource: ng.resource.IResourceService
			): ng.resource.IResourceClass {
				return $resource('/api/friends/:fp');
			}
		]
	}
}
