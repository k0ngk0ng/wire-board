import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import * as T from "../../../web/node_modules/three/build/three.module.js";
import { ClassicPropPlayback, ClassicPropClipPlayback, validateClassicPropCatalog, classicAmbientClipNames } from "../web/classic-prop-animation.ts";

// Failure inventory was written before the renderer or helper implementation.
const hash = "a".repeat(64), geometry = "b".repeat(64);
function catalog() {
  return {version:1,unit:"metres",track:"france",geometry,catalogHash:hash,
    animations:{low:{duration:.46875},high:{duration:1.5}},
    props:[0,1].map(index=>({index,id:`france:${index}`,node:`prop_${index}`,motionNode:`prop_${index}_motion`,clips:{low:`prop_${index}_destroy_low_speed`,high:`prop_${index}_destroy_high_speed`},ruleEdges:[index]}))};
}
const world = (bits=0) => ({catalog:hash,geometry,byEdge:new Map([[0,0],[1,1]]),broken:new Uint8Array([bits])});
const event = (prop, at=1000, high=false, id=1) => ({id,type:"prop",prop,at,high,dir:2.8,seat:0,intensity:.5,x:0,y:0});

test("unbroken props stand still, and event-only prediction cannot knock them down",()=>{
  const p=new ClassicPropPlayback(catalog());
  assert.deepEqual(p.update(world(),[event(0)],1100,false).map(x=>x.phase),["standing","standing"]);
  assert.deepEqual(p.update(undefined,[event(0)],1100,false).map(x=>x.phase),["standing","standing"]);
});

test("confirmed low and high hits seek by authority elapsed time and preserve native durations",()=>{
  const p=new ClassicPropPlayback(catalog());
  const rows=p.update(world(3),[event(0),event(1,1000,true,2)],1250,false);
  assert.deepEqual(rows.map(x=>[x.phase,x.time,x.duration,x.high]),[["animating",.25,.46875,false],["animating",.25,1.5,true]]);
  assert.equal(rows[0].clip,"prop_0_destroy_low_speed");
  assert.equal(rows[1].clip,"prop_1_destroy_high_speed");
});

test("duplicate events, clock regression and later alternate events never restart a hit",()=>{
  const p=new ClassicPropPlayback(catalog());
  p.update(world(1),[event(0)],1200,false);
  let row=p.update(world(1),[event(0),event(0,1300,true,9)],1300,false)[0];
  assert.equal(row.eventId,1);assert.equal(row.high,false);assert.equal(row.time,.3);
  row=p.update(world(1),[event(0)],1100,false)[0];assert.equal(row.time,.3);
});

test("completion is held once at the original final frame after event-ring eviction",()=>{
  const p=new ClassicPropPlayback(catalog());
  p.update(world(1),[event(0)],1200,false);
  const end=p.update(world(1),[],1700,false)[0];
  assert.equal(end.phase,"complete");assert.equal(end.time,.46875);assert.equal(end.eventId,1);
  assert.deepEqual(p.update(world(1),[event(0)],9000,false)[0],end);
});

test("a broken-only restoration or an expired first event is hidden without replay",()=>{
  for(const events of [[],[event(0)]]) {
    const p=new ClassicPropPlayback(catalog());
    assert.equal(p.update(world(1),events,3000,false)[0].phase,"hidden");
    assert.equal(p.update(world(1),[event(0,3050,true,4)],3100,false)[0].phase,"hidden");
  }
});

test("reduced motion immediately holds final pose and does not replay when enabled again",()=>{
  const p=new ClassicPropPlayback(catalog());
  let row=p.update(world(1),[event(0,1000,true)],1100,true)[0];
  assert.equal(row.phase,"complete");assert.equal(row.time,1.5);
  row=p.update(world(1),[event(0,1000,true)],1200,false)[0];assert.equal(row.phase,"complete");assert.equal(row.time,1.5);
});

test("future authority times wait, malformed/non-prop events cannot animate, and another prop stays independent",()=>{
  const p=new ClassicPropPlayback(catalog());
  assert.equal(p.update(world(1),[event(0,1100)],1000,false)[0].phase,"standing");
  assert.equal(p.update(world(1),[event(0,1100)],1150,false)[0].time,.05);
  const q=new ClassicPropPlayback(catalog());
  assert.equal(q.update(world(2),[{...event(1),type:"collision"},event(-1),event(9),{...event(1),at:NaN}],1100,false)[1].phase,"hidden");
});

test("wrong confirmed-world identity fails before changing visuals",()=>{
  const p=new ClassicPropPlayback(catalog());
  assert.throws(()=>p.update({...world(),catalog:"c".repeat(64)},[],1100,false),/版本/);
  assert.throws(()=>p.update({...world(),geometry:"c".repeat(64)},[],1100,false),/版本/);
});

