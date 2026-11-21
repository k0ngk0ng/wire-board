import test from 'node:test';
import assert from 'node:assert/strict';
import { TouchOwnership, PointerLockHandoff, bindTouchOwnership } from '../web/touch-ownership.ts';

function fixture(locked = false) {
  const context = { locked, releaseOld: 0, unlockRequests: 0, interrupted: 0, disabled: false, native: 0, states: [], looks: [], errors: [], targets: new Map() };
  const handoff = new PointerLockHandoff();
  const input = new TouchOwnership({
    locked: () => context.locked,
    begin: () => handoff.begin(context.locked, () => { context.releaseOld++; }, () => { context.unlockRequests++; }),
    changed: state => context.states.push({ moveX: state.moveX, moveY: state.moveY, actions: [...state.actions] }),
    look: (x, y) => context.looks.push([x, y]),
    interrupted: () => { context.interrupted++; },
    failed: error => context.errors.push(error.message),
  });
  const source = new EventTarget();
  const lockSource = new EventTarget();
  const dispose = bindTouchOwnership(source, lockSource, input, () => context.disabled, () => { context.native++; });
  const target = (name, fail = false) => {
    const value = { captures: [], releases: [], captured: new Set(),
      setPointerCapture(id) { if (context.locked || fail) throw new DOMException('capture rejected', 'InvalidStateError'); this.captures.push(id); this.captured.add(id); },
      hasPointerCapture(id) { return this.captured.has(id); },
      releasePointerCapture(id) { this.releases.push(id); this.captured.delete(id); },
    };
    context.targets.set(name, value);
    return value;
  };
  return { context, handoff, input, source, lockSource, dispose, target };
}

function pointer(source, type, pointerId, clientX = 0, clientY = 0) {
  const event = new Event(type, { cancelable: true });
  Object.defineProperties(event, { pointerId: { value: pointerId }, clientX: { value: clientX }, clientY: { value: clientY } });
  source.dispatchEvent(event);
  return event;
}

test('locked mouse hands off once and publishes the first touch before delayed unlock or capture', () => {
  const view = fixture(true);
  const move = view.target('move');
  const fire = view.target('fire');
  view.input.press(1, 'move', 10, 10, move);
  assert.deepEqual(view.context.states, [{ moveX: 0, moveY: 0, actions: [] }]);
  view.input.press(2, 'fire', 100, 100, fire);
  assert.deepEqual(view.context.states.at(-1).actions, ['fire']);
  assert.equal(view.context.releaseOld, 1);
  assert.equal(view.context.unlockRequests, 1);
  assert.equal(view.handoff.pending, true);
  assert.deepEqual(move.captures, []);
  assert.deepEqual(fire.captures, []);
  view.context.locked = false;
  assert.equal(view.handoff.lockChanged(false), false);
  view.lockSource.dispatchEvent(new Event('pointerlockchange'));
  assert.deepEqual(move.captures, [1]);
  assert.deepEqual(fire.captures, [2]);
  assert.deepEqual(view.input.sample().actions, new Set(['fire']));
  assert.equal(view.handoff.pending, false);
  assert.deepEqual(view.context.errors, []);
  view.dispose();
});

test('window events continue both fingers outside targets before capture and ignore unrelated pointers', () => {
  const view = fixture(true);
  view.input.press(1, 'move', 0, 0, view.target('move'));
  view.input.press(2, 'fire', 100, 100, view.target('fire'));
  assert.equal(pointer(view.source, 'pointermove', 99, 500, 500).defaultPrevented, false);
  assert.equal(pointer(view.source, 'pointermove', 1, 500, 500).defaultPrevented, true);
  pointer(view.source, 'pointermove', 2, 600, 700);
  assert.ok(view.input.sample().moveX > 0);
  assert.deepEqual(view.context.looks, [[500, 600]]);
  pointer(view.source, 'lostpointercapture', 1);
  assert.equal(view.input.owns(1), true);
  pointer(view.source, 'pointerup', 1);
  assert.equal(view.input.sample().moveX, 0);
  assert.equal(view.input.sample().actions.has('fire'), true);
  pointer(view.source, 'pointercancel', 2);
  assert.equal(view.input.sample().actions.has('fire'), false);
  assert.equal(view.context.interrupted, 1);
  view.context.locked = false;
  view.lockSource.dispatchEvent(new Event('pointerlockchange'));
  assert.deepEqual([...view.context.targets.values()].flatMap(value => value.captures), []);
  view.dispose();
});

