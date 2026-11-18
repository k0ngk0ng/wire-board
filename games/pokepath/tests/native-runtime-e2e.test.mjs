import test from "node:test";
import assert from "node:assert/strict";
import { access, mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { createHash } from "node:crypto";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { trajectorySource } from "./native-runtime-trajectory.mjs";

const execute = promisify(execFile);
const root = fileURLToPath(new URL("../../../", import.meta.url));
const builder = new URL("../build/browser-runtime.mjs", import.meta.url);
const output = join(root, ".local/pokepath/verification/native-runtime");
const temporary = join(root, ".local/pokepath/native-runtime-test");
const profile = join(temporary, "profile");
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const environment = { ...process.env, GOCACHE: join(root, ".local/go-build"), GOMODCACHE: join(root, ".local/go-mod"), GOTMPDIR: join(root, ".local/tmp"), TMPDIR: join(root, ".local/tmp"), TEST_TELEMETRY_DIR: join(root, ".local/go-telemetry"), GOTOOLCHAIN: "local", AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_NO_VERSION_CHECK: "1", AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter" };
const digest = bytes => createHash("sha256").update(bytes).digest("hex");
let session;

async function browser(...args) {
  const result = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 60000, maxBuffer: 12 << 20 });
  const value = JSON.parse(result.stdout);
  if (!value.success) throw Error(JSON.stringify(value));
  return value.data;
}

async function evaluate(code) {
  return (await browser("eval", "-b", Buffer.from(code).toString("base64"))).result;
}

const referenceProgram = `package main
import("encoding/json";"fmt";"os";"github.com/k0ngk0ng/wire-board/games/pokepath/engine")
func main(){var options engine.Options;if err:=json.Unmarshal([]byte("{\\"starter\\":\\"charmander\\"}"),&options);err!=nil{panic(err)};s,err:=engine.New(options,2,123456,1730000000000);if err!=nil{panic(err)};initial,err:=engine.Marshal(s);if err!=nil{panic(err)};var action engine.Action;if err=json.Unmarshal([]byte("{\\"type\\":\\"wave\\"}"),&action);err!=nil{panic(err)};action.Epoch=s.Epoch;if err=engine.Apply(s,1,action);err!=nil{panic(err)};if err=engine.Advance(s,400);err!=nil{panic(err)};final,err:=engine.Marshal(s);if err!=nil{panic(err)};cloned,err:=engine.Clone(s);if err!=nil{panic(err)};if err=engine.Advance(cloned,140);err!=nil{panic(err)};clone,err:=engine.Marshal(cloned);if err!=nil{panic(err)};out:=map[string]any{"initial":json.RawMessage(initial),"final":json.RawMessage(final),"view":engine.Project(s),"clone":json.RawMessage(clone),"cloneView":engine.Project(cloned)};if err=json.NewEncoder(os.Stdout).Encode(out);err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}}`;

