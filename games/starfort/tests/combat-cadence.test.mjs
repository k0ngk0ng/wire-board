// Failure scenarios specified before implementation:
// - one-second snapshots collapse different tower intervals into one volley;
// - the next batch restarts cadence or commits its first hit before flight ends;
// - shield, HP, splash victims, healing and death use a different clock;
// - an enemy spawned and killed in one batch never appears before its impact;
// - shot pre-state restores HP already removed by a prior shot;
// - final snapshot reconciliation resurrects a dead enemy or overwrites healing;
// - duplicate snapshots, truncated rings, reconnect, pause/hidden reset replay history;
// - late/low-rate draws replay expired shots; reduced motion leaves old effects.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  CombatPlayback,
  FLIGHT_MS,
  PLAYBACK_BUFFER_MS,
  activeEffects,
} from "../web/combat-timeline.ts";
import { freshEvents } from "../web/presentation.ts";
const enemy = (id, hp, shield = 0) => ({
  id,
  kind: "grunt",
  x: 100,
  y: 0,
  progress: 100,
  hp,
  maxHP: 100,
  shield,
  health: hp + shield,
  maxHealth: 120,
  speed: 30,
  slowUntil: 0,
});
const snap = (elapsedMS, enemies, events = []) => ({
  elapsedMS,
  tick: elapsedMS / 200,
  enemies,
  baseHP: 100,
  events,
  phase: "battle",
});
const event = (id, type, atMS, target, extra = {}) => ({
  id,
  type,
  atMS,
  tick: Math.ceil(atMS / 200),
  x: 0,
  y: 0,
  targetId: target?.id,
  target,
  ...extra,
});
const hp = (play, at, id = 1) =>
  play.read(at).enemies.find((e) => e.id === id)?.hp;
