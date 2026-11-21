import assert from 'node:assert/strict';
import { lstatSync, readFileSync, realpathSync, writeFileSync } from 'node:fs';
import { basename, relative, resolve, sep } from 'node:path';

export async function withProxyReconnect(path, waitClosed, waitBaseline) {
  const owned = resolve('.local/cs16');
  const selected = resolve(path);
  assert.ok(selected.startsWith(owned + sep), 'network control must stay within .local/cs16');
  assert.match(basename(selected), /^network-(main|peer|watch)\.json$/);
  const metadata = lstatSync(selected);
  assert.ok(metadata.isFile() && !metadata.isSymbolicLink(), 'network control must be an ordinary file');
  assert.ok(realpathSync(selected).startsWith(realpathSync(owned) + sep), 'network control resolved outside .local/cs16');
  const bytes = readFileSync(selected);
  const original = JSON.parse(bytes.toString('utf8'));
  assert.ok(Number.isInteger(original.rtt) && original.rtt >= 0 && original.rtt <= 1500);
  assert.ok(Number.isInteger(original.jitter) && original.jitter >= 0 && original.jitter <= 250);
  assert.equal(original.offline, false, 'ordinary input precondition requires an online proxy');
  try {
    writeFileSync(selected, JSON.stringify({ ...original, offline: true }) + '\n');
    const closed = await waitClosed();
    writeFileSync(selected, bytes);
    const baseline = await waitBaseline();
    assert.deepEqual(readFileSync(selected), bytes, 'proxy configuration changed during baseline setup');
    return { path: relative(resolve('.'), selected), original, closed, baseline, restored: original };
  } finally {
    writeFileSync(selected, bytes);
  }
}
