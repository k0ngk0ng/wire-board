import test from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { promisify } from "node:util";
import { referenceDOMClass } from "./dom-oracle.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const run = promisify(execFile);
const archive = join(root, ".local/pokepath/verification/target-priority");
const baseline = (options = []) => Array.from({ length: 4 }, (_, slot) => ({ slot, hp: 1000, armor: 0, distance: [25, 85, 255, 300][slot], speed: 0, invisible: false, conditions: [], ...options[slot] }));
const inputs = [];
const add = (name, mode, enemies, extra = {}) => inputs.push({ name, mode, enemies, ...extra });
add("forward", "first", baseline());
add("backward", "last", baseline());
add("high-hp", "highHP", baseline([{ hp: 700 }, { hp: 900 }, { hp: 900 }, { hp: 800 }]), { order: [3, 2, 1, 0] });
add("low-hp", "lowHP", baseline([{ hp: 700 }, { hp: 900 }, { hp: 900 }, { hp: 800 }]), { order: [3, 2, 1, 0] });
add("highest-armor", "highArmor", baseline([{ armor: 81 }, { armor: 107 }, { armor: 107 }, { armor: 8 }]), { order: [3, 2, 1, 0] });
add("no-armor-mixed", "noArmor", baseline([{ armor: 81 }, { armor: 1 }, { armor: 0 }, { armor: 0 }]), { order: [0, 1, 3, 2] });
add("no-armor-fallback", "noArmor", baseline([{ armor: 81 }, { armor: 1 }, { armor: 70 }, { armor: 9 }]));
add("speed-includes-slow", "faster", baseline([{ speed: 90, conditions: ["slow"] }, { speed: 65 }, { speed: 115, conditions: ["stun"] }, { speed: 85 }]));
add("speed-excludes-stun", "slower", baseline([{ speed: 90, conditions: ["slow"] }, { speed: 65 }, { speed: 115, conditions: ["stun"] }, { speed: 85 }]));
for (const [status, modes] of [["poison", ["poisoned", "notPoisoned"]], ["burn", ["burned", "notBurned"]], ["stun", ["stuned", "notStuned"]], ["slow", ["slowed", "notSlowed"]], ["nightmare", ["nightmared"]]]) {
  for (const mode of modes) add(mode, mode, baseline([{}, { conditions: [status] }, {}, { conditions: [status] }]));
}
add("cursed-mixed", "cursed", baseline([{}, { conditions: ["curse"] }, {}, { conditions: ["curse"] }]));
add("curseable-mixed", "curseable", baseline([{}, { conditions: ["curse"] }, {}, { conditions: ["curse"] }]));
add("curseable-all", "curseable", baseline(Array.from({ length: 4 }, () => ({ conditions: ["curse"] }))));
add("silph-scope", "invisible", baseline([{}, { invisible: true }, {}, { invisible: true }]), { scope: true });
for (const seed of [23, 71, 99127, 198461]) add(`random-${seed}`, "random", baseline(), { seed });

