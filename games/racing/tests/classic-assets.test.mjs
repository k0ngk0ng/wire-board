// Failure scenarios are declared in the v3 implementation plan before code:
// wrong-route preloading, duplicate car requests, premature readiness, corrupt
// byte/hash acceptance, cancellation poisoning future attempts, unbounded cache.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { ClassicAssetCache, selectClassicAssets, fetchClassicFiles, classicResourceBase } from '../web/classic-assets.ts';

const record = (path, text) => ({path, bytes: Buffer.byteLength(text), sha256: createHash('sha256').update(text).digest('hex'), mime: 'application/octet-stream'});
const paths = ['tracks/france.glb', 'tracks/france.route.json', 'tracks/france-sky.png', 'tracks/france-night.glb', 'tracks/france-night-sky.png', 'tracks/uk.glb', 'tracks/uk.route.json', 'tracks/uk-sky.png', 'cars/p911.glb', 'cars/fm50.glb', 'cars/ldia.glb'];
const manifest = {version: 3, assets: paths.map(p => record(p, p))};

test('new racing assets bind to the current build CDN base, never a legacy media directory',()=>{
  assert.equal(classicResourceBase('/'), '');
  const base='https://cdn.ichenj.com/bga/web/'+'a'.repeat(40)+'/';
  assert.equal(classicResourceBase(base),base.slice(0,-1));
  for(const invalid of ['https://cdn.ichenj.com/bga/v20261002-1','https://outside.example/','../racing'])assert.throws(()=>classicResourceBase(invalid));
});

test('selected daylight route and current cars only; duplicate cars deduplicated', () => {
  const files = selectClassicAssets(manifest, 'france', 'day', ['p911', 'fm50', 'p911']);
  assert.deepEqual(new Set(files.map(f => f.path)), new Set(['tracks/france.glb', 'tracks/france.route.json', 'tracks/france-sky.png', 'cars/p911.glb', 'cars/fm50.glb']));
  assert.equal(files.length, 5);
  assert(files.every(f => !f.path.includes('uk') && !f.path.includes('night')));
});

test('night shares the same rule route, but loads only night scene and sky', () => {
  const files = selectClassicAssets(manifest, 'france', 'night', ['p911']);
  assert.deepEqual(new Set(files.map(f => f.path)), new Set(['tracks/france-night.glb', 'tracks/france.route.json', 'tracks/france-night-sky.png', 'cars/p911.glb']));
  assert.throws(() => selectClassicAssets(manifest, '../france', 'day', ['p911']));
  assert.throws(() => selectClassicAssets(manifest, 'france', 'snow', ['p911']));
  assert.throws(() => selectClassicAssets(manifest, 'france', 'day', ['unknown']));
});

test('streamed bytes and cached verified bytes produce real aggregate progress', async () => {
  const files = [record('a.glb', 'abcdef'), record('b.glb', '123456789')];
  const cache = new ClassicAssetCache(1024);
  const calls = [], progress = [];
  const fetcher = async url => {
    calls.push(url);
    const text = url.endsWith('/a.glb') ? 'abcdef' : '123456789';
    const bytes = new TextEncoder().encode(text);
    return new Response(new ReadableStream({start(c) { c.enqueue(bytes.slice(0, 2)); c.enqueue(bytes.slice(2)); c.close(); }}));
  };
  const values = await fetchClassicFiles(files, {base: 'https://assets.example/reference', cache, fetcher, progress: p => progress.push({...p})});
  assert.equal(values.size, 2);
  assert.deepEqual(calls.sort(), ['https://assets.example/reference/racing/v3/a.glb', 'https://assets.example/reference/racing/v3/b.glb']);
  assert(progress.some(p => p.loaded > 0 && p.loaded < 15));
  assert.equal(progress.at(-1).loaded, 15);
  assert.equal(progress.at(-1).total, 15);
  await fetchClassicFiles(files, {base: 'https://assets.example/reference', cache, fetcher, progress: p => progress.push({...p})});
  assert.equal(calls.length, 2, 'verified cache was downloaded twice');
  assert.equal(progress.at(-1).cached, 15);
});

test('wrong size or digest never becomes a verified cached resource', async () => {
  const cache = new ClassicAssetCache(1024), file = record('a.glb', 'abcdef');
  await assert.rejects(fetchClassicFiles([file], {base: '', cache, fetcher: async () => new Response('abc')}), /大小|size/i);
  await assert.rejects(fetchClassicFiles([file], {base: '', cache, fetcher: async () => new Response('xxxxxx')}), /校验|digest|hash/i);
  const values = await fetchClassicFiles([file], {base: '', cache, fetcher: async () => new Response('abcdef')});
  assert.equal(new TextDecoder().decode(values.get(file.path)), 'abcdef');
});

test('an aborted load does not poison a later retry', async () => {
  const cache = new ClassicAssetCache(1024), file = record('a.glb', 'abcdef');
  const abort = new AbortController(); abort.abort();
  await assert.rejects(fetchClassicFiles([file], {base: '', cache, signal: abort.signal, fetcher: async () => new Response('abcdef')}), e => e.name === 'AbortError');
  assert.equal((await fetchClassicFiles([file], {base: '', cache, fetcher: async () => new Response('abcdef')})).size, 1);
});

test('verified byte cache stays bounded and evicts least recently used entries', async () => {
  const cache = new ClassicAssetCache(8), calls = [];
  const fetcher = async url => { calls.push(url); return new Response(url.endsWith('a.glb') ? 'aaaaaa' : 'bbbbbb'); };
  const a = record('a.glb', 'aaaaaa'), b = record('b.glb', 'bbbbbb');
  await fetchClassicFiles([a], {base: '', cache, fetcher});
  await fetchClassicFiles([b], {base: '', cache, fetcher});
  await fetchClassicFiles([a], {base: '', cache, fetcher});
  assert.equal(calls.length, 3);
  assert(cache.bytes <= 8);
});
