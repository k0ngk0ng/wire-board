import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, renameSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const digest = data => createHash("sha256").update(data).digest("hex");
// Vite consumes only this explicitly public output. The generic CDN delivery
// stage uploads and strips it with all other client assets before packaging.
export function preparePvZRuntime(directory) {
 const root=realpathSync(directory), pkg="./games/pvz/cmd/browser-wasm";
 const env={...process.env,GOOS:"js",GOARCH:"wasm",CGO_ENABLED:"0",GOWORK:"off",GOTOOLCHAIN:"local",GOENV:"off",GOFLAGS:"",GOEXPERIMENT:""};
 for(const [name,relative] of Object.entries({GOCACHE:".local/go-build",GOMODCACHE:".local/go-mod",GOTMPDIR:".local/tmp",TMPDIR:".local/tmp"})) {env[name] ||=path.join(root,relative);mkdirSync(env[name],{recursive:true});}
 const go=args=>execFileSync("go",args,{cwd:root,env,encoding:"utf8",maxBuffer:16<<20,stdio:["ignore","pipe","pipe"]});
 const toolchain=JSON.parse(go(["env","-json","GOROOT","GOVERSION"]));
 const loader=readFileSync(path.join(toolchain.GOROOT,"lib/wasm/wasm_exec.js"));
 const files=new Set(["go.mod","go.sum"]);
 for(const line of go(["list","-deps","-f",'{{if not .Standard}}{{.Dir}}{{range .GoFiles}}|{{.}}{{end}}{{range .EmbedFiles}}|{{.}}{{end}}{{end}}',pkg]).split("\n").filter(Boolean)) {
  const [dir,...names]=line.split("|");for(const name of names) {const file=realpathSync(path.join(dir,name)),relative=path.relative(root,file);if(relative===".."||relative.startsWith(".."+path.sep)||path.isAbsolute(relative))throw Error("PvZ runtime dependency outside checkout");files.add(relative.split(path.sep).join("/"));}
 }
 const inputs=[...files].sort().map(file=>({path:file,sha256:digest(readFileSync(path.join(root,file)))}));
 const source={format:1,package:pkg,goVersion:toolchain.GOVERSION,sourceSHA256:digest(JSON.stringify(inputs)),loaderSHA256:digest(loader),builderSHA256:digest(readFileSync(fileURLToPath(import.meta.url)))};
 const parent=path.join(root,".local/pvz/runtime-build");mkdirSync(parent,{recursive:true});const cache=path.join(parent,digest(JSON.stringify(source))),publicRoot=path.join(cache,"public"),target=path.join(publicRoot,"pvz/v1/runtime");
 const valid=()=>{try{const m=JSON.parse(readFileSync(path.join(target,"build.json"),"utf8"));if(m.sourceSHA256!==source.sourceSHA256||m.builderSHA256!==source.builderSHA256||m.goVersion!==source.goVersion||m.loaderSHA256!==source.loaderSHA256)return false;for(const file of ["pvz.wasm","wasm_exec.js"]){const b=readFileSync(path.join(target,file));if(m.files[file]?.sha256!==digest(b)||m.files[file]?.bytes!==b.length)return false;if(file.endsWith(".wasm")&&!WebAssembly.validate(b))return false;}return true;}catch{return false;}};
 if(valid())return publicRoot;
 const temporary=mkdtempSync(path.join(parent,"compile-"));
 try {const output=path.join(temporary,"public/pvz/v1/runtime");mkdirSync(output,{recursive:true});go(["build","-trimpath","-buildvcs=false","-ldflags=-s -w","-o",path.join(output,"pvz.wasm"),pkg]);const wasm=readFileSync(path.join(output,"pvz.wasm"));if(!WebAssembly.validate(wasm))throw Error("Invalid PvZ Wasm");writeFileSync(path.join(output,"wasm_exec.js"),loader);writeFileSync(path.join(output,"build.json"),JSON.stringify({...source,inputs,files:{"pvz.wasm":{sha256:digest(wasm),bytes:wasm.length},"wasm_exec.js":{sha256:digest(loader),bytes:loader.length}}},null,2)+"\n");if(valid())return publicRoot;if(existsSync(cache))rmSync(cache,{recursive:true,force:true});renameSync(temporary,cache);if(!valid())throw Error("PvZ runtime verification failed");return publicRoot;}
 finally {rmSync(temporary,{recursive:true,force:true});}
}
