import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { extname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const run = promisify(execFile);
const sha = bytes => createHash("sha256").update(bytes).digest("hex");
const id = randomUUID().slice(0, 8);
const requested = new Set(process.argv.slice(2));
assert.ok([...requested].every(value => ["--ideal-60hz", "--settlement", "--starter-defaults", "--warm-enemies", "--item-combinations", "--critical-combinations", "--coupled-jitter"].includes(value) || /^--speed=(?:0\.4|0\.8|1|1\.2|1\.7|2|2\.5|5)$/.test(value)), "Unknown comparison option");
const speed = Number([...requested].find(value => value.startsWith("--speed="))?.slice(8) ?? 1);
const ideal = requested.has("--ideal-60hz");
const settlementOnly = requested.has("--settlement");
const defaults = requested.has("--starter-defaults");
const warmEnemies = requested.has("--warm-enemies");
const itemCombinations = requested.has("--item-combinations");
const criticalCombinations = requested.has("--critical-combinations");
const coupledJitter = requested.has("--coupled-jitter");
assert.ok(!criticalCombinations || itemCombinations, "Critical combinations require the item matrix");
assert.ok(!coupledJitter || criticalCombinations && speed <= 1, "Coupled jitter requires controlled critical inputs and one rule substep per frame");
const output = join(root, ".local/pokepath/verification", `first-wave-reference${defaults ? "-defaults" : ""}${ideal ? "-ideal" : ""}${warmEnemies ? "-warm" : ""}${itemCombinations ? "-items" : ""}${criticalCombinations ? "-critical" : ""}${coupledJitter ? "-coupled" : ""}${speed !== 1 ? `-speed-${speed}` : ""}`, ...(itemCombinations ? [`run-${id}`] : []));
const owned = join(output, `fixture-${id}`);
const sourceRoot = join(root, ".local/pokepath/research/source");
const research = join(root, ".local/pokepath/research");
const studyCSS = join(root, ".local/pokepath/rejected-embedding/assets/src/css");
const media = join(root, "games/pokepath/assets/public/pokepath/v1");
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: join(owned, "profile"), AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const command = ["node games/pokepath/tests/first-wave-reference.mjs", ...requested].join(" ");
const inputs = [{ starter: "charmander", deploy: false, speed }, ...["charmander", "treecko", "froaki"].map(starter => ({ starter, deploy: true, target: defaults ? undefined : "first", speed }))];
if (itemCombinations) for (const starter of ["charmander", "treecko", "froaki"]) for (const item of ["protein", "carbos", "zinc", "eviolite", "fullIncense", { charmander: "koffingJelly", treecko: "fertiliser", froaki: "loadedDice" }[starter]]) inputs.push({ starter, deploy: true, target: defaults ? undefined : "first", speed, item });
if (criticalCombinations) {
  for (const starter of ["charmander", "treecko", "froaki"]) for (const item of ["laggingTail", "quickClaw", "zoomLens"]) inputs.push({ starter, deploy: true, target: item === "quickClaw" ? undefined : defaults ? undefined : "first", speed, item });
  inputs.push({ starter: "froaki", deploy: true, target: "first", speed, item: "metronome" });
  inputs.push(...inputs.map(input => ({ ...input, forceCritical: true })));
  for (const input of inputs) input.controlCritical = true;
}
for (const input of inputs) input.coupledJitter = coupledJitter;
const report = { command, inputs, publishedReference: false, scope: "Original v1.7.3 running only inside the local reference browser; independent Go runs separately with fixed inputs, equal speed and the route 1-1 start", cases: [], passed: false };
const types = { ".mjs": "text/javascript", ".js": "text/javascript", ".css": "text/css", ".png": "image/png", ".json": "application/json", ".ttf": "font/ttf", ".svg": "image/svg+xml", ".ico": "image/x-icon" };
const program = `package main
import("encoding/json";"math";"os";"sort";"github.com/k0ngk0ng/wire-board/games/pokepath/engine")
type Input struct {Starter string;Deploy,ForceCritical,ControlCritical,CoupledJitter bool;Target,Item string;Speed float64}
type Unit struct {Index int \x60json:"index"\x60;X float64 \x60json:"x"\x60;Y float64 \x60json:"y"\x60;HP float64 \x60json:"hp"\x60;Entered bool \x60json:"entered"\x60;Dead bool \x60json:"dead"\x60}
func must(err error){if err!=nil{panic(err)}}
func main(){
 var inputs []Input;must(json.NewDecoder(os.Stdin).Decode(&inputs));results:=[]any{}
 for _,input:=range inputs {
  s,err:=engine.New(engine.Options{Starter:input.Starter,Speed:input.Speed},1,99127,1730000000000);must(err)
  if input.Item!=""{for index,key:=range s.Progression.ShopStock{if key==input.Item{s.Progression.ShopStock[index]=""}};s.Inventory=append(s.Inventory,engine.OwnedItem{ID:input.Item});must(engine.Apply(s,0,engine.Action{Type:"equip",Epoch:s.Epoch,PokemonID:s.Team[0].ID,ItemID:input.Item}));if s.Team[0].Item!=input.Item{panic("independent did not retain fixture item")}}
  if input.Deploy{if input.Target!="" {must(engine.Apply(s,0,engine.Action{Type:"target",Epoch:s.Epoch,PokemonID:s.Team[0].ID,Mode:input.Target}))};must(engine.Apply(s,0,engine.Action{Type:"deploy",Epoch:s.Epoch,PokemonID:s.Team[0].ID,TileID:1}))}
  must(engine.Apply(s,0,engine.Action{Type:"wave",Epoch:s.Epoch}));ordinals:=map[int]int{}
  if input.ControlCritical{if input.ForceCritical{s.Team[0].Stats.CriticalPct=100}else{s.Team[0].Stats.CriticalPct=0}}
  for index,enemy:=range s.Enemies{ordinals[enemy.ID]=index}
  trace,attacks,account,shots:=[]any{},[]any{},[]any{},[]any{};seen:=s.NextEventID-1;jitter:=[][2]float64{};observedRicochets:=map[int]bool{}
  capture:=func(){units:=[]Unit{};for _,enemy:=range s.Enemies{units=append(units,Unit{ordinals[enemy.ID],enemy.X,enemy.Y,max(0,enemy.HP),enemy.Entered,enemy.Dead})};trace=append(trace,map[string]any{"time":s.Clock,"units":units})}
  capture()
  for frame:=int64(1);s.Clock<150000&&s.WaveActive;frame++ {
   next:=(frame*50+2)/3;before:=s.Gold;randomBefore:=s.Random;eventBefore:=s.NextEventID-1;must(engine.Advance(s,next-s.Clock));ordinary:=int64(0)
   if input.CoupledJitter {
    probe:=engine.State{Random:randomBefore};draws:=[]float64{}
    for probe.Random!=s.Random&&len(draws)<1024{draws=append(draws,engine.NextRandom(&probe))};if probe.Random!=s.Random{panic("unbounded controlled random frame")}
    born:=[]engine.Projectile{};for _,projectile:=range s.Projectiles{if projectile.RicochetHits>0&&!observedRicochets[projectile.ID]{observedRicochets[projectile.ID]=true;born=append(born,projectile)}};sort.Slice(born,func(i,j int)bool{return born[i].ID<born[j].ID});used:=map[int]bool{}
    for _,projectile:=range born{
     if projectile.LifeMS!=0{panic("ricochet birth moved before controlled observation")};found:=-1
     for _,event:=range s.Events{if event.ID<=eventBefore||event.Type!="hit"||event.SourceKind!="physical"{continue};for index:=0;index+1<len(draws);index++{if used[index]||used[index+1]{continue};x:=event.X+(draws[index]-.5)*6+projectile.Sprite.FrameWidth/2;y:=event.Y+(draws[index+1]-.5)*6+projectile.Sprite.FrameHeight/2;if math.Abs(projectile.X-x)<=.000001&&math.Abs(projectile.Y-y)<=.000001{found=index;break}};if found>=0{break}}
     if found<0{panic("ricochet birth does not match consumed random draws")};used[found],used[found+1]=true,true;jitter=append(jitter,[2]float64{draws[found],draws[found+1]})
    }
   }
   for _,event:=range s.Events {
    if event.ID<=seen{continue};seen=event.ID
    switch event.Type{
    case "fire":
     positions:=[]Unit{};for _,enemy:=range s.Enemies{positions=append(positions,Unit{ordinals[enemy.ID],enemy.X,enemy.Y,max(0,enemy.HP),enemy.Entered,enemy.Dead})}
     shots=append(shots,map[string]any{"time":float64(event.At)/input.Speed,"enemy":ordinals[event.Target],"x":s.Towers[0].X,"y":s.Towers[0].Y,"origin":engine.Point{X:event.X,Y:event.Y},"range":s.Team[0].Stats.Range,"units":positions})
    case "hit":if event.HPDamage>0{kind:="direct";if event.SourceKind=="burn"||event.SourceKind=="poison"{kind=event.SourceKind};attacks=append(attacks,map[string]any{"time":float64(event.At)/input.Speed,"enemy":ordinals[event.Target],"kind":kind,"critical":event.Critical,"amount":event.HPDamage,"x":event.X,"y":event.Y})}
    case "defeatEnemy":ordinary+=int64(event.Amount);account=append(account,map[string]any{"time":event.At,"kind":"killGold","amount":event.Amount})
    case "escape":account=append(account,map[string]any{"time":event.At,"kind":"escape","amount":event.Amount})
    }
   }
   if bonus:=s.Gold-before-ordinary;bonus>0{account=append(account,map[string]any{"time":s.Tick,"kind":"bonus","amount":bonus})}
   if frame%12==0||!s.WaveActive{capture()}
  }
  if s.WaveActive{panic("independent first wave did not finish")}
  results=append(results,map[string]any{"starter":input.Starter,"deploy":input.Deploy,"item":input.Item,"forceCritical":input.ForceCritical,"jitter":jitter,"targetMode":s.Team[0].Target,"rulesVersion":engine.RulesVersion(),"attacks":attacks,"shots":shots,"account":account,"trace":trace,"completedAt":s.Clock,"outcome":map[string]any{"gold":s.Gold,"health":s.Health,"stars":s.Stars,"wave":s.Wave,"kills":s.TotalKills}})
 }
 must(json.NewEncoder(os.Stdout).Encode(results))
}`;

await mkdir(owned, { recursive: true });
const nativeFile = join(owned, "trace.go");
await writeFile(nativeFile, program);
const fixture = await readFile(new URL("first-wave-reference-fixture.mjs", import.meta.url));
report.fixtureSHA256 = sha(fixture);
const html = (await readFile(join(research, "game.html"), "utf8")).replace(/<script\b[^>]*src=["'][^"']*["'][^>]*><\/script>/g, "").replace("</body>", '<script type="module" src="/fixture.mjs"></script></body>');
report.referenceSources = Object.fromEntries(await Promise.all(["game/Game.js", "game/Main.js", "game/core/Area.js", "game/core/Player.js", "game/component/Tower.js", "game/component/Enemy.js", "game/component/Projectile.js"].map(async name => [name, sha(await readFile(join(sourceRoot, "src/js", name)))])));
const projectileLines = (await readFile(join(sourceRoot, "src/js/game/component/Projectile.js"), "utf8")).split("\n");
const jitterSites = { x: projectileLines.findIndex(line => line.includes("const sx = this.enemy.center.x + (Math.random()")) + 1, y: projectileLines.findIndex(line => line.includes("const sy = this.enemy.center.y + (Math.random()")) + 1 };
assert.ok(jitterSites.x > 0 && jitterSites.y === jitterSites.x + 1, "Unknown original jitter call sites");
report.controlledJitter = coupledJitter ? { method: "Consumed native xorshift draws verified against newborn offsets, then replayed at the exact original jitter call sites", jitterSites } : undefined;
const server = createServer(async (request, response) => {
  const path = new URL(request.url, "http://localhost").pathname;
  response.setHeader("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; worker-src 'none'; object-src 'none'; script-src 'self'");
  if (path === "/") { response.writeHead(200, { "Content-Type": "text/html; charset=utf-8" }); response.end(html); return; }
  if (path === "/fixture.mjs") { response.writeHead(200, { "Content-Type": "text/javascript" }); response.end(fixture); return; }
  if (!/^\/(?:styles\.css|src\/(?:assets|js|css)\/[a-zA-Z0-9_/.-]+)$/.test(path) || path.includes("..")) { response.writeHead(404); response.end(); return; }
  const file = path === "/styles.css" ? join(research, "styles.css") : path.startsWith("/src/js/") ? join(sourceRoot, path) : path.startsWith("/src/css/") ? join(studyCSS, path.slice(9)) : join(media, path);
  try {
    const bytes = await readFile(file);
    response.writeHead(200, { "Content-Type": types[extname(file)] ?? "application/octet-stream" });
    response.end(bytes);
  } catch { response.writeHead(404); response.end(); }
});
let session;
const browser = async (...args) => {
  const stdout = await run("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 50000, maxBuffer: 16 << 20 });
  const value = JSON.parse(stdout.stdout);
  assert.equal(value.success, true, JSON.stringify(value));
  return value.data;
};
try {
  const inputFile = join(owned, "input.json");
  await writeFile(inputFile, JSON.stringify(inputs));
  const runner = await run("sh", ["-c", 'go run "$1" < "$2"', "trace", nativeFile, inputFile], { cwd: root, env: environment, timeout: 60000, maxBuffer: 12 << 20 });
  const actual = JSON.parse(runner.stdout);
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  session = (await run("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", `wave-ref-${id}`], { cwd: root, env: environment })).stdout.trim();
  await browser("open", "about:blank");
  await browser("set", "viewport", "1238", "674");
  for (const input of inputs) {
    const name = `${input.starter}-${input.deploy ? "deployed" : "undefended"}${input.item ? `-${input.item}` : ""}${input.forceCritical ? "-critical" : ""}`;
    const trace = actual.find(value => value.starter === input.starter && value.deploy === input.deploy && value.item === (input.item ?? "") && value.forceCritical === Boolean(input.forceCritical));
    const query = new URLSearchParams({ starter: input.starter, deploy: input.deploy ? "1" : "0", ideal: ideal ? "1" : "0", warm: warmEnemies ? "1" : "0", target: input.target ?? "", speed: String(input.speed), item: input.item ?? "", critical: input.controlCritical ? input.forceCritical ? "1" : "0" : "" });
    if (coupledJitter) { query.set("jitter", JSON.stringify(trace.jitter)); query.set("jitterSites", JSON.stringify(jitterSites)); }
    await browser("open", `http://127.0.0.1:${server.address().port}/?${query}`);
    await browser("wait", "--fn", "Boolean(window.waveReference)");
    const expected = (await browser("eval", "window.waveReference")).result;
    const image = join(output, `${name}-${settlementOnly ? "settlement" : "reference"}.png`);
    await browser("screenshot", image);
    report.cases.push({ name, original: expected, native: trace, screenshot: { path: image, sha256: sha(await readFile(image)) } });
  }
  const properties = (records, fields) => records.map(record => fields.map(key => record[key]));
  const firstDifference = (original, native, fields) => {
    const left = properties(original, fields);
    const right = properties(native, fields);
    for (let i = 0; i < Math.max(left.length, right.length); i++) {
      if (JSON.stringify(left[i]) !== JSON.stringify(right[i])) return { index: i, original: original[i] ?? null, native: native[i] ?? null };
    }
    return null;
  };
  const largestTimeDifference = (original, native) => original.length === native.length ? original.reduce((largest, event, index) => Math.max(largest, Math.abs(event.time - native[index].time)), 0) : null;
  for (const scenario of report.cases) {
    const { original, native } = scenario;
    if (criticalCombinations) for (const trace of [original, native]) assert.ok(trace.attacks.filter(attack => attack.kind === "direct").every(attack => attack.critical === trace.forceCritical), "Controlled critical-rate fixture did not retain its requested rate");
    const hitFields = criticalCombinations ? ["enemy", "kind", "critical", "amount"] : ["enemy", "kind", "amount"];
    scenario.checks = {
      outcome: JSON.stringify(Object.entries(native.outcome).sort()) === JSON.stringify(Object.entries(original.outcome).sort()),
      targetMode: { original: original.targetMode, native: native.targetMode, matches: native.targetMode === original.targetMode },
      hitDifference: firstDifference(original.attacks, native.attacks, hitFields),
      shotDifference: firstDifference(original.shots, native.shots, ["enemy"]),
      rewardDifference: firstDifference(original.account, native.account, ["kind", "amount"]),
      completionDriftMS: native.completedAt - original.completedAt,
      largestShotDriftMS: largestTimeDifference(original.shots, native.shots),
      largestHitDriftMS: largestTimeDifference(original.attacks, native.attacks),
    };
    if (native.deploy) {
      const originalOrigin = original.shots[0]?.origin;
      const nativeOrigin = native.shots[0]?.origin;
      scenario.checks.firstLaunchOrigin = { original: originalOrigin ?? null, native: nativeOrigin ?? null, matches: originalOrigin && nativeOrigin && Math.hypot(originalOrigin.x - nativeOrigin.x, originalOrigin.y - nativeOrigin.y) <= 0.000001 };
    }
    if (native.starter === "froaki" && !native.item && !native.forceCritical) {
      const chain = attacks => properties(attacks.slice(0, 4), ["enemy", "kind", "amount"]);
      const expected = [[0, "direct", 6], [1, "direct", 5], [0, "direct", 4], [1, "direct", 3]];
      scenario.checks.firstRicochetMatches = [original, native].every(trace => JSON.stringify(chain(trace.attacks)) === JSON.stringify(expected));
    }
  }
  report.settlementPassed = report.cases.every(({ checks }) => checks.outcome && !checks.rewardDifference && Math.abs(checks.completionDriftMS) <= 120);
  report.orderedHitsPassed = report.cases.every(({ checks }) => !checks.hitDifference);
  report.firstRicochetPassed = report.cases.find(({ checks }) => checks.firstRicochetMatches !== undefined)?.checks.firstRicochetMatches === true;
  report.firstLaunchOriginsPassed = report.cases.every(({ native, checks }) => !native.deploy || checks.firstLaunchOrigin?.matches === true);
  report.targetModesPassed = report.cases.every(({ checks }) => checks.targetMode.matches === true);
  assert.ok(report.firstLaunchOriginsPassed, "Initial projectile launch position differs; inspect cases[].checks.firstLaunchOrigin");
  assert.ok(report.targetModesPassed, "Initial target mode differs; inspect cases[].checks.targetMode");
  assert.ok(report.settlementPassed, "First-wave settlement differs; inspect cases[].checks");
  assert.ok(report.firstRicochetPassed, "First Froakie ricochet differs; inspect the ordered damage trace");
  report.launchOrderPassed = report.cases.every(({ checks }) => !checks.shotDifference);
  if (!settlementOnly) {
    assert.ok(report.orderedHitsPassed, "First-wave hit sequence differs; inspect cases[].checks");
    assert.ok(report.launchOrderPassed, "First-wave target sequence differs; inspect cases[].checks");
    if (warmEnemies && ideal && speed === 1) {
      assert.ok(report.cases.every(({ checks }) => checks.largestShotDriftMS !== null && checks.largestShotDriftMS <= 1 && Math.abs(checks.completionDriftMS) <= 1), "A preloaded 60 Hz launch or wave boundary does not fall on the same frame");
    }
  }
  report.passed = true;
} catch (error) {
  report.error = error.stack ?? String(error);
  if (session) { try { report.browserErrors = await browser("errors"); } catch {} }
  process.exitCode = 1;
} finally {
  if (session) { try { await browser("close"); } catch {} }
  if (server.listening) await new Promise(resolve => server.close(resolve));
  await rm(owned, { recursive: true, force: true });
  await writeFile(join(output, settlementOnly ? "settlement.json" : "result.json"), JSON.stringify(report, null, 2) + "\n");
  process.stdout.write(JSON.stringify({ passed: report.passed, cases: report.cases.length, error: report.error, output }) + "\n");
}