test("our native Go rules load as verified browser Wasm and replay the same trajectory", { timeout: 180000 }, async () => {
  await access(builder);
  await mkdir(output, { recursive: true });
  await mkdir(temporary, { recursive: true });
  await mkdir(environment.AGENT_BROWSER_SOCKET_DIR, { recursive: true });
  await mkdir(environment.TMPDIR, { recursive: true });
  const { buildBrowserRuntime } = await import(builder.href);
  const artifact = buildBrowserRuntime();
  const manifest = JSON.parse(await readFile(join(artifact.runtimeRoot, "manifest.json"), "utf8"));
  assert.equal(manifest.format, 1);
  assert.equal(manifest.engine, "wire-board-pokepath-go");
  assert.equal(manifest.catalog.manifest.path, "../resources.json");
  const catalogRoot = join(root, "games/pokepath/assets/public/pokepath/v1");
  const catalogIndex = await readFile(join(catalogRoot, "resources.json"));
  assert.equal(manifest.catalog.manifest.bytes, catalogIndex.length);
  assert.equal(manifest.catalog.manifest.sha256, digest(catalogIndex));
  assert.ok(manifest.sourceFiles.every(file => file.path.startsWith("games/pokepath/engine/") || file.path.startsWith("games/pokepath/cmd/browser/") || ["go.mod", "go.sum"].includes(file.path) || manifest.dependencies.some(dependency => file.dependency === dependency.module && dependency.module === "golang.org/x/text" && dependency.version === "v0.34.0" && file.path.startsWith(dependency.sourcePrefix))));
  assert.ok(manifest.sourceFiles.every(file => !file.path.includes("upstream") && !file.path.endsWith(".js")));
  for (const entry of manifest.files) {
    const bytes = await readFile(join(artifact.runtimeRoot, entry.path));
    assert.equal(bytes.length, entry.bytes, entry.path);
    assert.equal(digest(bytes), entry.sha256, entry.path);
  }
  const license = await readFile(join(artifact.runtimeRoot, "go-runtime-LICENSE.txt"), "utf8");
  assert.match(license, /Copyright 2018 The Go Authors/);
  assert.match(license, /Redistribution and use in source and binary forms/);
  const nativeSource = join(temporary, "reference.go");
  await writeFile(nativeSource, referenceProgram);
  const reference = JSON.parse((await execute("go", ["run", nativeSource], { cwd: root, env: environment, timeout: 60000, maxBuffer: 12 << 20 })).stdout);
  const trajectoryPath = join(temporary, "trajectory.go");
  await writeFile(trajectoryPath, trajectorySource);
  const trajectory = JSON.parse((await execute("go", ["run", trajectoryPath], { cwd: root, env: environment, timeout: 90000, maxBuffer: 20 << 20 })).stdout);
  await writeFile(join(output, "trajectory.json"), JSON.stringify(trajectory) + "\n");
  const wrapper = await readFile(new URL("../web/native-runtime.ts", import.meta.url), "utf8");
  const compiled = await require("esbuild").transform(wrapper, { loader: "ts", target: "es2022", format: "esm", legalComments: "none" });
  const requests = [];
  const report = { command: "node --test games/pokepath/tests/native-runtime-e2e.test.mjs", passed: false, sourceSHA256: manifest.sourceSHA256, files: manifest.files, checks: [] };
  const server = createServer(async (request, response) => {
    requests.push(request.url);
    if (request.url === "/trajectory.json") { response.writeHead(200, { "Content-Type": "application/json" }); response.end(JSON.stringify(trajectory)); return; }
    if (request.url === "/fixture.js") {
      response.writeHead(200, { "Content-Type": "text/javascript" });
      response.end("import{loadNativeRuntime}from'/native-runtime.js';localStorage.setItem('wb_sound','off');window.progress=[];window.loadNative=loadNativeRuntime;window.ready=false;loadNativeRuntime({baseURL:'/good/',catalogBaseURL:'/facts/',route:0,onProgress:value=>window.progress.push(value)}).then(runtime=>{window.runtime=runtime;window.ready=true;document.querySelector('#status').textContent='原创 Go 规则已就绪'}).catch(error=>{window.error=error.message})");
      return;
    }
    if (request.url === "/native-runtime.js") { response.writeHead(200, { "Content-Type": "text/javascript" }); response.end(compiled.code); return; }
    const fact = request.url.match(/^\/(facts|facts-corrupt)\/(resources\.json|catalog\.json|routes\/\d+\.json)$/);
    if (fact) {
      const data = Buffer.from(await readFile(join(catalogRoot, fact[2])));
      if (fact[1] === "facts-corrupt" && fact[2].startsWith("routes/")) data[data.length - 1] ^= 1;
      response.writeHead(200, { "Content-Type": "application/json", "Cache-Control": "public,max-age=3600,immutable" });
      response.end(data);
      return;
    }
    const match = request.url.match(/^\/(good|cold|corrupt|missing|cancel|history-manifest|history-engine|history-support|history-license)\/([A-Za-z0-9_.-]+)$/);
    if (match) {
      if (match[1] === "missing" && match[2] === "engine.wasm") { response.writeHead(404); response.end("missing"); return; }
      try {
        const data = Buffer.from(await readFile(join(artifact.runtimeRoot, match[2])));
        if (match[1] === "corrupt" && match[2] === "engine.wasm") data[data.length - 1] ^= 1;
        const type = match[2].endsWith(".wasm") ? "application/wasm" : match[2].endsWith(".js") ? "text/javascript" : match[2].endsWith(".json") ? "application/json" : "text/plain";
        response.writeHead(200, { "Content-Type": type, "Content-Length": data.length, "Cache-Control": "public,max-age=3600,immutable" }); response.end(data);
      } catch { response.writeHead(404); response.end("missing"); }
      return;
    }
    response.writeHead(200, { "Content-Type": "text/html", "Content-Security-Policy": "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self'; object-src 'none'; frame-src 'none'" });
    response.end(`<!doctype html><html><head><meta charset="utf-8"><title>原创 Go 规则 Wasm 验收</title></head><body><div id="status">等待原创 Go 规则</div><script type="module" src="/fixture.js"></script></body></html>`);
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  const base = `http://127.0.0.1:${server.address().port}/`;
  session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "pn"], { cwd: root, env: environment, encoding: "utf8" })).stdout.trim();
  try {
    await browser("open", base);
    await browser("wait", "--fn", "window.ready===true||Boolean(window.error)");
    assert.equal(await evaluate("window.error??null"), null);
    const trajectory = await evaluate(`(()=>{const runtime=window.runtime;const initial=runtime.create({starter:'charmander'},2,123456,1730000000000);const copy=runtime.clone(initial);const wave=runtime.apply(copy,1,{type:'wave',epoch:copy.epoch});const final=runtime.advance(wave,400);const clone=runtime.advance(runtime.clone(final),140);return{initial,final,view:runtime.project(final),clone,cloneView:runtime.project(clone),unchanged:JSON.stringify(initial)===JSON.stringify(copy)}})()`);
    assert.deepEqual(trajectory.initial, reference.initial);
    assert.deepEqual(trajectory.final, reference.final);
    assert.deepEqual(trajectory.view, reference.view);
    assert.deepEqual(trajectory.clone, reference.clone);
    assert.deepEqual(trajectory.cloneView, reference.cloneView);
    assert.equal(trajectory.unchanged, true);
    const progress = await evaluate("window.progress");
    const finalProgress = progress.at(-1);
    assert.equal(finalProgress.loaded, finalProgress.total);
    assert.ok(finalProgress.total > 1000000);
    report.checks.push({ name: "same-native-and-browser-trajectory", passed: true, progress, project: trajectory.view });
    assert.ok(requests.includes("/facts/catalog.json"));
    assert.ok(requests.includes("/facts/routes/0.json"));
    assert.equal(requests.some(path => /^\/facts\/routes\/(?!0\.json$)/.test(path)), false, "boot fetched another route");
    const additionalRoute = await evaluate("window.runtime.ensureRoute(1).then(()=>({loaded:true,catalog:window.runtime.catalog.version}))");
    assert.equal(additionalRoute.loaded, true);
    assert.ok(requests.includes("/facts/routes/1.json"));
    assert.equal(requests.some(path => /^\/facts\/routes\/(?![01]\.json$)/.test(path)), false, "route navigation fetched unrelated routes");
    report.checks.push({ name: "core-and-selected-route-only", passed: true, requests: requests.filter(path => path.startsWith("/facts/")) });
    const foresight = await evaluate(`(()=>{const runtime=window.runtime,session=runtime.createPredictionRuntime();try{const initial=runtime.create({starter:'charmander'},2,123456,1730000000000),handle=session.import(initial),view=session.project(handle),pokemon=view.team[0],sites=session.deployment(handle,pokemon.id).sites,tile=view.tiles.find(tile=>sites[tile.id]==='empty'),invalid=view.tiles.find(tile=>sites[tile.id]==='blocked');const footprint=session.deploymentRange(handle,pokemon.id,tile.id);let invalidRejected=false;try{session.deploymentRange(handle,pokemon.id,invalid.id)}catch{invalidRejected=true}const untouched=JSON.stringify(session.export(handle))===JSON.stringify(initial);const after=session.apply(handle,1,{type:'deploy',pokemonID:pokemon.id,tileID:tile.id,epoch:initial.epoch}),placed=session.project(after);return{footprint,range:placed.team[0].combatStats.range,shape:placed.team[0].combatRangeType,center:{x:placed.towers[0].x,y:placed.towers[0].y},untouched,invalidRejected,live:session.liveStates}}finally{session.dispose()}})()`);
    assert.equal(foresight.untouched, true);
    assert.equal(foresight.invalidRejected, true);
    assert.equal(foresight.live, 2);
    assert.equal(foresight.footprint.range, foresight.range);
    assert.equal(foresight.footprint.shape, foresight.shape);
    assert.deepEqual(foresight.footprint.center, foresight.center);
    report.checks.push({ name: "read-only-deployment-footprint-agrees-with-retained-deploy", passed: true, ...foresight });
    const retained = await evaluate("(()=>{const runtime=window.runtime,session=runtime.createPredictionRuntime();const state=runtime.create({starter:'charmander'},2,123456,1730000000000),first=session.import(state),second=session.clone(first),wave=session.apply(second,1,{type:'wave',epoch:state.epoch}),final=session.advance(wave,400);const full=session.project(wave),view=session.project(final),exported=session.export(final),legacy=runtime.advance(runtime.apply(state,1,{type:'wave',epoch:state.epoch}),400);const result={sameState:JSON.stringify(exported)===JSON.stringify(legacy),view,legacyView:runtime.project(legacy),independent:session.export(first).waveActive===false,metadata:{clock:final.clock,epoch:final.epoch,paused:final.paused},before:session.liveStates};session.release(second);session.release(second);session.release({nativeHandle:999999});try{session.project(second);result.releasedRejected=false}catch{result.releasedRejected=true}session.dispose();result.after=session.liveStates;result.goLive=JSON.parse(globalThis.wireBoardPokepathNative.request(JSON.stringify({op:'info'}))).value.retainedStates;return result})()");
    assert.equal(retained.sameState, true);
    assert.deepEqual(retained.view, retained.legacyView);
    assert.equal(retained.independent, true);
    assert.equal(retained.releasedRejected, true);
    assert.equal(retained.after, 0);
    assert.equal(retained.goLive, 0);
    report.checks.push({ name: "retained-session-equivalence-and-release", passed: true, ...retained });
    const completeTrajectory = await evaluate("(async()=>{const script=await(await fetch('/trajectory.json')).json(),runtime=window.runtime,cold=await window.loadNative({baseURL:'/cold/',catalogBaseURL:'/facts/',route:0});let session=runtime.createPredictionRuntime(),state=session.import(runtime.create(script.options,script.players,script.seed,script.now));const checked=[];const compare=after=>{for(const point of script.checkpoints.filter(value=>value.after===after)){const snapshot=session.export(state),view=session.project(state);const equal=(a,b)=>{if(a===b)return true;if(!a||!b||typeof a!=='object'||typeof b!=='object')return false;const ak=Object.keys(a),bk=Object.keys(b);return ak.length===bk.length&&ak.every(key=>Object.hasOwn(b,key)&&equal(a[key],b[key]))};if(!equal(snapshot,point.state)||!equal(view,point.view))throw Error('native/Wasm trajectory differs at '+point.name);if(point.name==='projectile-in-flight'){session.dispose();session=cold.createPredictionRuntime();state=session.import(point.state)}checked.push({name:point.name,after,clock:snapshot.clock,towers:snapshot.towers.length,projectiles:snapshot.projectiles.length})}};try{compare(0);for(let index=0;index<script.operations.length;index++){const op=script.operations[index],next=op.type==='tick'?session.advance(state,op.milliseconds):session.apply(state,op.actor,op.action);session.release(state);state=next;compare(index+1)}return{checked,coverage:script.coverage,live:session.liveStates,coldWasmRestore:true}}finally{session.dispose()}})()");
    assert.ok(completeTrajectory.coverage.waves >= 5);
    assert.ok(completeTrajectory.coverage.maximumTowers >= 3);
    assert.ok(Object.keys(completeTrajectory.coverage.conditions).length >= 2);
    assert.ok(completeTrajectory.checked.some(point => point.name === "projectile-in-flight"));
    report.checks.push({ name: "multi-tower-conditions-flight-and-complete-wave-native-parity", passed: true, ...completeTrajectory });
    const invalid = await evaluate(`(()=>{const runtime=window.runtime;const state=runtime.create({starter:'charmander'},2,123456,1730000000000);return [()=>runtime.apply(state,-1,{type:'wave',epoch:state.epoch}),()=>runtime.advance(state,Infinity),()=>runtime.create({},2,Number.MAX_SAFE_INTEGER+1,1730000000000),()=>runtime.clone({schema:999})].map(fn=>{try{fn();return false}catch{return true}})})()`);
    assert.deepEqual(invalid, [true, true, true, true]);
    report.checks.push({ name: "invalid-json-and-multiplayer-boundaries", passed: true });
    const cached = await evaluate("window.cachedProgress=[];window.loadNative({baseURL:'/good/',catalogBaseURL:'/facts/',route:0,onProgress:value=>window.cachedProgress.push(value)}).then(value=>value===window.runtime)");
    assert.equal(cached, true);
    assert.equal((await evaluate("window.cachedProgress")).at(-1).cached, true);
    for (const directory of ["corrupt", "missing"]) {
      const failure = await evaluate(`window.loadNative({baseURL:'/${directory}/',catalogBaseURL:'/facts/',route:0}).then(()=>({passed:false}),error=>({passed:true,message:error.message}))`);
      assert.equal(failure.passed, true, directory);
      assert.match(failure.message, directory === "corrupt" ? /校验|digest|SHA|完整/i : /404|加载|请求|HTTP/i);
      report.checks.push({ name: `${directory}-runtime-rejected`, ...failure });
    }
    const sharedReads = await evaluate("(async()=>{const shared=new Map;for(const path of ['resources.json','catalog.json','routes/0.json'])shared.set('/pokepath/v1/'+path,await(await fetch('/facts/'+path)).arrayBuffer());const original=window.fetch;let duplicateFacts=0;window.fetch=(...args)=>{if(String(args[0]).includes('/facts/'))duplicateFacts++;return original(...args)};const seen=[];window.disposedResourceStore=false;const mapping={'/pokepath/v1/native/manifest.json':'/history-manifest/manifest.json','/pokepath/v1/native/engine.wasm':'/history-engine/engine.wasm','/pokepath/v1/native/wasm_exec.js':'/history-support/wasm_exec.js','/pokepath/v1/native/go-runtime-LICENSE.txt':'/history-license/go-runtime-LICENSE.txt'};try{window.historicalMapping=mapping;const runtime=await window.loadNative({baseURL:'/unused/',catalogBaseURL:'/facts/',route:0,resolveResource:path=>mapping[path],readResource:async path=>{if(window.disposedResourceStore)throw Error('disposed resource store');seen.push(path);return shared.get(path)},onProgress:value=>window.sharedProgress=value});return{duplicateFacts,seen,ready:runtime.project(runtime.create({starter:'charmander'},2,123456,1730000000000)).gold,progress:window.sharedProgress}}finally{window.fetch=original}})()");
    assert.equal(sharedReads.duplicateFacts, 0, "native loader fetched already provided data bytes again");
    assert.equal(sharedReads.ready, 50);
    assert.ok(sharedReads.seen.includes("/pokepath/v1/catalog.json"));
    assert.ok(sharedReads.progress.resources.some(file => file.path === "/pokepath/v1/native/engine.wasm"));
    assert.ok(requests.includes("/history-engine/engine.wasm") && requests.includes("/history-support/wasm_exec.js"));
    report.checks.push({ name: "shared-bytes-and-independent-historical-resource-urls", passed: true, ...sharedReads });
    const rebound = await evaluate("window.disposedResourceStore=true;window.reboundReads=[];window.loadNative({baseURL:'/unused/',catalogBaseURL:'/facts/',route:2,resolveResource:path=>window.historicalMapping[path],readResource:async path=>{window.reboundReads.push(path);return undefined}}).then(runtime=>({ready:true,reads:window.reboundReads,gold:runtime.project(runtime.create({starter:'charmander'},2,123456,1730000000000)).gold}))");
    assert.equal(rebound.ready, true);
    assert.equal(rebound.gold, 50);
    assert.ok(rebound.reads.includes("/pokepath/v1/routes/2.json"));
    report.checks.push({ name: "spa-resource-hooks-rebind-to-current-board", passed: true, ...rebound });
    const lateCancellation = await evaluate("(async()=>{const instantiate=WebAssembly.instantiate,before=globalThis.wireBoardPokepathNative,controller=new AbortController();WebAssembly.instantiate=async(...args)=>{const result=await instantiate.apply(WebAssembly,args);controller.abort();return result};try{await window.loadNative({baseURL:'/cancel/',catalogBaseURL:'/facts/',route:0,signal:controller.signal});return{rejected:false}}catch(error){await Promise.resolve();return{rejected:error.name==='AbortError',bridgeUnchanged:globalThis.wireBoardPokepathNative===before}}finally{WebAssembly.instantiate=instantiate}})()");
    assert.deepEqual(lateCancellation, { rejected: true, bridgeUnchanged: true });
    report.checks.push({ name: "cancel-before-go-start-does-not-replace-runtime", ...lateCancellation });
    const cancelled = await evaluate("(()=>{const controller=new AbortController();controller.abort();return window.loadNative({baseURL:'/good/',catalogBaseURL:'/facts/',route:0,signal:controller.signal}).then(()=>false,error=>error.name==='AbortError')})()");
    assert.equal(cancelled, true);
    report.checks.push({ name: "cache-and-abort-boundaries", passed: true, cachedProgress: await evaluate("window.cachedProgress") });
    assert.deepEqual((await browser("errors")).errors, []);
    await browser("screenshot", join(output, "ready.png"));
    report.requests = requests;
    report.passed = true;
  } catch (error) {
    report.error = error.stack ?? String(error);
    throw error;
  } finally {
    try { await browser("close"); } catch {}
    await new Promise(resolve => server.close(resolve));
    await rm(temporary, { recursive: true, force: true });
    await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  }
});