test("one batch preserves 577ms and 340ms attack intervals, without a receipt-time volley", () => {
  const initial = snap(0, [enemy(1, 100)]),
    p = new CombatPlayback(initial, 0);
  const events = [
    event(1, "shot", 200, enemy(1, 100), { towerId: 1 }),
    event(2, "shot", 777, enemy(1, 90), { towerId: 1 }),
    event(3, "shot", 200, enemy(1, 100), { towerId: 2 }),
    event(4, "shot", 540, enemy(1, 95), { towerId: 2 }),
    event(5, "shot", 880, enemy(1, 85), { towerId: 2 }),
  ];
  const effects = p.push(snap(1000, [enemy(1, 80)], events), events, 1000);
  assert.deepEqual(
    effects.filter((e) => e.towerId === 1).map((e) => e.born),
    [1200, 1777],
  );
  assert.deepEqual(
    effects.filter((e) => e.towerId === 2).map((e) => e.born),
    [1200, 1540, 1880],
  );
  const next = [event(6, "shot", 1354, enemy(1, 80), { towerId: 1 })];
  assert.equal(
    p.push(snap(2000, [enemy(1, 70)], next), next, 1920)[0].born,
    2354,
  );
});
test("shield and HP change only on the same clock as impact, not shot or receipt", () => {
  const initial = snap(0, [enemy(1, 100, 20)]),
    p = new CombatPlayback(initial, 0);
  const events = [
    event(1, "shot", 200, enemy(1, 100, 20)),
    event(2, "hit", 200, enemy(1, 95)),
  ];
  const scheduled = p.push(snap(1000, [enemy(1, 95)], events), events, 1000);
  const shot = scheduled.find((e) => e.type === "shot"),
    hit = scheduled.find((e) => e.type === "hit");
  assert.equal(hit.born - shot.born, FLIGHT_MS);
  assert.equal(p.read(hit.born - 1).enemies[0].health, 120);
  assert.equal(p.read(hit.born).enemies[0].health, 95);
});
test("splash, healing, later shots and death apply authoritative post-states in order", () => {
  const p = new CombatPlayback(snap(0, [enemy(1, 100), enemy(2, 100)]), 0);
  const events = [
    event(1, "shot", 200, enemy(1, 100)),
    event(2, "hit", 200, enemy(1, 60)),
    event(3, "hit", 200, enemy(2, 70)),
    event(4, "heal", 400, enemy(1, 80)),
    event(5, "shot", 500, enemy(1, 80)),
    event(6, "hit", 500, enemy(1, 20)),
    event(7, "shot", 600, enemy(1, 20)),
    event(8, "hit", 600, enemy(1, 0)),
    event(9, "death", 600, enemy(1, 0)),
  ];
  p.push(snap(1000, [enemy(2, 70)], events), events, 1000);
  assert.equal(hp(p, 1380), 60);
  assert.equal(hp(p, 1380, 2), 70);
  assert.equal(hp(p, 1500), 60);
  assert.equal(hp(p, 1580), 80);
  assert.equal(hp(p, 1600), 80);
  assert.equal(hp(p, 1680), 20);
  assert.equal(hp(p, 1780), undefined);
  assert.equal(hp(p, 2200), undefined);
});
test("short-lived spawn and every splash victim remain visible until their own impact", () => {
  const p = new CombatPlayback(snap(0, []), 0);
  const events = [
    event(1, "spawn", 200, enemy(1, 100)),
    event(2, "spawn", 200, enemy(2, 100)),
    event(3, "shot", 300, enemy(1, 100)),
    event(4, "hit", 300, enemy(1, 0)),
    event(5, "death", 300, enemy(1, 0)),
    event(6, "hit", 300, enemy(2, 0)),
    event(7, "death", 300, enemy(2, 0)),
  ];
  p.push(snap(1000, [], events), events, 1000);
  assert.equal(p.read(1199).enemies.length, 0);
  assert.equal(p.read(1200).enemies.length, 2);
  assert.equal(p.read(1479).enemies.length, 2);
  assert.equal(p.read(1480).enemies.length, 0);
});
test("adjacent snapshots preserve in-flight HP and do not let reconciliation overwrite later hits", () => {
  const p = new CombatPlayback(snap(0, [enemy(1, 100)]), 0);
  const first = [
    event(1, "shot", 1000, enemy(1, 100)),
    event(2, "hit", 1000, enemy(1, 80)),
  ];
  p.push(snap(1000, [enemy(1, 80)], first), first, 1000);
  const second = [
    event(3, "shot", 1100, enemy(1, 80)),
    event(4, "hit", 1100, enemy(1, 60)),
  ];
  p.push(snap(2000, [enemy(1, 60)], second), second, 2000);
  assert.equal(hp(p, 2100), 100);
  assert.equal(hp(p, 2179), 100);
  assert.equal(hp(p, 2180), 80);
  assert.equal(hp(p, 2280), 60);
});
test("reset drops pending health changes; late frames catch up without replaying expired projectiles", () => {
  const p = new CombatPlayback(snap(0, [enemy(1, 100)]), 0);
  const events = [
    event(1, "shot", 200, enemy(1, 100)),
    event(2, "hit", 200, enemy(1, 40)),
  ];
  const effects = p.push(snap(1000, [enemy(1, 40)], events), events, 1000);
  assert.equal(
    activeEffects(effects, 1200, false).filter((e) => e.type === "shot").length,
    1,
  );
  assert.equal(
    activeEffects(effects, 1200 + FLIGHT_MS, false).filter(
      (e) => e.type === "shot",
    ).length,
    0,
  );
  assert.deepEqual(activeEffects(effects, 5000, false), []);
  assert.deepEqual(activeEffects(effects, 1200, true), []);
  assert.equal(hp(p, 5000), 40);
  p.reset(snap(4000, [enemy(1, 90)]), 5000);
  assert.equal(hp(p, 99999), 90);
  assert.equal(PLAYBACK_BUFFER_MS, 1000);
});
test("large complete event batches retain every event and gaps demand snapshot reset", () => {
  let [cursor] = freshEvents(
    null,
    "room",
    snap(0, [], [event(1, "shot", 0, enemy(1, 100))]),
    0,
  );
  const events = Array.from({ length: 80 }, (_, i) =>
    event(i + 2, "hit", 200, enemy(1, 99)),
  );
  let batch;
  [cursor, batch] = freshEvents(cursor, "room", snap(1000, [], events), 1000);
  assert.equal(batch.length, 80);
  assert.equal(cursor.reset, false);
  const [gap, skipped] = freshEvents(
    cursor,
    "room",
    snap(2000, [], [event(90, "shot", 1200, enemy(1, 90))]),
    2000,
  );
  assert.equal(gap.reset, true);
  assert.deepEqual(skipped, []);
});

