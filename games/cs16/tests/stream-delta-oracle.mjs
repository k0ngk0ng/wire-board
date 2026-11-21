import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { mergeCS16StreamData } from '../web/stream-delta.ts';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const fixture = resolve(root, '.local/cs16/verification/server/field-delta-cross-language.json');
const output = resolve(root, '.local/cs16/verification/public-delta/cross-language.json');
const bytes = readFileSync(fixture);
const samples = JSON.parse(bytes);
const results = [];
for (const [index, sample] of samples.entries()) {
  const merged = mergeCS16StreamData(sample.before, sample.packet);
  assert.deepEqual(merged, { ...sample.after, recover: false });
  results.push({ index, beforeFrame: sample.before.frame, afterFrame: merged.frame, packetBytes: Buffer.byteLength(JSON.stringify(sample.packet)), passed: true });
}
mkdirSync(dirname(output), { recursive: true });
writeFileSync(output, JSON.stringify({ passed: true, producer: 'Go TestStreamFieldDeltasRebuildExactAuthoritativeView', sourceSHA256: createHash('sha256').update(bytes).digest('hex'), samples: results }, null, 2) + '\n');
process.stdout.write(JSON.stringify({ passed: true, samples: results.length, output }) + '\n');
