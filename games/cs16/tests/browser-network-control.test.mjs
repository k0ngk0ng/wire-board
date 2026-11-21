import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { resolve, join } from 'node:path';
import { withProxyReconnect } from './browser-network-control.mjs';

const root = resolve('.local/cs16/browser/network-control-tests');
mkdirSync(root, { recursive: true });
const fixture = config => {
  const directory = mkdtempSync(join(root, 'case-'));
  const path = join(directory, 'network-main.json');
  const bytes = JSON.stringify(config, null, 2) + '\n';
  writeFileSync(path, bytes);
  return { directory, path, bytes, clean: () => rmSync(directory, { recursive: true, force: true }) };
};

test('controlled reconnect preserves exact RTT jitter bytes and other clients', async () => {
  const value = fixture({ rtt: 250, jitter: 20, offline: false, extra: 'retained' });
  const peer = join(value.directory, 'network-peer.json');
  writeFileSync(peer, '{"rtt":40,"jitter":5,"offline":false}\n');
  const peerBytes = readFileSync(peer);
  const order = [];
  try {
    const result = await withProxyReconnect(value.path, async () => {
      order.push('closed');
      assert.deepEqual(JSON.parse(readFileSync(value.path, 'utf8')), { rtt: 250, jitter: 20, offline: true, extra: 'retained' });
      return { at: 12 };
    }, async () => {
      order.push('baseline');
      assert.equal(readFileSync(value.path, 'utf8'), value.bytes);
      return { schema: 1, epoch: 'one', life: 2 };
    });
    assert.deepEqual(order, ['closed', 'baseline']);
    assert.equal(readFileSync(value.path, 'utf8'), value.bytes);
    assert.deepEqual(readFileSync(peer), peerBytes);
    assert.equal(result.original.rtt, 250);
    assert.equal(result.baseline.schema, 1);
  } finally { value.clean(); }
});

for (const failure of ['closed', 'baseline']) test('restores control bytes when ' + failure + ' observation fails', async () => {
  const value = fixture({ rtt: 40, jitter: 5, offline: false });
  try {
    await assert.rejects(withProxyReconnect(value.path, async () => { if (failure === 'closed') throw Error('closed failure'); }, async () => { throw Error('baseline failure'); }));
    assert.equal(readFileSync(value.path, 'utf8'), value.bytes);
  } finally { value.clean(); }
});

test('rejects a path outside the owned directory without accessing it', async () => {
  await assert.rejects(withProxyReconnect(resolve('games/cs16/controls.json'), async () => {}, async () => {}));
});

test('rejects a symbolic link without modifying the target', async () => {
  const value = fixture({ rtt: 40, jitter: 5, offline: false });
  const target = join(value.directory, 'target.json');
  try {
    writeFileSync(target, value.bytes);
    rmSync(value.path);
    symlinkSync(target, value.path);
    await assert.rejects(withProxyReconnect(value.path, async () => {}, async () => {}));
    assert.equal(readFileSync(target, 'utf8'), value.bytes);
  } finally { value.clean(); }
});

for (const config of [{ rtt: 40, jitter: 5, offline: true }, { rtt: '40', jitter: 5, offline: false }, { rtt: 1501, jitter: 5, offline: false }, { rtt: 40, jitter: -1, offline: false }]) test('rejects invalid or already-offline proxy configuration ' + JSON.stringify(config), async () => {
  const value = fixture(config);
  try {
    await assert.rejects(withProxyReconnect(value.path, async () => {}, async () => {}));
    assert.equal(readFileSync(value.path, 'utf8'), value.bytes);
  } finally { value.clean(); }
});

test('rejects invalid filename and malformed JSON without mutation', async () => {
  const value = fixture({ rtt: 40, jitter: 5, offline: false });
  try {
    const other = join(value.directory, 'other.json');
    writeFileSync(other, value.bytes);
    await assert.rejects(withProxyReconnect(other, async () => {}, async () => {}));
    assert.equal(readFileSync(other, 'utf8'), value.bytes);
    writeFileSync(value.path, 'malformed\n');
    await assert.rejects(withProxyReconnect(value.path, async () => {}, async () => {}));
    assert.equal(readFileSync(value.path, 'utf8'), 'malformed\n');
  } finally { value.clean(); }
});