test("a dying enemy across packet boundaries keeps its previous endpoint until impact", async () => {
  const { combatMotion } = await import("../web/presentation.ts");
  const start = { ...enemy(1, 100), x: 0, progress: 0 };
  const end = { ...enemy(1, 0), x: 100, progress: 100 };
  // The authoritative previous snapshot has already removed this enemy;
  // playback retains it for the last projectile's remaining 180 ms.
  const motion = combatMotion([], new Map([[1, end]]), [enemy(1, 100)], [], []);
  assert.equal(motion.previous.get(1).progress, 100);
  assert.equal(motion.destinations.get(1).progress, 100);
  // Only a genuinely new spawn gets a new start position.
  const born = combatMotion(
    [],
    new Map(),
    [],
    [end],
    [event(1, "spawn", 1200, start)],
  );
  assert.equal(born.previous.get(1).progress, 0);
});

test("audio follows drawn event times once and discards cues skipped by a stalled frame", async () => {
  const { dueSoundEvents } = await import("../web/combat-timeline.ts");
  const events = [
    { ...event(1, "shot", 200, enemy(1, 100)), born: 1200 },
    { ...event(2, "hit", 200, enemy(1, 60)), born: 1380 },
    { ...event(3, "shot", 776, enemy(1, 60)), born: 1776 },
  ];
  let [due, pending] = dueSoundEvents(events, 1210);
  assert.deepEqual(
    due.map((e) => e.id),
    [1],
  );
  assert.equal(pending.length, 2);
  [due, pending] = dueSoundEvents(pending, 1501);
  assert.deepEqual(due, []);
  assert.equal(pending.length, 1);
  [due, pending] = dueSoundEvents(pending, 1780);
  assert.deepEqual(
    due.map((e) => e.id),
    [3],
  );
  assert.deepEqual(pending, []);
});

test("summon appears with its cue before a same-time shot can hit it", () => {
  const p = new CombatPlayback(snap(0, []), 0);
  const events = [
    event(1, "summon", 200, enemy(1, 100)),
    event(2, "shot", 200, enemy(1, 100)),
    event(3, "hit", 200, enemy(1, 50)),
  ];
  const effects = p.push(snap(1000, [enemy(1, 50)], events), events, 1000);
  assert.equal(effects[0].born, effects[1].born);
  assert.equal(hp(p, 1200), 100);
  assert.equal(hp(p, 1380), 50);
});

test("early packets keep the previous movement segment until the next logical boundary", async () => {
  const { CombatTimeline } = await import("../web/combat-timeline.ts");
  const first = { start: 1000, end: 2000, from: 0, to: 100 };
  const second = { start: 2000, end: 3000, from: 100, to: 200 };
  const movement = new CombatTimeline(first);
  movement.pushAt(second, 2000);
  assert.equal(movement.read(1920), first);
  assert.equal(movement.read(1999), first);
  assert.equal(movement.read(2000), second);
});

test("base HUD publishes changed health on impact, without per-frame updates or stale reset callbacks", () => {
  const p = new CombatPlayback(snap(0, [enemy(1, 100)]), 0),
    reports = [];
  const stop = p.subscribe((state) => reports.push(state));
  const leak = event(1, "base_hit", 200, enemy(1, 100), { amount: 4 });
  p.push({ ...snap(1000, [], [leak]), baseHP: 96 }, [leak], 1000);
  p.read(1200);
  p.read(1379);
  assert.deepEqual(reports, [{ baseHP: 100 }]);
  p.read(1380);
  p.read(1390);
  p.read(2180);
  assert.deepEqual(reports, [{ baseHP: 100 }, { baseHP: 96 }]);
  p.reset({ ...snap(4000, []), baseHP: 80 }, 5000);
  assert.deepEqual(reports.at(-1), { baseHP: 80 });
  stop();
  p.reset({ ...snap(5000, []), baseHP: 70 }, 6000);
  assert.equal(reports.length, 3);
});

