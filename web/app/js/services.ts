/// <reference path="reference.ts" />

module App {
	'use strict';

	export class InitService {
		public injection(): any[] {
			return [
				'$log',
				'$rootScope',
				'SelfResource',
				InitService
			]
		}

		constructor(
			private $log: ng.ILogService,
			private $rootScope: IRootScope,
			private Self: IResourceClass
		) {
		}

		public load() {
			this.Self.get((self: ISelf) => {
				this.$rootScope.selfCid = self.selfCid;
			});
		}
	}
}
