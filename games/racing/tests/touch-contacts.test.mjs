import test from 'node:test';
import assert from 'node:assert/strict';
import { RacingTouchContacts } from '../web/racing-touch-contacts.ts';

test('native active touches remove only the lifted finger even when pointer release was lost',()=>{
 const c=new RacingTouchContacts();c.press(0,'left');c.press(7,'throttle');
 assert.deepEqual([...c.actions].sort(),['left','throttle']);
 c.reconcile([7]);assert.deepEqual([...c.actions],['throttle']);
 c.press(3,'right');c.reconcile([3]);assert.deepEqual([...c.actions],['right']);
 c.reconcile([]);assert.equal(c.actions.size,0);
});
test('one finger may slide across arrows, out of controls, and between pedals',()=>{
 const c=new RacingTouchContacts();c.press(1,'left');c.move(1,'right');assert(c.actions.has('right'));assert(!c.actions.has('left'));
 c.move(1,null);assert.equal(c.actions.size,0);c.move(1,'left');assert(c.actions.has('left'));
 c.press(2,'throttle');c.move(2,'brake');assert(c.actions.has('brake'));assert(!c.actions.has('throttle'));
 c.move(2,'horn');assert(!c.actions.has('horn'));assert(!c.actions.has('brake'));
 c.move(1,'throttle');assert(!c.actions.has('throttle'));
});
test('identifiers do not replay old presses or acquire controls without a new start',()=>{
 const c=new RacingTouchContacts();c.press(0,'rear');c.reconcile([]);c.reconcile([0,8]);assert.equal(c.actions.size,0);
 c.move(0,'left');assert.equal(c.actions.size,0);c.press(0,'throttle');assert.deepEqual([...c.actions],['throttle']);
 c.clear();assert.equal(c.actions.size,0);c.reconcile([0]);assert.equal(c.actions.size,0);
});
test('opposite directions and independent horn/rear contacts remain separately owned',()=>{
 const c=new RacingTouchContacts();c.press(1,'left');c.press(2,'right');c.press(3,'horn');c.press(4,'rear');c.press(5,'drift');
 assert(c.actions.has('left')&&c.actions.has('right'));c.reconcile([2,3,4,5]);assert(!c.actions.has('left'));assert(c.actions.has('right'));
 c.move(3,'rear');assert(!c.actions.has('horn'));assert(c.actions.has('rear'));c.clear();assert.equal(c.actions.size,0);
});
