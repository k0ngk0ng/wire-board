import { readFile } from "node:fs/promises";
import assert from "node:assert/strict";
import { domSHA, referenceDOMClass, verifyDOMOracle } from "./dom-oracle.mjs";

const referenceRoot = new URL("../../../.local/pokepath/research/source/src/js/", import.meta.url);
const source = path => readFile(new URL(path, referenceRoot), "utf8");
const [routeJSON, enemySource, towerSource, ...classes] = await Promise.all([
  readFile(new URL("../assets/public/pokepath/v1/routes/0.json", import.meta.url), "utf8"),
  source("game/component/Enemy.js"), source("game/component/Tower.js"),
  referenceDOMClass("utils/Utility.js", "Utility", ["numberDot"]),
  referenceDOMClass("utils/Sprite.js", "Sprite", ["draw"]),
  referenceDOMClass("game/component/Enemy.js", "Enemy", ["draw", "drawStatusEffects", "drawSlowIndicator", "drawStunIndicator"]),
  referenceDOMClass("game/component/PlacementTile.js", "PlacementTile", ["drawRange", "draw", "update", "canPlacePokemonHere", "computeEffectiveRange"])
]);
const route = JSON.parse(routeJSON);
const directionsStart = enemySource.indexOf("const currentWp = this.waypoints[this.waypointIndex]");
const directionsEnd = enemySource.indexOf("this.center = {", directionsStart);
const cropStart = towerSource.indexOf("const crop = {", towerSource.indexOf("draw() {"));
const cropEnd = towerSource.indexOf("this.drawAbilityField();", cropStart);
const towerStart = towerSource.indexOf("if (this.pokemon?.item?.id === 'inverter') {", cropEnd);
const towerEnd = towerSource.indexOf("if (['intrepidSword', 'dauntlessShield'].includes(this.ability?.id))", towerStart);
assert.ok(directionsStart > 0 && directionsEnd > directionsStart && cropStart > 0 && cropEnd > cropStart && towerStart > cropEnd && towerEnd > towerStart);
const enemyFacing = enemySource.slice(directionsStart, directionsEnd);
const towerArt = `${towerSource.slice(cropStart, cropEnd)}\n${towerSource.slice(towerStart, towerEnd)}`;
const enemy = (id, key, x, y, waypoint, health, extra = {}) => ({ id, key, x, y, waypoint, pathIndex: 0, entered: true, health, maxHealth: 31000, armor: 0, maxArmor: 0, conditions: [], ...extra });
const pokemon = (id, species, item = "") => ({ id, species, item, target: "first", stats: { range: 112 }, level: 5 });
const tower = (id, pokemonID, x, y, targetID = 0) => ({ id, pokemonID, x, y, targetID, tileID: id });
const basic = { wave: 1, team: [], towers: [], enemies: [] };
const armor = enemy(21, "octillery", 263.25, 213, 4, 10351, { armor: 1234, maxArmor: 4000 });
const bodies = [enemy(5, "rattata", 358.5, 15.5, 1, 31000), enemy(6, "rattata", 198, 69, 2, 15500), enemy(7, "rattata", 118.5, 170, 3, 255), enemy(8, "rattata", 390, 213, 4, 1500)];
const ward = pokemon(17, "charmander");
const cases = {
  opening: basic,
  foggedView: { ...basic, effect: 2 },
  entry: { ...basic, enemies: [enemy(2, "rattata", 358.5, 4, 1, 31000, { entered: false })] },
  fourDirections: { ...basic, enemies: bodies },
  alternatePath: { ...basic, enemies: [enemy(9, "rattata", 205, 205, 0, 1200, { pathOriginWaypoint: 4 }), enemy(10, "rattata", 403, 393, 1, 1200, { pathMirrored: true })] },
  overlaps: { ...basic, enemies: [enemy(11, "rattata", 352, 213, 4, 15000), enemy(12, "kecleon", 360, 211, 4, 1200, { invisible: true })] },
  health: { ...basic, display: 2, enemies: [armor, enemy(22, "mew", 420, 381, 6, 25001, { invisible: true }), enemy(23, "calyrex", 330, 549, 8, 15551)] },
  wave100: { ...basic, display: 1, wave: 100, enemies: [armor, ...bodies] },
  ordinaryWave: { ...basic, display: 1, wave: 99, enemies: [armor, enemy(25, "mew", 390, 381, 6, 10551)] },
  curseAndGuard: { ...basic, enemies: [enemy(33, "stakataka", 490, 381, 6, 17000, { passiveId: "ironDefense", passiveCounters: { ironDefenseHits: 47 }, armor: 3200, maxArmor: 8000, conditions: [{ kind: "curse", durationMs: 2000, remainingMs: 1000 }] })] },
  fadeStart: { ...basic, enemies: [enemy(28, "rattata", 330, 213, 4, 0, { dead: true, deathRemainingMS: 560 })] },
  fadeHalf: { ...basic, enemies: [enemy(29, "rattata", 330, 213, 4, 0, { dead: true, deathRemainingMS: 280 }), enemy(30, "kecleon", 440, 381, 6, 0, { dead: true, deathRemainingMS: 280, invisible: true })] },
  absent: { ...basic, enemies: [enemy(31, "rattata", 330, 213, 4, 0, { dead: true, deathRemainingMS: 0 }), enemy(32, "rattata", 388, 213, 4, 17000, { escaped: true })] },
  rebirth: { ...basic, enemies: [enemy(35, "hooh", 330, 213, 4, 1, { reviveRemainingMS: 4580, armor: 50000, maxArmor: 50000 })] },
  deployed: { ...basic, team: [ward], towers: [tower(40, 17, 252, 132)] },
  attacking: { ...basic, team: [ward], towers: [tower(40, 17, 252, 132, 6)], enemies: bodies },
  inverted: { ...basic, team: [pokemon(18, "charmander", "inverter"), pokemon(19, "bidoof", "inverter")], towers: [tower(40, 18, 252, 132), tower(41, 19, 480, 304)] },
  hover: { ...basic, team: [ward], towers: [tower(40, 17, 252, 132)], selected: 17, enemies: [armor] },
  allRanges: { ...basic, team: [ward], towers: [tower(40, 17, 252, 132)], all: true, enemies: [armor], effect: 2 },
  spectrierSorted: { ...basic, team: [ward, pokemon(20, "spectrier")], towers: [tower(42, 20, 254, 123), tower(40, 17, 252, 132)], hit: { x: 253, y: 128, expected: 20 } },
  depthSorted: { ...basic, team: [ward, pokemon(24, "treecko")], towers: [tower(42, 24, 254, 136), tower(40, 17, 252, 132)], hit: { x: 253, y: 136, expected: 24 } }
};
for (const [key, passiveId] of [["moltres", "flameBody"], ["articuno", "snowCloak"], ["zapdos", "static"], ["carnivine", "ingrain"], ["whimsicott", "tailWind"]]) {
  for (const particles of [false, true]) {
    cases[`${passiveId}-${particles ? "particles" : "ring"}`] = { ...basic, now: particles ? 486 : 1711, particles, enemies: [enemy(45, key, 359, 381, 6, 9000, { passiveId })] };
  }
}
cases.auraAtEdge = { ...basic, now: 229, enemies: [enemy(46, "articuno", 420, -130, 1, 9000, { passiveId: "snowCloak", entered: false })] };
cases.fadingAura = { ...basic, now: 2450, enemies: [enemy(47, "moltres", 480, 381, 6, 0, { passiveId: "flameBody", dead: true, deathRemainingMS: 260 }), enemy(48, "articuno", 228, 213, 4, 0, { passiveId: "snowCloak", dead: true, deathRemainingMS: 260 })] };
const tiles = [route.placements.find(tile => tile.terrain === 3), route.placements.find(tile => tile.terrain === 4), ...route.placements.filter(tile => tile.terrain === 1).slice(0, 3)];
assert.ok(tiles.every(Boolean) && tiles.length === 5);
const [wet, high, occupied, hosted, free] = tiles;
const centerOf = tile => ({ x: tile.x + 12, y: tile.y + 12 });
const at = (id, pokemonID, tile) => ({ ...tower(id, pokemonID, tile.x + 12, tile.y + 12), tileID: tile.id });
const preview = { ...basic, team: [pokemon(40, "charmander", "airBalloon"), pokemon(41, "rattata"), pokemon(42, "turtwig")], towers: [at(51, 41, occupied), at(52, 42, hosted)], tiles, selected: 40 };
for (const [name, hovered] of [["deployPreview", undefined], ["deployHoverItem", high], ["deployHoverHost", hosted], ["deployHoverBlocked", wet]]) {
  cases[name] = { ...preview, deployment: { tileSize: 24, point: hovered ? centerOf(hovered) : undefined, footprint: hovered && hovered.id !== wet.id ? { center: centerOf(hovered), range: 112, shape: "circle", innerRange: 0, halfWidth: 24 } : undefined, sites: { [high.id]: "empty", [wet.id]: "blocked", [occupied.id]: "occupied", [hosted.id]: "host", [free.id]: "empty" } } };
}
const rangeScene = (species, item, tile, raw, range, shape, halfWidth = 24, innerRange = 0) => ({ ...basic, team: [{ ...pokemon(60, species, item), stats: { range: raw } }], towers: [], selected: 60, tiles: [tile], originalRange: true, deployment: { tileSize: 24, point: centerOf(tile), sites: { [tile.id]: "empty" }, footprint: { center: centerOf(tile), range, shape, halfWidth, innerRange } } });
cases.deployMountainRange = rangeScene("pidgey", "", high, 125, 250, "circle");
cases.deployNoGuard = rangeScene("megaPidgeot", "", high, 220, 2200, "circle");
cases.deployRing = rangeScene("hoothoot", "", free, 146, 146, "donut", 24, 100);
cases.deployCross = rangeScene("yamask", "", free, 139, 139, "cross");
cases.deployWideStar = { ...rangeScene("yamask", "starPiece", free, 139, 139, "star", 36), victory: true };
cases.deployWideX = rangeScene("cryogonal", "wideLens", free, 180, 180, "xShape", 72);
const lightTile = route.placements.find(tile => tile.id === 15);
const litField = route.placements.find(tile => tile.id === 14);
cases.deployIlluminatedRange = rangeScene("pidgey", "", litField, 125, 140, "circle");
cases.deployIlluminatedRange.team.push({ ...pokemon(61, "patrat"), stats: { range: 100 } });
cases.deployIlluminatedRange.towers.push(at(62, 61, lightTile));
const reference = `
 const {catalog,input,parameters}=window.oracleData, {route}=input, scene=input.cases[parameters.scene];
 const SHOW_ENEMY_SPEED_DEBUG=false;
 ${classes.map(value=>value.code.replace("class Enemy {", "class Enemy extends Sprite {")).join("\n")}
 const canvas=document.createElement('canvas');canvas.width=route.width;canvas.height=route.height;
 Object.assign(canvas.style,{position:'absolute',left:'259px',top:'25px',transform:'none',outline:'none'});
 document.querySelector('.section-scene-container').remove();
 document.body.append(canvas);
 const ctx=canvas.getContext('2d'), images=new Map();
 const serialized=JSON.stringify(scene);
 await Promise.all([...new Set([route.background,route.effect,'src/assets/images/icons/curse.png',...scene.enemies.map(value=>catalog.enemies[value.key].sprite.image),...scene.team.map(value=>catalog.species[value.species].sprite.image)])].map(async path=>{const image=new Image();image.src='./'+path;await image.decode();images.set(path,image)}));
 ctx.drawImage(images.get(route.background),0,0,canvas.width,canvas.height);
 const main={utility:new Utility(),displayHealth:scene.display??0,lang:Number(parameters.language),area:{waveNumber:scene.wave},durationCC:true,particles:Boolean(scene.particles)};
 function turn(){${enemyFacing}}
 function paintTower(){${towerArt}}
 const savedNow=Date.now,savedRandom=Math.random;
 let seed=20261118;
 Date.now=()=>scene.now??0;
 Math.random=()=>((seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296);
 try { for(const value of [...scene.enemies].reverse()){
  if(value.escaped||value.dead&&!value.deathRemainingMS)continue;
  const spec=catalog.enemies[value.key].sprite, width=spec.frameWidth,height=spec.frameHeight;
  const path=(value.pathMirrored?route.mirrorPaths:route.paths)[value.pathIndex].slice(value.pathOriginWaypoint??0);
  const duration=value.reviveRemainingMS?5000-value.reviveRemainingMS:0;
  const opacity=value.dead?1-(560-value.deathRemainingMS)*.03/(1000/60):duration?1+.5*Math.sin(duration/120):1;
  const original=Object.assign(Object.create(Enemy.prototype),{ctx,main,width,height,position:{x:value.x-width/2,y:value.y-height/2},center:{x:value.x,y:value.y},waypoints:path,waypointIndex:value.waypoint,sprite:images.get(spec.image),frames:{current:0,direction:0,max:spec.frames,rows:8,hold:5,elapsed:0},loaded:true,enemy:{id:value.key},hp:value.health,hpMax:value.maxHealth,armor:value.armor,armorMax:value.maxArmor,invisible:value.invisible,passive:value.passiveId?{id:value.passiveId}:undefined,passiveCharges:value.passiveCounters?.ironDefenseHits??0,curseIcon:images.get('src/assets/images/icons/curse.png'),cursed:value.conditions.some(condition=>condition.kind==='curse'),isRegeneratorReviving:duration>0,reviveScale:1+.05*Math.sin(duration/150),statusEffects:value.conditions.map(condition=>({type:condition.kind,duration:condition.durationMs/1000,timer:condition.durationMs-condition.remainingMs}))});
  turn.call(original);
  ctx.save();ctx.globalAlpha=opacity;original.draw();ctx.restore();
 }} finally { Date.now=savedNow; Math.random=savedRandom; }
 const isSpectrier=tower=>scene.team.find(p=>p.id===tower.pokemonID).species==='spectrier';
 for(const tower of [...scene.towers].sort((a,b)=>Number(isSpectrier(a))-Number(isSpectrier(b))||a.y-b.y)){
  const pokemon=scene.team.find(p=>p.id===tower.pokemonID), specie=catalog.species[pokemon.species], spec=specie.sprite;
  paintTower.call({ctx,width:spec.frameWidth,height:spec.frameHeight,position:{x:tower.x-12,y:tower.y-12},frames:{current:0,direction:0},sprite:images.get(spec.image),pokemon:{item:pokemon.item?{id:pokemon.item}:undefined,ability:{id:specie.abilityId}}});
 }
 if(scene.deployment){
  const adapt=p=>{const spec=catalog.species[p.species];return{id:spec.id,tiles:spec.terrains,ability:{id:spec.abilityId},item:p.item?{id:p.item}:null,rangeType:spec.rangeType,innerRange:spec.innerRange,range:p.stats.range}};
  const selected=adapt(scene.team.find(p=>p.id===scene.selected));
  main.game={deployingUnit:selected};main.UI={};main.area.towers=scene.towers.map(t=>{const pokemon=adapt(scene.team.find(p=>p.id===t.pokemonID));return{pokemon,range:pokemon.range,ability:pokemon.ability,center:{x:t.x,y:t.y}}});
  Object.assign(main.area,{routeNumber:route.id,victoryStar:scene.victory});
  main.player={stars:0,ribbons:0,fossilInTeam:0};
  const point=scene.deployment.point??{};
  for(const site of scene.tiles){
   const owner=scene.towers.find(tower=>tower.tileID===site.id), pokemon=owner?adapt(scene.team.find(p=>p.id===owner.pokemonID)):false;
   const tile=Object.assign(Object.create(PlacementTile.prototype),{main,ctx,land:site.terrain,position:site,size:24,center:{x:site.x+12,y:site.y+12},tower:pokemon,passenger:false,color:'rgba(255, 255, 255, 0.5)'},scene.originalRange?{}:{computeEffectiveRange:()=>selected.range});
   if(!pokemon&&point.x>site.x&&point.x<site.x+24&&point.y>site.y&&point.y<site.y+24&&tile.canPlacePokemonHere(selected))tile.color='white';
   tile.update(point);
  }
 }
 for(const tower of scene.towers.filter(tower=>scene.all||!scene.deployment&&tower.pokemonID===scene.selected)){
  const pokemon=scene.team.find(p=>p.id===tower.pokemonID),specie=catalog.species[pokemon.species];
  const tile=Object.assign(Object.create(PlacementTile.prototype),{center:{x:tower.x,y:tower.y},ctx,main});
  tile.drawRange(pokemon.stats.range,specie.rangeType,specie.innerRange,{id:specie.abilityId},pokemon.item?{id:pokemon.item}:null,Boolean(scene.all));
 }
 if(scene.effect){ctx.save();ctx.globalAlpha=1;ctx.drawImage(images.get(route.effect),0,0,canvas.width,canvas.height);ctx.restore()}
 window.battleObservation={width:canvas.width,height:canvas.height,unchanged:serialized===JSON.stringify(scene),hit:scene.hit?.expected};
`;
const parameters = Object.keys(cases).map(scene => ({ language: scene === "health" ? "2" : scene === "wave100" ? "0" : "8", theme: "original", font: "PressStart2P", scene }));
parameters.push({ language: "8", theme: "original", font: "PressStart2P", scene: "health" });
const observation = "window.battleObservation";
await verifyDOMOracle({ name: "battle-body", command: "node games/pokepath/tests/battle-body-oracle.mjs", scope: "Unmasked 720x624 Canvas from original draw fragments versus the shipped independent renderer, with equal positions, assets, clocks, random values and settings", sources: [...classes.map(part => part.identity), { path: "game/component/Tower.js", sha256: domSHA(towerSource), version: "1.7.3" }], input: { route: { ...route, waves: {} }, cases }, reference, native: 'import "./games/pokepath/tests/battle-body-fixture.mjs";', cases: parameters, steps: [{ name: "draw" }], styles: [], measure: { native: observation, reference: observation } });
