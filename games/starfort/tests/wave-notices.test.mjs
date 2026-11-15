// Failure inventory precedes implementation in OVERLAY_PROMPTS.md.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { bossEntry, activeDisasterLabel, taskPromptKey, shouldPromptTask, noticePosition } from '../web/wave-notices.ts';

test('boss forecast names the actual last spawn route and ignores inactive entries', () => {
 const map={path:[{x:0,y:0},{x:100,y:0}],routes:[{id:'west',label:'西侧山路',active:true},{id:'closed',label:'未开放',active:false},{id:'east',label:'东侧山路',active:true},{id:'south',label:'南侧河道',active:true}]};
 assert.equal(bossEntry(map,{boss:true,total:44}).id,'east');
 assert.equal(bossEntry(map,{boss:true,total:20}).label,'东侧山路');
 assert.equal(bossEntry(map,{boss:true,total:21}).id,'south');
 assert.equal(bossEntry(map,{boss:false,total:44}),undefined);
 assert.equal(bossEntry({...map,routes:[]},{boss:true,total:44}),undefined);
 assert.equal(bossEntry({path:map.path},{boss:true,total:1}).id,'main');
});

test('disaster text covers only generated effects and ends with the last effect', () => {
 const g={wave:5,kind:'flood',triggered:true,rewarded:false,zones:[{kind:'flood',createdMS:1000,expiresMS:4000},{kind:'flood',createdMS:2000,expiresMS:6000}]};
 assert.equal(activeDisasterLabel(g,999),null);
 assert.equal(activeDisasterLabel(g,1000),'洪水');
 assert.equal(activeDisasterLabel(g,4000),'洪水');
 assert.equal(activeDisasterLabel(g,6000),null);
 assert.equal(activeDisasterLabel({...g,rewarded:true},3000),null);
 assert.equal(activeDisasterLabel({...g,triggered:false},3000),null);
 const typhoon={...g,kind:'typhoon',zones:[{kind:'typhoon',createdMS:1000,expiresMS:5000,activated:false}]};
 assert.equal(activeDisasterLabel(typhoon,3000),null);
 assert.equal(activeDisasterLabel({...typhoon,zones:[{...typhoon.zones[0],activated:true}]},3000),'台风');
 assert.equal(activeDisasterLabel({...typhoon,zones:[{...typhoon.zones[0],activated:true,skipped:true}]},3000),null);
 assert.equal(activeDisasterLabel({...g,kind:'earthquake',zones:[{kind:'earthquake',createdMS:1000}]},6000),'地震');
});

test('task inquiry waits for the boss but not an unopened reward bag or dock', () => {
 const state={phase:'prepare',spectating:false,bossOpen:false,worldChoice:false,rewardOpen:false,placing:false};
 assert.equal(shouldPromptTask(state),true);
 for(const patch of [{bossOpen:true},{worldChoice:true},{rewardOpen:true},{placing:true},{spectating:true},{phase:'battle'},{phase:'paused'}])assert.equal(shouldPromptTask({...state,...patch}),false);
 assert.equal(shouldPromptTask({...state,bossOpen:false,hasBoss:true,storedRewards:true,shopExists:true}),true);
 assert.notEqual(taskPromptKey('r','m',10,'same'),taskPromptKey('r','m',11,'same'));
 assert.notEqual(taskPromptKey('r','m',10,'same'),taskPromptKey('r','next',10,'same'));
});

test('map-left notices fit and prefer a road-free gap below the HUD', () => {
 const viewport={width:320,height:560};
 const avoid=[{left:8,top:8,right:312,bottom:116},{left:8,top:460,right:132,bottom:552}];
 const roads=[[{x:0,y:190},{x:320,y:190}],[{x:0,y:400},{x:320,y:400}]];
 const result=noticePosition(viewport,{width:180,height:104},avoid,roads,20);
 assert.equal(result.left,8);
 assert.ok(result.top>=210&&result.top+104<=380,result);
 assert.ok(result.top+104<=viewport.height-8);
 const tiny=noticePosition({width:200,height:240},{width:180,height:104},[],[],20);
 assert.ok(Number.isFinite(tiny.top)&&tiny.top>=8&&tiny.top+104<=232);
});
