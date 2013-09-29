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

	export function PrivateResource(): any[] {
		return [ '$rootScope', '$resource',
			function(
				$rootScope: IRootScope, 
				$resource: ng.resource.IResourceService
			): ng.resource.IResourceClass {
				return $resource('/api/collections/:sid/data/profile', {
					sid: $rootScope.selfCid
				}, {
					save: { method: 'PUT' }
				})
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
