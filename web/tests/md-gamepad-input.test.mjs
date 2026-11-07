// Failure spectrum: CONTROLLER_PHYSICAL_PORT_FAILURES.md precedes this test.
// Virtual Gamepad snapshots are not a physical brand compatibility claim.
import test from 'node:test';
import assert from 'node:assert/strict';
import {KeyboardController}from '../src/controllers/input.ts';
import {GamepadSampler,defaultGamepadProfile,gamepadKey,mergeLogicalButtons}from '../src/controllers/gamepad.ts';
const keyboard={up:'KeyW',down:'KeyS',left:'KeyA',right:'KeyD',mode:'KeyF',start:'Space',a:'KeyJ',b:'KeyK',c:'KeyL',x:'KeyU',y:'KeyI',z:'KeyO'};
const profile=()=>({revision:0,turboHz:10,keyboard:{...keyboard}});
const pad=()=>({id:'Virtual MD standard',index:3,mapping:'standard',connected:true,axes:[0,0],buttons:Array(17).fill(0)});
const sample=(s,p,t=0)=>[...s.sample(p,t)].sort();
test('standard MD6 preset maps all six ordinary buttons plus mode/start and directions',()=>{
 const p=pad(),mapping=defaultGamepadProfile(p,Object.keys(keyboard));
 for(const[name,index]of Object.entries({a:2,b:0,c:1,x:4,y:3,z:5,mode:8,start:9,up:12,down:13,left:14,right:15}))assert.ok(mapping.bindings[name].some(binding=>binding.type==='button'&&binding.index===index),name);
 assert.deepEqual(mapping.bindings.left.find(binding=>binding.type==='axis'),{type:'axis',index:0,direction:-1});
 const sampler=new GamepadSampler(profile());sample(sampler,[p]);for(const[name,index]of Object.entries({a:2,b:0,c:1,x:4,y:3,z:5,mode:8,start:9})){p.buttons.fill(0);p.buttons[index]=1;assert.deepEqual(sample(sampler,[p],10),[name]);assert.deepEqual(sample(sampler,[p],85),[name]);p.buttons.fill(0);sample(sampler,[p],100);}
});
test('MD U/I are continuous X/Y while FC named turbos keep their original phase',()=>{
 const input=new KeyboardController(profile());input.press('KeyU',0);input.press('KeyI',0);input.press('KeyO',0);for(const at of [0,49,50,75,100,175])assert.deepEqual([...input.sample(at)].sort(),['x','y','z']);
 const fc=new KeyboardController({revision:0,turboHz:10,keyboard:{a:'KeyK',b:'KeyJ',turboA:'KeyI',turboB:'KeyU'}});fc.press('KeyU',0);assert.deepEqual([...fc.sample(0)],['b']);assert.deepEqual([...fc.sample(75)],[]);fc.press('KeyJ',80);assert.deepEqual([...fc.sample(85)],['b']);
});
test('device profile identity, sparse attachment and source release remain independent for MD',()=>{
 const p=pad(),cfg=profile();cfg.gamepads={[gamepadKey(p)]:{deadzone:.3,bindings:{z:[{type:'button',index:7}],right:[{type:'axis',index:0,direction:1}]}}};const s=new GamepadSampler(cfg);sample(s,[null,null,null,p]);p.buttons[7]=1;p.axes[0]=.8;assert.deepEqual(sample(s,[null,null,null,p],10),['right','z']);
 assert.deepEqual([...mergeLogicalButtons(['a','left'],s.sample([],20))].sort(),['a','left']);assert.deepEqual(sample(s,[p],30),[]);p.buttons[7]=0;p.axes[0]=0;sample(s,[p],40);p.buttons[7]=1;assert.deepEqual(sample(s,[p],50),['z']);
 const changed=profile();changed.gamepadDevice='none';s.configure(changed);assert.deepEqual(sample(s,[p],60),[]);
});
