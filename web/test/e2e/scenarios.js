'use strict';

describe('H0tb0x App', function() {

	it('should redirect index.html to #/', function() {
		browser().navigateTo('/index.html');
		expect(browser().location().url()).toBe('/');
	});
 
	// describe('Friend list view', function() {

	// 	beforeEach(function() {
	// 		browser().navigateTo('/#friends');
	// 	});

	// 	it('should list all friends', function() {
	// 		expect(repeater('.friend li').count()).toBeGreaterThan(0);
	// 	});

	// });

	describe('Collection list view', function() {

		beforeEach(function() {
			browser().navigateTo('/#/collections');
		});


		it('should list all collections', function() {
			expect(repeater('table tr').count()).toBeGreaterThan(0);
		});

	});

});