// New compact AoE/repair protocol failures are specified before the consumer:
// splash drops secondary victims, damage is applied before flight, snapshots
// mutate in place, a crater appears before impact or workers stay trapped.
test('compact splash applies every authoritative victim only on impact',()=>{
 const initial=snap(0,[enemy(1,100,20),enemy(2,100,10)]),p=new CombatPlayback(initial,0);
 const events=[event(1,'shot',200,enemy(1,100,20)),event(2,'hit',200,enemy(1,90),{damage:[{id:2,amount:25,x:110,y:0}]})];
 p.push(snap(1000,[enemy(1,90),enemy(2,85)]),events,1000);
 assert.equal(hp(p,1379,2),100);assert.equal(hp(p,1380,1),90);assert.equal(hp(p,1380,2),85);
 assert.equal(p.read(1380).enemies[1].shield,0);assert.equal(initial.enemies[1].hp,100);
});
test('craters and repair release share the projectile impact timeline',()=>{
 const initial={...snap(0,[enemy(1,100),enemy(2,100)]),craters:[]},p=new CombatPlayback(initial,0);
 const crater={id:1,towerId:2,owner:0,route:'main',progress:100,x:100,y:0,radius:28,total:6,remaining:6,workers:[],exemptEnemy:1,createdMS:200};
 const events=[event(1,'crater',200,undefined,{crater}),event(2,'repair_start',400,{...enemy(2,100),repairing:1,repairSinceMS:400},{craterId:1}),event(3,'crater_repair',400,undefined,{crater:{...crater,remaining:5.8,workers:[2]}}),event(4,'crater_filled',600,undefined,{crater:{...crater,remaining:0,workers:[2],filledMS:600}})];
 p.push({...snap(1000,[enemy(1,100),enemy(2,100)]),craters:[{...crater,remaining:0,workers:[2],filledMS:600}]},events,1000);
 assert.equal(p.read(1379).craters.length,0);assert.equal(p.read(1380).craters[0].remaining,6);
 assert.equal(p.read(1580).enemies[1].repairing,1);assert.equal(p.read(1580).craters[0].remaining,5.8);
 assert.equal(p.read(1780).enemies[1].repairing,0);assert.equal(p.read(1780).craters[0].remaining,0);
});

test('first advancing packet anchors after loading delay without dropping the first fast shots', () => {
  const p = new CombatPlayback(snap(0,[enemy(1,100)]), 20);
  p.push(snap(0,[enemy(1,100)]),[],270);
  const shots=[event(1,'shot',200,enemy(1,100)),event(2,'shot',524,enemy(1,90)),event(3,'shot',848,enemy(1,80))];
  const effects=p.push(snap(1000,[enemy(1,70)]),shots,1710);
  assert.deepEqual(effects.map(e=>e.born),[1910,2234,2558]);
  assert.equal(p.push(snap(2000,[enemy(1,60)]),[event(4,'shot',1172,enemy(1,70))],2711)[0].born,2882);
});

test('one compact frost ring applies all slow targets at impact',()=>{
 const p=new CombatPlayback(snap(0,[enemy(1,100),enemy(2,100)]),0);
 const e=event(1,'frost_ring',200,undefined,{slowTargets:[1,2],slowUntil:16});
 p.push(snap(1000,[enemy(1,100),enemy(2,100)]),[e],1000);
 assert.equal(p.read(1379).enemies[1].slowUntil,0);
 assert.deepEqual(p.read(1380).enemies.map(e=>e.slowUntil),[16,16]);
});

test('ice wall and permanent frost appear on the authoritative effect clock',()=>{
 const initial=snap(0,[enemy(1,100)]),p=new CombatPlayback(initial,0);
 const wall={id:1,x:100,y:0,progress:100,createdMS:200,expiresMS:6200};
 const frozen={...enemy(1,100),frostbound:true,level:4};
 const events=[event(1,'ice_wall',200,undefined,{iceWall:wall}),event(2,'frostbound',600,frozen)];
 p.push({...snap(1000,[frozen]),iceWalls:[wall]},events,1000);
 assert.equal(p.read(1379).iceWalls?.length??0,0);
 assert.equal(p.read(1380).iceWalls.length,1);
 assert.equal(p.read(1599).enemies[0].frostbound,undefined);
 assert.equal(p.read(1600).enemies[0].frostbound,true);
 p.push({...snap(7000,[frozen]),iceWalls:[]},[],7000);
 assert.equal(p.read(8180).iceWalls.length,0);
 assert.equal(p.read(8180).enemies[0].frostbound,true);
});

