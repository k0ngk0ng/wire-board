import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
const root=fileURLToPath(new URL("../../",import.meta.url));
const output=path.join(root,".local/yuyuhakusho/resident-assets/lint");
mkdirSync(output,{recursive:true});
const inspect=rows=>JSON.parse(execFileSync("python3",["-B","scripts/ci/resident_source_lint.py","--stdin"],{cwd:root,input:JSON.stringify(rows),encoding:"utf8"}));
test("resident source lint enforces Go and Python naming, paths, comments and prohibited execution",()=>{
 const rows=[
  {file:"internal/server/resident_assets.go",source:"package server\nvar resourceCount = 1",passed:true},
  {file:"internal/server/resident_assets.go",source:"package server\nvar resource_count = 1",passed:false},
  {file:"internal/server/resident_assets.go",source:"package server\n// forbidden\nvar resourceCount = 1",passed:false},
  {file:"internal/server/Outside.go",source:"package server",passed:false},
  {file:"scripts/asset_reuse.py",source:"def valid_name():\n return 1\n",passed:true},
  {file:"scripts/asset_reuse.py",source:"def badName():\n pass\n",passed:false},
  {file:"scripts/asset_reuse.py",source:"pass # forbidden\n",passed:false},
  {file:"scripts/asset_reuse.py",source:"eval('1')",passed:false},
  {file:"../outside.py",source:"pass",passed:false},
 ];
 const result=inspect(rows);
 for(const[index,row]of rows.entries())assert.equal(result.files[index].passed,row.passed,JSON.stringify(result.files[index]));
});
test("resident source lint checks its registered Go and publication Python sources",()=>{
 const report=JSON.parse(execFileSync("python3",["-B","scripts/ci/resident_source_lint.py"],{cwd:root,encoding:"utf8"}));
 writeFileSync(path.join(output,"result.json"),JSON.stringify(report,null,2)+'\n');
 assert.equal(report.passed,true,JSON.stringify(report));
});

test("the unchanged legacy gzip parser is bounded without Git history and cannot hide new floating comparisons",()=>{
 const file="internal/server/static_delivery.go",source=readFileSync(path.join(root,file),"utf8");
 assert.equal(inspect([{file,source}]).passed,true);
 assert.equal(inspect([{file,source:source.replace("quality > 0","quality > 1")}]).passed,false);
 assert.equal(inspect([{file,source:source+"\nfunc invalidBoundary(value float64) bool { return value > 0.5 }\n"}]).passed,false);
});
