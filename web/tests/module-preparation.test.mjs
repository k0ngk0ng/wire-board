// Before implementation: missing waiting extension, wrong ordering/layout,
// preparation leaking into create, draft options replacing saved options,
// lost assets/identity/busy props, game-specific transport instead of prepare.
import test from 'node:test';
import assert from 'node:assert/strict';
import {build} from 'esbuild';
import {createRequire} from 'node:module';
import {mkdirSync,writeFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
const root=fileURLToPath(new URL('../../',import.meta.url));
const out=path.join(root,'.local/module-preparation');mkdirSync(out,{recursive:true});
const result=await build({stdin:{contents:`
import React from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import {ModuleCreate,ModuleWaiting} from './src/game-api/host.tsx';
import {AssetsContext} from './src/ui/board-host.tsx';
export function exercise(mode,room,busy=false,host=false,enabled=true){
 let capture=null;const commands=[];
 globalThis.__preparationFixture={id:'fixture',name:'准备扩展',description:'契约fixture',minPlayers:2,maxPlayers:4,supportsBots:true,Board:()=>null,
 Options:()=>React.createElement('span',{'data-slot':'options'},'规则'),
 Cover:()=>React.createElement('svg',{'data-slot':'cover'}),
 ...(enabled?{Preparation:props=>{capture=props;return React.createElement('section',{'data-slot':'preparation'},'选图')}}:{})};
 const props=mode==='waiting'?{room,busy,host,command:(...args)=>commands.push(args)}:{kind:'fixture',busy,onSubmit:()=>{},draft:{name:'测试',password:''},onDraftChange:()=>{}};
 const html=renderToStaticMarkup(React.createElement(AssetsContext.Provider,{value:'https://assets.example.test/v1'},React.createElement(mode==='waiting'?ModuleWaiting:ModuleCreate,props)));
 return {html,capture,commands};
}
`,resolveDir:path.join(root,'web'),sourcefile:'preparation-fixture.tsx',loader:'tsx'},bundle:true,platform:'node',format:'cjs',jsx:'automatic',write:false,loader:{'.css':'empty'},plugins:[{name:'isolated-module-registry',setup(b){b.onResolve({filter:/^\.\.\/profiles$/},()=>({path:'profiles',namespace:'preparation-profile'}));b.onLoad({filter:/.*/,namespace:'preparation-profile'},()=>({contents:'export const PlayerName=({user})=>user.name;',loader:'js'}));b.onResolve({filter:/^\.\/registry$/},args=>args.importer.endsWith('/game-api/host.tsx')?{path:'registry',namespace:'preparation-fixture'}:null);b.onLoad({filter:/.*/,namespace:'preparation-fixture'},()=>({contents:'export const gameModule=()=>globalThis.__preparationFixture;export const gameName=()=>"fixture";',loader:'js'}));}}]});
const compiled=path.join(out,'ui-fixture.cjs');writeFileSync(compiled,result.outputFiles[0].contents);
const {exercise}=createRequire(import.meta.url)(compiled);
const room={id:'r1',kind:'fixture',name:'测试房间',host:'host',capacity:4,status:'waiting',version:7,you:1,options:{choice:3},seats:[{id:'host',name:'房主',ready:false},{id:'guest',name:'玩家',ready:true},{id:'bot',name:'电脑',ready:true,bot:true}]};
const evidence=[];
test('waiting retains shared layout and forwards authoritative preparation props/actions',()=>{
 const view=exercise('waiting',room,true,false);
 assert.ok(view.capture,'Preparation was not rendered');
 assert.equal(view.capture.room,room);assert.equal(view.capture.options,room.options);
 assert.equal(view.capture.host,false);assert.equal(view.capture.busy,true);
 assert.equal(view.capture.assets,'https://assets.example.test/v1');
 for(const token of ['waiting-cover','waiting-seats','seats-grid','waiting-actions'])assert.ok(view.html.includes(token));
 assert.ok(view.html.indexOf('data-slot="options"')<view.html.indexOf('data-slot="preparation"'));
 assert.ok(view.html.indexOf('data-slot="preparation"')<view.html.indexOf('class="seats-grid"'));
 view.capture.prepare({type:'pick',value:2});
 assert.deepEqual(view.commands,[['prepare',{action:{type:'pick',value:2}}]]);
 const fresh={...room,version:8,options:{choice:7}};const update=exercise('waiting',fresh,false,true);
 assert.equal(update.capture.options,fresh.options);assert.equal(update.capture.host,true);assert.equal(update.capture.busy,false);
 evidence.push({check:'waiting props/layout/transport',passed:true,html:view.html});
});
test('create never renders Preparation and optional absence preserves waiting shell',()=>{
 const create=exercise('create',room);assert.equal(create.capture,null);assert.ok(!create.html.includes('data-slot="preparation"'));assert.ok(create.html.includes('data-slot="options"'));
 const absent=exercise('waiting',room,false,true,false);assert.equal(absent.capture,null);assert.ok(absent.html.includes('waiting-seats'));assert.ok(absent.html.includes('seats-grid'));
 evidence.push({check:'create separation/optional extension',passed:true,html:create.html});
});
test.after(()=>writeFileSync(path.join(out,'ui-render.json'),JSON.stringify({scope:'real React server rendering with isolated module fixture; no game rules',checks:evidence},null,2)+'\n'));
