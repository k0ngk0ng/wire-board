import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { pvzResidentResources } from "../web/resident-assets.ts";
const manifest = JSON.parse(readFileSync(new URL("../assets/public/pvz/classic-v2/manifest.json",import.meta.url),"utf8"));
const registry = JSON.parse(readFileSync(new URL("../../../assets/resources.json",import.meta.url),"utf8")).files;
const prefix = "games/pvz/assets/public";
const media = Object.fromEntries(Object.entries(registry).filter(([path])=>path.startsWith(prefix+"/")).map(([path,file])=>[path.slice(prefix.length),file.bytes]));
media["/pvz/v1/runtime/pvz.wasm"] = 123;
test("each match includes future cards, effects, runtime and only its scene music",()=>{
 for(const scene of Object.keys(manifest.maps)){
  const paths=pvzResidentResources(scene,manifest,media).map(file=>file.url);
  assert.ok(paths.includes("/pvz/classic-v2/manifest.json"));
  assert.ok(paths.includes("/pvz/v1/runtime/pvz.wasm"));
  for(const path of Object.values(manifest.byUnit).flat())assert.ok(paths.includes("/pvz/classic-v2/"+path),path);
  for(const track of manifest.music[scene].tracks)assert.ok(paths.includes("/pvz/classic-v2/"+track.src));
  assert.ok(paths.every(path=>!path.startsWith("/pvz/classic-v1/")));
  assert.ok(paths.filter(path=>path.includes("/music/")).every(path=>manifest.music[scene].tracks.some(track=>path.endsWith("/"+track.src))));
 }
});
test("unknown scenes and missing required bytes stop readiness",()=>{
 assert.throws(()=>pvzResidentResources("missing",manifest,media));
 assert.throws(()=>pvzResidentResources("day",manifest,{}));
});
