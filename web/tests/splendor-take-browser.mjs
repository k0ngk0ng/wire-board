import assert from "node:assert/strict";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";
import { createHash } from "node:crypto";

const root=fileURLToPath(new URL("../../",import.meta.url));
const engine=process.env.BROWSER_ENGINE||"webkit";
const name=process.env.GEM_TAKE_REPORT||engine;
assert.match(name,/^[a-z0-9-]+$/);
const out=path.join(root,".local/splendor/fullscreen-take",name);
const temporary=path.join(out,"temporary");
await mkdir(path.join(temporary,"data"),{recursive:true});
await mkdir(path.join(temporary,"tmp"),{recursive:true});
process.env.TMPDIR=path.join(temporary,"tmp");
const env={...process.env,GOCACHE:path.join(root,".local/go-build"),GOMODCACHE:path.join(root,".local/go-mod"),GOTMPDIR:path.join(temporary,"tmp"),GOPROXY:"off",GOTELEMETRY:"off"};
for(const directory of [env.GOCACHE,env.GOMODCACHE,env.GOTMPDIR])await mkdir(directory,{recursive:true});
const business=JSON.parse(await readFile(path.join(root,".local/business.json"),"utf8"));
const assets=business.CDN_ORIGIN.replace(/\/$/,"")+"/bga";
const source=`package main
import("fmt";"net";"net/http";"os";"os/signal";"syscall";"github.com/k0ngk0ng/wire-board/internal/server")
func main(){app,err:=server.New(server.Config{Version:"dev",ClientLoading:true,DataDir:os.Args[1],InviteCode:"gem-click-local",AssetsBaseURL:${JSON.stringify(assets)}},os.DirFS("web/dist"));if err!=nil{panic(err)};defer app.Close();listener,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{panic(err)};fmt.Println("http://"+listener.Addr().String());service:=&http.Server{Handler:app.Handler()};go service.Serve(listener);stop:=make(chan os.Signal,1);signal.Notify(stop,os.Interrupt,syscall.SIGTERM);<-stop;service.Close()}`;
const goFile=path.join(temporary,"fixture.go"),binary=path.join(temporary,"fixture");
await writeFile(goFile,source);
await promisify(execFile)("go",["build","-o",binary,goFile],{cwd:root,env});
const backend=spawn(binary,[path.join(temporary,"data")],{cwd:root,env,stdio:["ignore","pipe","pipe"]});
let backendLog="";
backend.stderr.on("data",data=>backendLog+=data);
const backendURL=await new Promise((resolve,reject)=>{let text="";backend.stdout.on("data",data=>{text+=data;if(text.includes("\n"))resolve(text.split("\n")[0])});backend.once("exit",code=>reject(Error("fixture exited "+code+backendLog)));backend.once("error",reject)});
const require=createRequire(new URL("../package.json",import.meta.url));
const manifest=JSON.parse(await readFile(path.join(root,"web/dist/game-load-manifest.json"),"utf8"));
const delivery=JSON.parse(await readFile(path.join(root,"web/dist/cdn-assets-manifest.json"),"utf8"));
const servedResources=new Map();
const plugin={name:"splendor-click-manifest",configureServer(server){server.middlewares.use((request,response,next)=>{
 if(request.url==="/game-load-manifest.json"){response.setHeader("Content-Type","application/json");response.end(JSON.stringify({...manifest,entry:"/src/main.tsx",home:{files:[],bytes:0,modules:[]},games:Object.fromEntries(Object.keys(manifest.games).map(kind=>[kind,[]]))}));return}
 if(request.url?.startsWith("/bga/")){
  const logical=new URL(request.url,"http://127.0.0.1").pathname.slice(4),record=delivery.files[logical]??delivery.external?.[logical];
  if(record){let pending=servedResources.get(record.url);if(!pending){pending=(async()=>{const source=await fetch(record.url);if(!source.ok)throw Error("published resource "+source.status);const body=Buffer.from(await source.arrayBuffer());if(body.length!==record.bytes||createHash("sha256").update(body).digest("hex")!==record.sha256)throw Error("published resource differs");return body})();servedResources.set(record.url,pending)}void pending.then(body=>{response.setHeader("Content-Type",record.mime);response.setHeader("Content-Length",body.length);response.setHeader("Cache-Control","public,max-age=31536000,immutable");response.end(body)}).catch(error=>{response.statusCode=502;response.end(String(error))});return}
 }
 if(request.url==="/api/version"){response.setHeader("Content-Type","application/json");response.end(JSON.stringify({version:"dev",assets:[]}));return}next();
})}};
const frontend=await require("vite").createServer({configFile:false,root:path.join(root,"web"),cacheDir:path.join(temporary,"vite"),publicDir:false,esbuild:{jsx:"automatic"},optimizeDeps:{include:["react","react/jsx-runtime","react/jsx-dev-runtime","react-dom/client","lucide-react","three","fflate"]},plugins:[plugin],resolve:{dedupe:["react","react-dom","lucide-react","three","fflate"]},server:{host:"127.0.0.1",port:0,hmr:false,headers:{"Service-Worker-Allowed":"/"},fs:{allow:[root]},proxy:{"/api":{target:backendURL,ws:true},"/bga":backendURL,"/_media":backendURL}}});
await frontend.listen();
const engines=await import(pathToFileURL(process.argv[2]));
const browser=await engines[engine].launch({headless:false,executablePath:process.argv[3]});
const context=await browser.newContext({viewport:{width:1174,height:744},isMobile:true,hasTouch:true});
await context.addInitScript(()=>{Object.defineProperty(HTMLElement.prototype,"requestFullscreen",{configurable:true,value:undefined});localStorage.setItem("wb_sound","off")});
const page=await context.newPage();
const report={passed:false,engine,cases:[],errors:[],sourceCSS:createHash("sha256").update(await readFile(path.join(root,"games/splendor/web/splendor-fullscreen.css"))).digest("hex")};
page.on("pageerror",error=>report.errors.push(String(error)));
const api=(suffix,body)=>page.evaluate(async({suffix,body})=>{const response=await fetch("/api"+suffix,{method:body?"POST":"GET",headers:{"Content-Type":"application/json"},body:body?JSON.stringify(body):undefined});const result=await response.json();if(!response.ok)throw Error(JSON.stringify(result));return result},{suffix,body});
const command=async(type,extra={})=>{const {room}=await api("/state");return api("/rooms/"+room.id,{type,...extra,version:room.version,nonce:crypto.randomUUID()})};
try{
 await page.goto(frontend.resolvedUrls.local[0]);await page.waitForSelector("input");
 await api("/register",{name:"GemTouch"+Date.now().toString().slice(-7),password:"local-gem-click",invite:"gem-click-local"});
 for(const [width,height,capacity=2] of [[896,568],[980,568],[760,500,4],[1174,744],[1024,650,4],[1024,651,4],[980,620],[844,390],[820,1080],[390,844],[320,568]]){
  await page.setViewportSize({width,height});
  await api("/rooms",{kind:"splendor",capacity,name:"宝石实际触屏确认",options:{}});
  for(let seat=1;seat<capacity;seat++)await command("add_bot");await page.reload();
  await page.getByRole("button",{name:"我准备好了",exact:true}).tap();
  await page.getByRole("button",{name:"开始游戏",exact:true}).tap();
  await page.waitForFunction(()=>{const b=document.querySelector('[data-bank-color="0"]');return b&&!b.disabled&&!document.querySelector('[data-game-loading]')},undefined,{timeout:90000});
  if(!await page.locator("[data-room-fullscreen]").count())await page.getByRole("button",{name:"全屏",exact:true}).tap();
  await page.waitForSelector("[data-room-fullscreen]");
  for(const color of [0,1,2])await page.locator('[data-bank-color="'+color+'"]').tap();
  const button=page.locator(".gem-bank .bank-actions .primary");await button.scrollIntoViewIfNeeded();
  const geometry=await page.evaluate(()=>{const b=document.querySelector('.gem-bank .bank-actions .primary'),r=b.getBoundingClientRect(),box=e=>{const q=e.getBoundingClientRect();return {left:q.left,top:q.top,right:q.right,bottom:q.bottom,width:q.width,height:q.height}};const points=[[.5,.5],[.2,.5],[.8,.5],[.5,.25],[.5,.75]].map(([x,y])=>{const px=r.x+r.width*x,py=r.y+r.height*y,hit=document.elementFromPoint(px,py);return {x:px,y:py,buttonOwnsPoint:!!hit&&(hit===b||b.contains(hit)),hit:hit?.className,ancestors:hit?[...function*(e){while(e){yield e.className;e=e.parentElement}}(hit)].slice(0,4):[]}});return {button:box(b),disabled:b.disabled,inert:!!b.closest('[inert]'),bank:box(document.querySelector('.gem-bank')),personal:box(document.querySelector('.personal-area')),points}});
  report.cases.push({width,height,capacity,...geometry});await page.screenshot({path:path.join(out,width+"x"+height+"-selected.png")});
  assert.equal(geometry.disabled,false);assert.equal(geometry.inert,false);
  assert.ok(geometry.points.every(point=>point.buttonOwnsPoint),"take target covered: "+JSON.stringify(geometry));
  const before=(await api("/state")).room;
  await button.tap();
  await page.waitForFunction(async({seat,tokens})=>{const {room}=await(await fetch('/api/state')).json();return [0,1,2].every(color=>room.game.splendor.players[seat].tokens[color]===tokens[color]+1)},{seat:before.you,tokens:before.game.splendor.players[before.you].tokens},{timeout:10000});
  report.cases.at(-1).serverAcceptedThreeGems=true;
  await page.screenshot({path:path.join(out,width+"x"+height+"-accepted.png")});
  await command("close");await command("leave");
 }
 assert.deepEqual(report.errors,[]);report.passed=true;
}catch(error){report.error=String(error);process.exitCode=1;console.error(error);report.text=await page.locator("body").innerText().catch(()=>null);await page.screenshot({path:path.join(out,"failure.png")}).catch(()=>{})}
finally{await writeFile(path.join(out,"result.json"),JSON.stringify(report,null,2)+"\n");await browser.close();await frontend.close();backend.kill("SIGTERM");await new Promise(resolve=>backend.once("exit",resolve));await writeFile(path.join(out,"backend.log"),backendLog);await rm(temporary,{recursive:true,force:true})}
