import test from 'node:test';
import assert from 'node:assert/strict';
import { WeaponHistory } from '../web/weapon-history.ts';
import { InputChannel, emptyInput } from '../web/input.ts';

const player = changes => ({ weapon: 'deagle', life: 1, appliedSeq: 0, inventory: ['deagle', 'knife'].map(id => ({ id })), ...changes });

test('Digit3 then Q before its acknowledgement sends two ordered selects and preserves immediate history', () => {
  const history = new WeaponHistory();
  const sent = [];
  const owner = player({});
  history.confirm(owner, 'room');
  const channel = new InputChannel(action => { sent.push(action); return new Promise(() => {}); });
  channel.update({ identity: 'room', epoch: 'epoch', life: 1, receivedSeq: 0, enabled: true, window: 8, frame: 10 });
  const choose = id => channel.push({ ...emptyInput(), select: id }, action => history.select(id, action.seq, owner.inventory));
  choose('knife');
  assert.equal(history.current, 'knife');
  assert.equal(history.previous, 'deagle');
  choose(history.previous);
  assert.deepEqual(sent.map(action => action.select), ['knife', 'deagle']);
  assert.equal(history.current, 'deagle');
  history.confirm(player({ weapon: 'knife', appliedSeq: 1 }), 'room');
  assert.equal(history.current, 'deagle');
  assert.equal(history.previous, 'knife');
  history.confirm(player({ weapon: 'deagle', appliedSeq: 2 }), 'room');
  assert.equal(history.current, 'deagle');
  assert.equal(history.previous, 'knife');
});

test('rejected selections, recovery and life change restore authoritative equipment', () => {
  const history = new WeaponHistory();
  history.confirm(player({}), 'room');
  history.select('knife', 10, player({}).inventory);
  assert.equal(history.reject(10, player({}), 'room'), true);
  assert.equal(history.current, 'deagle');
  assert.equal(history.previous, '');
  history.select('knife', 11, player({}).inventory);
  history.confirm(player({}), 'room', true);
  assert.equal(history.current, 'deagle');
  history.confirm(player({ weapon: 'knife', life: 2 }), 'room');
  assert.equal(history.previous, '');
  assert.equal(history.select('awp', 12, player({}).inventory), false);
});
