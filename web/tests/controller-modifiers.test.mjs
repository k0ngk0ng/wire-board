import test from 'node:test';
import assert from 'node:assert/strict';
import { KeyboardController, mappedModifiersAllowed } from '../src/controllers/input.ts';

const profile = { revision: 0, turboHz: 10, keyboard: { forward: 'KeyW', reload: 'KeyR', crouch: 'ControlLeft', walk: 'ShiftLeft' } };
const event = changes => ({ code: 'KeyW', ctrlKey: false, metaKey: false, altKey: false, ...changes });

test('mapped Ctrl combinations require an opted-in active game and a held mapped modifier', () => {
  const input = new KeyboardController(profile);
  input.press('ControlLeft', 0);
  const held = code => input.heldKey(code);
  assert.equal(mappedModifiersAllowed(profile, event({ ctrlKey: true }), true, held), true);
  assert.equal(mappedModifiersAllowed(profile, event({ ctrlKey: true, code: 'KeyR' }), true, held), true);
  assert.equal(mappedModifiersAllowed(profile, event({ ctrlKey: true }), false, held), false);
  input.clear();
  assert.equal(mappedModifiersAllowed(profile, event({ ctrlKey: true }), true, held), false);
});

test('Meta, unbound Alt and unbound combinations retain browser shortcut behavior', () => {
  const held = () => true;
  assert.equal(mappedModifiersAllowed(profile, event({ ctrlKey: true, metaKey: true }), true, held), false);
  assert.equal(mappedModifiersAllowed(profile, event({ altKey: true }), true, held), false);
  assert.equal(mappedModifiersAllowed(profile, event({ ctrlKey: true, code: 'KeyL' }), true, held), false);
  assert.equal(mappedModifiersAllowed(profile, event({ code: 'ControlLeft', ctrlKey: true }), false, held), true);
  assert.equal(mappedModifiersAllowed(profile, event({}), false, held), true);
});
