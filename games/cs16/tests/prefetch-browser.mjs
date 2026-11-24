import assert from 'node:assert/strict';
import { mkdir, readFile, writeFile, rm } from 'node:fs/promises';
import { execFile, execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { promisify } from 'node:util';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const out = path.join(root, '.local/cs16/prefetch');
const temporary = path.join(out, 'temporary');
const env = { ...process.env, TMPDIR: path.join(root, '.local/cs16/t'), AGENT_BROWSER_SOCKET_DIR: path.join(root, '.local/cs16/s'), AGENT_BROWSER_NO_VERSION_CHECK: '1', XDG_CONFIG_HOME: path.join(temporary, 'config'), XDG_DATA_HOME: path.join(temporary, 'data'), XDG_CACHE_HOME: path.join(temporary, 'cache') };
for (const folder of [out, temporary, env.TMPDIR, env.AGENT_BROWSER_SOCKET_DIR, env.XDG_CONFIG_HOME, env.XDG_DATA_HOME, env.XDG_CACHE_HOME]) await mkdir(folder, { recursive: true });
const require = createRequire(new URL('../../../web/package.json', import.meta.url));
const baseline = process.env.PREFETCH_BASELINE === '1';
const baselineRef = process.env.PREFETCH_BASELINE_REF ?? 'HEAD';
const original = baseline ? execFileSync('git', ['show', `${baselineRef}:web/src/game-api/load-assets.ts`], { cwd: root, encoding: 'utf8' }) : undefined;
await promisify(execFile)('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', path.join(temporary, 'key.pem'), '-out', path.join(temporary, 'cert.pem'), '-subj', '/CN=127.0.0.1', '-days', '1'], { cwd: root, env });
const requests = [];
const protocols = new Set();
let active = 0;
let peak = 0;
let first = 0;
let last = 0;
const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><path fill="#3b6558" d="M0 0h8v8H0z"/></svg>';
const crc = buffer => { let value = -1; for (const byte of buffer) { value ^= byte; for (let bit = 0; bit < 8; bit++) value = value >>> 1 ^ (value & 1 ? 0xedb88320 : 0); } return (value ^ -1) >>> 0; };
const chunk = (type, data) => { const body = Buffer.concat([Buffer.from(type), data]); const prefix = Buffer.alloc(4); prefix.writeUInt32BE(data.length); const suffix = Buffer.alloc(4); suffix.writeUInt32BE(crc(body)); return Buffer.concat([prefix, body, suffix]); };
const header = Buffer.alloc(13); header.writeUInt32BE(8); header.writeUInt32BE(8, 4); header[8] = 8; header[9] = 6;
const pixels = Buffer.alloc(8 * 33, 0); for (let row = 0; row < 8; row++) for (let column = 0; column < 8; column++) { const offset = row * 33 + 1 + column * 4; pixels[offset] = 58; pixels[offset + 1] = 101; pixels[offset + 2] = 88; pixels[offset + 3] = 255; }
const png = Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]), chunk('IHDR', header), chunk('IDAT', require('node:zlib').deflateSync(pixels)), chunk('IEND', Buffer.alloc(0))]);
const media = Object.fromEntries(Array.from({ length: 64 }, (_, index) => [`/cs16/prefetch/texture-${index}.${index % 2 ? 'svg' : 'png'}`, index % 2 ? Buffer.byteLength(svg) : png.length]));
const entry = `import {prepareGame} from '/@fs${path.join(root, 'web/src/game-api/load-assets.ts')}';import {sealResidentResources} from '/@fs${path.join(root, 'web/src/game-api/resident-assets.ts')}';window.prefetchProbe={progress:[],run:async()=>{window.prefetchProbe.done=false;try{const lease=crypto.randomUUID(),signal=new AbortController().signal;await prepareGame({kind:'cs16'},'',signal,p=>window.prefetchProbe.progress.push(p),lease);${baseline ? "" : "await sealResidentResources(lease,signal)"}}catch(error){window.prefetchProbe.error=String(error)}window.prefetchProbe.done=true}};await window.prefetchProbe.run()`;
await writeFile(path.join(temporary, 'index.html'), '<meta name="viewport" content="width=device-width,initial-scale=1"><script type="module" src="/prefetch-entry.js"></script>');
await writeFile(path.join(temporary, 'prefetch-entry.js'), entry);
const fixture = { name: 'prefetch-full-resource-matrix', enforce: 'pre', transform(code, id) { return original && id === path.join(root, 'web/src/game-api/load-assets.ts') ? original : undefined; }, configureServer(server) { server.middlewares.use((request, response, next) => {
  if (request.url === '/prefetch-entry.js') { response.setHeader('Content-Type', 'application/javascript'); response.end(entry); }
  else if (request.url === '/game-load-manifest.json') { response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify({ build: 'prefetch-fixture', entry: '/prefetch-entry.js', games: { cs16: [] }, media })); }
  else if (request.url?.startsWith('/cs16/prefetch/')) {
    protocols.add(request.httpVersion);
    first ||= performance.now(); active++; peak = Math.max(peak, active); requests.push(request.url);
    setTimeout(() => { const vector = request.url.endsWith('.svg'); response.setHeader('Content-Type', vector ? 'image/svg+xml' : 'image/png'); response.setHeader('Cache-Control', 'public,max-age=31536000,immutable'); response.end(vector ? svg : png); active--; last = performance.now(); }, 60);
  } else next();
}); } };
const server = await require('vite').createServer({ configFile: false, root: temporary, cacheDir: path.join(temporary, 'vite'), publicDir: false, plugins: [require('@vitejs/plugin-react').default({ include: /\.[jt]sx$/ }), fixture], optimizeDeps: { include: ['react', 'react-dom/client', 'lucide-react', 'three', 'fflate', 'three/examples/jsm/loaders/GLTFLoader.js', 'three/examples/jsm/utils/SkeletonUtils.js'] }, resolve: { alias: { react: path.join(root, 'web/node_modules/react'), 'react-dom': path.join(root, 'web/node_modules/react-dom'), 'lucide-react': path.join(root, 'web/node_modules/lucide-react'), three: path.join(root, 'web/node_modules/three'), fflate: path.join(root, 'web/node_modules/fflate') }, dedupe: ['react', 'react-dom', 'three', 'fflate', 'lucide-react'] }, server: { headers: { 'Service-Worker-Allowed': '/', 'Content-Security-Policy': "img-src 'self'" }, host: '127.0.0.1', port: 0, https: { key: await readFile(path.join(temporary, 'key.pem')), cert: await readFile(path.join(temporary, 'cert.pem')) }, fs: { allow: [root] } } });
await server.listen();
const execute = promisify(execFile);
const session = (await execute('agent-browser', ['session', 'id', '--scope', 'worktree', '--prefix', 'pf'], { cwd: root, env })).stdout.trim();
const browser = async (...args) => { const result = await execute('agent-browser', ['--session', session, '--args', '--ignore-certificate-errors', '--profile', path.join(temporary, 'profile'), ...args, '--json'], { cwd: root, env, timeout: 30000 }); const parsed = JSON.parse(result.stdout); assert.ok(parsed.success, result.stdout); return parsed.data; };
const report = { passed: false, scope: 'Production prepareGame and actual CS module; all 64 fixture textures must be prefetched and decoded before readiness, controlled 60 ms response latency; no production CDN speed claim', requests };
let socket;
try {
  await browser('open', server.resolvedUrls.local[0]);
  await browser('wait', '--fn', 'window.prefetchProbe?.done===true');
  const measured = await browser('eval', '({error:window.prefetchProbe.error,progress:window.prefetchProbe.progress})');
  report.probe = measured.result ?? measured;
  assert.ok(!report.probe.error, report.probe.error);
  report.peakRequests = peak; report.transferMillis = Math.round(last - first); report.uniqueResources = new Set(requests).size; report.protocols = [...protocols];
  assert.ok(protocols.has('2.0'), 'latency comparison requires HTTP/2');
  assert.equal(report.uniqueResources, 64, 'every texture must be prefetched');
  const beforeWarm = requests.length;
  report.coldRequests = beforeWarm;
  await browser('eval', 'window.prefetchProbe.run()');
  assert.equal(requests.length, beforeWarm, 'verified warm prefetch must not redownload resources');
  const endpoint = await browser('get', 'cdp-url');
  socket = new WebSocket(endpoint.cdpUrl);
  await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
  let sequence = 0; const pending = new Map();
  socket.onmessage = ({ data }) => { const value = JSON.parse(data); const request = pending.get(value.id); if (!request) return; pending.delete(value.id); value.error ? request.reject(Error(JSON.stringify(value.error))) : request.resolve(value.result); };
  const call = (method, params = {}, sessionId) => new Promise((resolve, reject) => { const id = ++sequence; pending.set(id, { resolve, reject }); socket.send(JSON.stringify({ id, method, params, sessionId })); });
  const targets = await call('Target.getTargets');
  const { sessionId } = await call('Target.attachToTarget', { targetId: targets.targetInfos.find(target => target.type === 'page' && target.url.startsWith(server.resolvedUrls.local[0])).targetId, flatten: true });
  await call('HeapProfiler.collectGarbage', {}, sessionId);
  const prototype = await call('Runtime.evaluate', { expression: 'HTMLImageElement.prototype' }, sessionId);
  const objects = await call('Runtime.queryObjects', { prototypeObjectId: prototype.result.objectId }, sessionId);
  const retained = await call('Runtime.callFunctionOn', { objectId: objects.objects.objectId, functionDeclaration: 'function(){return this.length}', returnByValue: true }, sessionId);
  report.retainedImages = retained.result.value;
  report.passed = peak >= 12 && report.retainedImages <= 2;
  assert.ok(report.passed, JSON.stringify({ peak, retainedImages: report.retainedImages, transferMillis: report.transferMillis }));
} finally {
  report.sourceSHA256 = createHash('sha256').update(original ?? await readFile(path.join(root, 'web/src/game-api/load-assets.ts'))).digest('hex');
  if (baseline) report.baselineRef = execFileSync('git', ['rev-parse', baselineRef], { cwd: root, encoding: 'utf8' }).trim();
  socket?.close();
  await writeFile(path.join(out, baseline ? 'baseline.json' : 'result.json'), JSON.stringify(report, null, 2) + '\n');
  await browser('close').catch(() => {});
  await server.close();
  await rm(temporary, { recursive: true, force: true });
}
