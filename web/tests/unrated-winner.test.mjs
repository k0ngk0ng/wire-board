// Failure inventory before implementation: unranked matches hide the actual
// winner/team/draw; casual/practice without winners prints an empty victory;
// displaying a winner changes rating eligibility or the existing result data.
import test from "node:test";
import assert from "node:assert/strict";
import {build} from "esbuild";
import {createRequire} from "node:module";
import {mkdirSync, writeFileSync} from "node:fs";
import {fileURLToPath} from "node:url";
import path from "node:path";
const root=fileURLToPath(new URL("../../",import.meta.url));
const out=path.join(root,".local/yuyu-mobile/settlement");mkdirSync(out,{recursive:true});
const bundle=await build({stdin:{contents:`
import React from 'react';import {renderToStaticMarkup} from 'react-dom/server';
import {Results} from './src/ui/results.tsx';
export function render(room){return renderToStaticMarkup(React.createElement(Results,{room,host:false,busy:false,command:()=>{}}));}
`,resolveDir:path.join(root,"web"),sourcefile:"unrated-results.tsx",loader:"tsx"},bundle:true,platform:"node",format:"cjs",jsx:"automatic",write:false,plugins:[{name:"result-boundaries",setup(b){
 b.onResolve({filter:/^\.\.\/profiles$/},()=>({path:"profiles",namespace:"result-fixture"}));
 b.onResolve({filter:/^\.\.\/game-api\/registry$/},()=>({path:"registry",namespace:"result-fixture"}));
 b.onLoad({filter:/.*/,namespace:"result-fixture"},args=>({contents:args.path==="profiles"?'export const PlayerName=({user})=>user.name;':'export const gameModule=()=>undefined;export const isIndependentGame=()=>true;',loader:"js"}));
}}]});
const file=path.join(out,"results.cjs");writeFileSync(file,bundle.outputFiles[0].contents);
const {render}=createRequire(import.meta.url)(file);
const room={kind:"yuyuhakusho",seats:[{id:"p0",name:"浦饭玩家"},{id:"p1",name:"飞影玩家"}],game:{winners:[0]},result:{unrated:true,rated:false,players:[{id:"p0",name:"浦饭玩家",ratingDelta:0},{id:"p1",name:"飞影玩家",ratingDelta:0}]}};
const evidence=[];
test("an unranked MD result still names the winner and explains points separately",()=>{
 const saved=JSON.stringify(room),html=render(room);
 assert.match(html,/浦饭玩家 获胜/);assert.doesNotMatch(html,/练习结束|练习对局/);
 assert.match(html,/不计积分/);assert.equal(JSON.stringify(room),saved);
 evidence.push({case:"winner without rating",html});
});
test("unranked team wins and draws retain the actual outcome",()=>{
 for(const [game,result,expected] of [[{winners:[0,1]},{...room.result,team:true},/浦饭玩家、飞影玩家 获胜/],[{winners:[]},{...room.result,draw:true},/本局和棋|本局和局/]]){
  const html=render({...room,game,result});assert.match(html,expected);assert.doesNotMatch(html,/练习结束/);evidence.push({case:"team or draw",html});
 }
});
test("ordinary rated winners and casual results without winners remain meaningful",()=>{
 const rated=render({...room,result:{...room.result,rated:true,unrated:false}});assert.match(rated,/浦饭玩家 获胜/);
 const noWinner=render({...room,game:{winners:[]}});assert.match(noWinner,/对局结束/);assert.doesNotMatch(noWinner,/> 获胜/);
 evidence.push({case:"rated and no winner",rated,noWinner});
});
test.after(()=>writeFileSync(path.join(out,"result.json"),JSON.stringify({scope:"actual platform settlement component; registry and player-link boundaries isolated",evidence},null,2)+"\n"));
