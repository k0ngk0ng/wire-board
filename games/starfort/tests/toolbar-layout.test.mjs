// Before implementation: controls wrap, primary actions disappear, overflow
// consumes map space, narrow HUD values collide, or desktop controls collapse.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { toolbarLayout } from '../web/toolbar-layout.ts';
test('ordinary combat keeps all three controls on one line', () => {
 assert.deepEqual(toolbarLayout(280,170,['build','tasks','info']),{inline:['build','tasks','info'],overflow:[]});
});
test('preparation keeps ready and build while secondary actions use more', () => {
 assert.deepEqual(toolbarLayout(280,170,['ready','pause','build','tasks','info']),{inline:['ready','build'],overflow:['pause','tasks','info']});
});
test('large numbers preserve a usable primary action and accessible overflow', () => {
 assert.deepEqual(toolbarLayout(280,206,['ready','pause','build','tasks','info']),{inline:['ready'],overflow:['pause','build','tasks','info']});
 assert.deepEqual(toolbarLayout(280,255,['resume','build','tasks','info']),{inline:[],overflow:['resume','build','tasks','info']});
});
test('wide views do not hide any operation and input lists remain unchanged', () => {
 const items=['ready','pause','build','medical','shop','tasks','info'];
 assert.deepEqual(toolbarLayout(1300,300,items),{inline:items,overflow:[]});
 assert.equal(items.length,7);
});