const program = `package main
import("encoding/json";"fmt";"os";"github.com/k0ngk0ng/wire-board/games/pokepath/engine")
type Unit struct { Slot int;HP,Armor,Distance,Speed float64;Invisible bool;Conditions []string }
type Input struct { Name,Mode string;Enemies []Unit;Order []int;Scope bool;Seed uint32 }
func must(err error){if err!=nil {panic(err)}}
func main(){
 var inputs []Input;must(json.NewDecoder(os.Stdin).Decode(&inputs));results:=map[string][]int{}
 for _,input:=range inputs {
  state,err:=engine.New(engine.Options{Starter:"charmander",Speed:1},2,99127,1730000000000);must(err)
  owner:=&state.Team[0];owner.Species="megaBarbacle";owner.Stats=engine.Stats{Power:6,Range:188,AttackMS:1000};owner.Sprite=engine.CatalogData().Species[owner.Species].Sprite
  ownerID:=owner.ID
  apply:=func(action engine.Action){action.Epoch=state.Epoch;if err:=engine.Apply(state,0,action);err!=nil{panic(fmt.Sprintf("%s/%s: %s",input.Name,action.Type,err))}}
  apply(engine.Action{Type:"deploy",PokemonID:ownerID,TileID:1})
  if input.Scope {state.Inventory=append(state.Inventory,engine.OwnedItem{ID:"silphScope"});apply(engine.Action{Type:"equip",PokemonID:ownerID,ItemID:"silphScope"})} else {apply(engine.Action{Type:"target",PokemonID:ownerID,Mode:input.Mode})}
  apply(engine.Action{Type:"wave"});state.Enemies=nil;slots:=map[int]int{}
  tower:=&state.Towers[0];tower.CooldownMS=0
  for _,unit:=range input.Enemies {
   id,err:=engine.SpawnEnemyAt(state,"rattata",engine.Point{X:tower.X+53+float64(unit.Slot)*9,Y:tower.Y},0,0);must(err);slots[id]=unit.Slot
   enemy:=&state.Enemies[len(state.Enemies)-1];enemy.Entered=true;enemy.HP,enemy.MaxHP=unit.HP,unit.HP;enemy.Armor,enemy.MaxArmor=unit.Armor,unit.Armor;enemy.Distance,enemy.BaseSpeed,enemy.Invisible=unit.Distance,unit.Speed,unit.Invisible
   for _,condition:=range unit.Conditions { magnitude:=0.0;if condition=="slow"{magnitude=.5};enemy.Conditions=append(enemy.Conditions,engine.Condition{Kind:condition,RemainingMS:3000,DurationMS:3000,Magnitude:magnitude,Stacks:1}) }
  }
  if len(input.Order)>0 {var reordered []engine.Enemy;for _,slot:=range input.Order{reordered=append(reordered,state.Enemies[slot])};state.Enemies=reordered}
  if input.Seed!=0{state.Random=input.Seed}
  must(engine.Advance(state,20))
  shots:=[]int{};for _,event:=range state.Events {if event.Type=="fire"&&event.Source==tower.ID{shots=append(shots,slots[event.Target])}}
  results[input.Name]=shots
 }
 must(json.NewEncoder(os.Stdout).Encode(map[string]any{"rulesVersion":engine.RulesVersion(),"targets":results}))
}`;

test("independent real attacks select every target in the recorded order", { timeout: 90000 }, async () => {
  const method = await referenceDOMClass("game/component/Tower.js", "Tower", ["getOrderedEnemies"]);
  const folder = join(archive, `fixture-${randomUUID().slice(0, 8)}`);
  await mkdir(folder, { recursive: true });
  const source = join(folder, "targets.go");
  const data = join(folder, "input.json");
  const referenceFile = join(folder, "reference.mjs");
  await writeFile(source, program);
  await writeFile(data, JSON.stringify(inputs));
  await writeFile(referenceFile, `${method.code}\nexport { Tower };\n`);
  const report = { command: "node --test games/pokepath/tests/target-priority-reference.test.mjs", reference: method.identity, expected: {}, inputs, passed: false };
  try {
    const { Tower } = await import(pathToFileURL(referenceFile).href);
    const reference = new Tower();
    for (const input of inputs) {
      const order = input.order ?? input.enemies.map(value => value.slot);
      const enemies = order.map(slot => {
        const unit = input.enemies[slot];
        return { ...unit, distanceTraveled: unit.distance, speed: unit.speed * (unit.conditions.includes("slow") ? 0.5 : 1), statusEffects: unit.conditions.map(type => ({ type })) };
      });
      reference.targetMode = input.mode;
      reference.pokemon = input.scope ? { item: { id: "silphScope" }, id: 650 } : undefined;
      let rollState = input.seed ?? 0;
      const restoreRandom = Math.random;
      if (input.seed) Math.random = () => {
        rollState ^= rollState << 13;
        rollState ^= rollState >>> 17;
        rollState ^= rollState << 5;
        return (rollState >>> 0) / 4294967296;
      };
      try {
        const first = reference.getOrderedEnemies(enemies);
        const expected = input.seed ? [first[0], ...reference.getOrderedEnemies(enemies).filter(enemy => enemy !== first[0])] : first;
        report.expected[input.name] = expected.map(enemy => enemy.slot);
      } finally { Math.random = restoreRandom; }
    }
    const { stdout } = await run("sh", ["-c", 'go run "$1" < "$2"', "target-priority", source, data], { cwd: root, timeout: 70000, maxBuffer: 4 << 20 });
    const result = JSON.parse(stdout);
    report.rulesVersion = result.rulesVersion;
    report.actual = result.targets;
    report.mismatches = inputs.filter(({ name }) => JSON.stringify(report.expected[name]) !== JSON.stringify(result.targets[name])).map(({ name }) => name);
    assert.deepEqual(report.mismatches, []);
    report.passed = true;
  } finally {
    await rm(folder, { recursive: true, force: true });
    await writeFile(join(archive, "result.json"), JSON.stringify(report, null, 2) + "\n");
  }
});