test('wall contact changes status immediately without revealing a simultaneous hit early',()=>{
 const p=new CombatPlayback(snap(0,[enemy(1,100)]),0);
 const damaged=enemy(1,90),frozen={...damaged,frostbound:true};
 const events=[event(1,'shot',600,enemy(1,100)),event(2,'hit',600,damaged),event(3,'frostbound',600,frozen)];
 p.push(snap(1000,[frozen]),events,1000);
 assert.equal(p.read(1600).enemies[0].frostbound,true);assert.equal(hp(p,1779),100);
 assert.equal(hp(p,1780),90);assert.equal(p.read(1780).enemies[0].frostbound,true);
});

test('a fast enemy cannot show frost before the creating projectile reaches the wall',()=>{
 const p=new CombatPlayback(snap(0,[enemy(1,100)]),0);
 const wall={id:1,x:100,y:0,createdMS:200,expiresMS:6200};
 const frozen={...enemy(1,90),frostbound:true};
 const events=[event(1,'shot',200,enemy(1,100)),event(2,'hit',200,enemy(1,90)),event(3,'ice_wall',200,undefined,{iceWall:wall}),event(4,'frostbound',200,frozen,{iceWallId:1})];
 p.push({...snap(1000,[frozen]),iceWalls:[wall]},events,1000);
 assert.equal(p.read(1379).enemies[0].frostbound,undefined);assert.equal(hp(p,1379),100);
 assert.equal(p.read(1380).enemies[0].frostbound,true);assert.equal(hp(p,1380),90);assert.equal(p.read(1380).iceWalls.length,1);
});

test('compact permanent-frost event needs no copied enemy snapshot',()=>{
 const p=new CombatPlayback(snap(0,[enemy(1,100)]),0);
 const frozen={...enemy(1,100),frostbound:true};
 const events=[event(1,'frostbound',600,undefined,{targetId:1,iceWallId:1})];
 p.push(snap(1000,[frozen]),events,1000);
 assert.equal(p.read(1600).enemies[0].frostbound,true);assert.equal(hp(p,1600),100);
});

// Before late-wave compaction: grouped victims lose shield/HP, thin vitals are
// ignored or visible before impact, and a later hit clears permanent frost.
test('grouped splash and compact primary vitals remain on the impact clock',()=>{
 const initial=snap(0,[enemy(1,100,20),enemy(2,100,10),enemy(3,100)]),p=new CombatPlayback(initial,0);
 const shot=event(1,'shot',200,enemy(1,100,20));
 const hit={...event(2,'hit',200,enemy(1,90)),target:undefined,targetId:1,vitals:{hp:90,shield:0,slowUntil:0},damageGroups:[{amount:25,targets:[2,3]}]};
 p.push(snap(1000,[enemy(1,90),enemy(2,85),enemy(3,75)]),[shot,hit],1000);
 assert.equal(hp(p,1379,1),100);assert.equal(hp(p,1379,2),100);
 assert.equal(hp(p,1380,1),90);assert.equal(hp(p,1380,2),85);assert.equal(hp(p,1380,3),75);
 assert.equal(initial.enemies[0].hp,100);assert.equal(p.read(1380).enemies[1].shield,0);
});
test('compact vitals do not erase an earlier permanent frost contact',()=>{
 const initial=snap(0,[{...enemy(1,100),frostbound:true}]),p=new CombatPlayback(initial,0);
 const hit={...event(1,'hit',200,enemy(1,90)),target:undefined,targetId:1,vitals:{hp:90,shield:0,slowUntil:0}};
 p.push(snap(1000,[{...enemy(1,90),frostbound:true}]),[hit],1000);
 assert.equal(p.read(1380).enemies[0].frostbound,true);assert.equal(hp(p,1380,1),90);
});
// Late waves use large entity IDs; compact target offsets must never hit a
// different enemy or change the impact time.
test('group target bases preserve late-wave enemy identities',()=>{
 const p=new CombatPlayback(snap(0,[enemy(2201,100),enemy(2202,100,10)]),0);
 const hit={...event(1,'hit',200,enemy(2201,90)),damageGroups:[{amount:25,base:2200,targets:[2]}]};
 p.push(snap(1000,[enemy(2201,90),enemy(2202,85)]),[hit],1000);
 assert.equal(hp(p,1379,2202),100);assert.equal(hp(p,1380,2202),85);
});