test("catalog binding rejects wrong geometry, GLB identity, duplicate nodes and altered CAN durations",()=>{
  assert.equal(validateClassicPropCatalog(catalog(),"france",geometry,hash).props.length,2);
  assert.throws(()=>validateClassicPropCatalog(catalog(),"uk",geometry,hash),/目录/);
  assert.throws(()=>validateClassicPropCatalog(catalog(),"france","c".repeat(64),hash),/目录/);
  assert.throws(()=>validateClassicPropCatalog(catalog(),"france",geometry,"c".repeat(64)),/目录/);
  const invalid=catalog();invalid.props[1].node=invalid.props[0].node;
  assert.throws(()=>validateClassicPropCatalog(invalid,"france",geometry,hash),/目录/);
  const timing=catalog();timing.animations.high.duration=2;
  assert.throws(()=>validateClassicPropCatalog(timing,"france",geometry,hash),/目录/);
});

test("only original ambient transform clips play at startup, all collision clips are excluded",()=>{
  const names=classicAmbientClipNames(catalog(),[
    {name:"object_357_loop",extras:{fps:64}},
    {name:"prop_0_destroy_low_speed",extras:{trigger:"collision"}},
    {name:"prop_0_destroy_high_speed"},
    {name:"unknown_collision",extras:{trigger:"collision"}},
  ]);
  assert.deepEqual([...names],["object_357_loop"]);
});

test("all eight frozen native GLBs bind to the actual catalog and preserve two collision clips per prop",()=>{
  for(const track of ["france","coastal","uk","park"]) {
    const c=JSON.parse(readFileSync(new URL(`../assets/public/racing/v3/props/${track}.json`,import.meta.url)));
    for(const suffix of ["","-night"]) {
      const bytes=readFileSync(new URL(`../assets/public/racing/v3/tracks/${track}${suffix}.glb`,import.meta.url));
      const doc=JSON.parse(bytes.subarray(20,20+bytes.readUInt32LE(12)));
      validateClassicPropCatalog(c,track,c.geometry,doc.extras.propsCatalogHash);
      const byName=new Map(doc.nodes.map(node=>[node.name,node]));
      for(const prop of c.props) {
        const node=byName.get(prop.node),motion=byName.get(prop.motionNode);
        assert.equal(node.extras.destructible,true);assert.equal(node.extras.propIndex,prop.index);
        assert(node.children.includes(doc.nodes.indexOf(motion)));
        for(const speed of ["low","high"]) {
          const a=doc.animations.find(a=>a.name===prop.clips[speed]);assert(a);
          assert.equal(a.extras.trigger,"collision");assert.equal(a.extras.loop,false);
          assert(a.channels.every(channel=>doc.nodes[channel.target.node]===motion));
          assert.equal(doc.accessors[a.samplers[0].input].max[0],c.animations[speed].duration);
        }
      }
      assert.equal(classicAmbientClipNames(c,doc.animations).size,doc.animations.length-c.props.length*2);
    }
  }
});

test("real Three animation actions do not start early, seek late authority and clamp the native final transform",()=>{
  const root=new T.Group(),motion=new T.Group();motion.name="prop_0_motion";root.add(motion);
  const low=new T.AnimationClip("prop_0_destroy_low_speed",.46875,[new T.VectorKeyframeTrack("prop_0_motion.position",[0,.46875],[0,0,0,0,-2,3])]);
  const high=new T.AnimationClip("prop_0_destroy_high_speed",1.5,[new T.VectorKeyframeTrack("prop_0_motion.position",[0,1.5],[0,0,0,0,-5,9])]);
  const mixer=new T.AnimationMixer(root),player=new ClassicPropClipPlayback(mixer,[low,high],T.LoopOnce),state=new ClassicPropPlayback(catalog());
  player.apply(state.update(world(),[],1100,false));assert.deepEqual(motion.position.toArray(),[0,0,0]);
  player.apply(state.update(world(1),[event(0)],1250,false));
  assert.ok(Math.abs(motion.position.y-(-2*.25/.46875))<1e-6);
  const action=mixer.existingAction(low);assert.equal(action.loop,T.LoopOnce);assert.equal(action.clampWhenFinished,true);assert.equal(action.paused,true);
  player.apply(state.update(world(1),[],1700,false));assert.deepEqual(motion.position.toArray(),[0,-2,3]);
  player.apply(state.update(world(1),[event(0)],8000,false));mixer.update(8);
  assert.deepEqual(motion.position.toArray(),[0,-2,3]);
  player.stop();assert.equal(action.isRunning(),false);
});