test('ordinary capture and duplicate lost capture release only their owned finger', () => {
  const view = fixture();
  const move = view.target('move');
  const fire = view.target('fire');
  view.input.press(1, 'move', 0, 0, move);
  view.input.press(2, 'fire', 100, 100, fire);
  pointer(view.source, 'pointermove', 1, 24, 0);
  pointer(view.source, 'pointerup', 2);
  pointer(view.source, 'lostpointercapture', 2);
  assert.equal(view.input.sample().moveX, 500);
  assert.equal(view.input.sample().actions.has('fire'), false);
  assert.equal(view.context.interrupted, 0);
  pointer(view.source, 'lostpointercapture', 1);
  assert.equal(view.input.owns(1), true);
  move.captured.delete(1);
  pointer(view.source, 'lostpointercapture', 1);
  assert.equal(view.input.sample().moveX, 0);
  assert.deepEqual(move.releases, []);
  assert.deepEqual(fire.releases, [2]);
  view.dispose();
});

test('clear, disabled routing and disposal remove contacts and pending captures without revival', () => {
  for (const pending of [false, true]) {
    const view = fixture(pending);
    const move = view.target('move');
    const fire = view.target('fire');
    view.input.press(1, 'move', 0, 0, move);
    view.input.press(2, 'fire', 100, 100, fire);
    view.input.press(3, 'crouch', 100, 100, view.target('crouch'));
    view.context.disabled = true;
    assert.equal(pointer(view.source, 'pointermove', 1, 24, 0).defaultPrevented, false);
    view.input.clear();
    assert.deepEqual(view.input.sample(), { moveX: 0, moveY: 0, actions: new Set() });
    view.context.locked = false;
    view.lockSource.dispatchEvent(new Event('pointerlockchange'));
    assert.equal(view.input.owns(1), false);
    assert.equal(view.input.owns(2), false);
    assert.equal(view.context.interrupted, 1);
    if (pending) assert.deepEqual(move.captures, []);
    view.context.disabled = false;
    view.input.press(4, 'move', 0, 0, move);
    view.dispose();
    assert.equal(pointer(view.source, 'pointermove', 4, 24, 0).defaultPrevented, false);
    assert.equal(view.input.owns(4), false);
  }
});

test('unexpected capture failure cancels only that contact and publishes surviving movement', () => {
  for (const pending of [false, true]) {
    const view = fixture(pending);
    view.input.press(1, 'move', 0, 0, view.target('move'));
    pointer(view.source, 'pointermove', 1, 24, 0);
    view.input.press(2, 'fire', 100, 100, view.target('fire', true));
    if (pending) {
      view.context.locked = false;
      view.lockSource.dispatchEvent(new Event('pointerlockchange'));
    }
    assert.equal(view.input.owns(2), false);
    assert.equal(view.input.owns(1), true);
    assert.equal(view.input.sample().moveX, 500);
    assert.equal(view.input.sample().actions.has('fire'), false);
    assert.equal(view.context.interrupted, 1);
    assert.deepEqual(view.context.errors, ['capture rejected']);
    assert.deepEqual(view.context.states.at(-1), { moveX: 500, moveY: 0, actions: [] });
    view.dispose();
  }
});

test('normal unlock keeps its cleanup while failed or cancelled handoff cannot suppress a later unlock', () => {
  const handoff = new PointerLockHandoff();
  assert.equal(handoff.lockChanged(true), false);
  assert.equal(handoff.lockChanged(false), true);
  assert.throws(() => handoff.begin(true, () => {}, () => { throw Error('exit failed'); }), /exit failed/);
  assert.equal(handoff.pending, false);
  assert.equal(handoff.lockChanged(false), true);
  handoff.begin(true, () => {}, () => {});
  handoff.reset();
  assert.equal(handoff.lockChanged(false), true);
  const view = fixture();
  const failed = new TouchOwnership({ locked: () => true, begin: () => { throw Error('exit failed'); }, changed: () => assert.fail('failed handoff published a contact'), look: () => {}, interrupted: () => {}, failed: error => view.context.errors.push(error.message) });
  failed.press(1, 'fire', 0, 0, view.target('fire'));
  assert.equal(failed.owns(1), false);
  assert.deepEqual(view.context.errors, ['exit failed']);
  view.dispose();
});

test('disposing touch routing clears captures without notifying a disposed owner', () => {
  for (const pending of [false, true]) {
    const view = fixture(pending);
    const move = view.target('move');
    const fire = view.target('fire');
    view.input.press(1, 'move', 0, 0, move);
    view.input.press(2, 'fire', 100, 100, fire);
    pointer(view.source, 'pointermove', 1, 24, 0);
    const notifications = view.context.states.length;
    const interruptions = view.context.interrupted;
    view.dispose();
    assert.equal(view.context.states.length, notifications);
    assert.equal(view.context.interrupted, interruptions);
    assert.deepEqual(view.input.sample(), { moveX: 0, moveY: 0, actions: new Set() });
    assert.equal(move.captured.size, 0);
    assert.equal(fire.captured.size, 0);
    view.context.locked = false;
    view.lockSource.dispatchEvent(new Event('pointerlockchange'));
    assert.equal(view.input.owns(1), false);
    assert.equal(view.input.owns(2), false);
    assert.equal(pointer(view.source, 'pointermove', 1, 48, 0).defaultPrevented, false);
  }
});
