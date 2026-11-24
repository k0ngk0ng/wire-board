import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";
import { mkdir, writeFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
const root=fileURLToPath(new URL("../../",import.meta.url));
const out=path.join(root,".local/stoneage/retirement");
const temporary=path.join(out,"frontend-temporary");
await mkdir(temporary,{recursive:true});
const require=createRequire(new URL("../package.json",import.meta.url));
test("retired games leave the actual frontend catalog and runtime while historical names remain readable",async()=>{
 const server=await require("vite").createServer({configFile:false,root:path.join(root,"web"),cacheDir:path.join(temporary,"vite"),publicDir:false,esbuild:{jsx:"automatic"},optimizeDeps:{noDiscovery:true},resolve:{dedupe:["react","react-dom","lucide-react","three","fflate"]},server:{middlewareMode:true,hmr:false,fs:{allow:[root]}}});
 const report={passed:false,removed:["stoneage","spf2t","escape"],remaining:[]};
 try {
  const registry=await server.ssrLoadModule("/src/game-api/registry.ts");
  for(const [kind,name] of [["stoneage","石器竞技场"],["spf2t","街霸方块"],["escape","密室逃脱"]]) {
   assert.equal(registry.gameModule(kind),undefined,kind);
   assert.equal(registry.isIndependentGame(kind),false,kind);
   await assert.rejects(registry.loadGame(kind),/未找到游戏界面/);
   assert.equal(registry.gameName(kind),name);
  }
  for(const kind of ["gomoku","yuyuhakusho","cs16"]){assert.ok(registry.gameModule(kind));report.remaining.push(kind)}
  report.passed=true;
 }catch(error){report.error=String(error);throw error}
 finally{await writeFile(path.join(out,"frontend-result.json"),JSON.stringify(report,null,2)+"\n");await server.close();await rm(temporary,{recursive:true,force:true})}
});
