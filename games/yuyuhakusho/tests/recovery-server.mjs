import { createRequire } from "node:module";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { createReadStream, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { Readable } from "node:stream";
import path from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const resources = path.resolve(process.argv[2] ?? "");
const terminal = process.argv.includes("--terminal");
const basketball = process.argv.includes("--basketball");
const observe = process.argv.includes("--observe-resources");
const resourceRequests = [];
if (!process.argv[2] || path.relative(root, resources).startsWith(".."))
  throw Error("Provide the prepared asset directory inside this workspace");
const temporary = path.join(
  root,
  basketball ? ".local/basketball/resident-fixture/temporary" : terminal ? ".local/yuyuhakusho/recovery-loop/terminal-fixture-temporary" : ".local/yuyuhakusho/recovery-loop/fixture-temporary",
);
for (const folder of [
  temporary,
  path.join(temporary, "data"),
  path.join(temporary, "tmp"),
  path.join(temporary, "go-cache"),
  path.join(temporary, "go-mod"),
])
  await mkdir(folder, { recursive: true });
const env = {
  ...process.env,
  GOCACHE: path.join(temporary, "go-cache"),
  GOMODCACHE: path.join(temporary, "go-mod"),
  GOTMPDIR: path.join(temporary, "tmp"),
  GOTELEMETRY: "off",
  WIRE_BOARD_TEST_DATA: path.join(temporary, "data"),
};
const source = `package main
import (
 "fmt"
 "net"
 "net/http"
 "os"
 "os/signal"
 "syscall"
 ${terminal || basketball ? '"encoding/json"' : ""}
 ${basketball ? 'basketball "github.com/k0ngk0ng/wire-board/games/basketball/server"' : 'yuyu "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"'}
 "github.com/k0ngk0ng/wire-board/internal/gameapi"
 "github.com/k0ngk0ng/wire-board/internal/server"
)
func main(){
 module:=${basketball ? "basketball" : "yuyu"}.Module()
 ${basketball ? `controls:=new(gameapi.ControllerDefinition);controlData,controlErr:=os.ReadFile("games/basketball/controls.json");if controlErr!=nil{panic(controlErr)};if controlErr=json.Unmarshal(controlData,controls);controlErr!=nil{panic(controlErr)};module.Controls=controls` : ""}
 ${terminal ? `original:=module.Start
 module.Start=func(ctx gameapi.Context,raw json.RawMessage)(gameapi.Match,error){
  match,err:=original(ctx,raw);if err!=nil{return match,err}
  var state yuyu.State;if err=json.Unmarshal(match.Data,&state);err!=nil{return match,err}
  prior,err:=os.ReadFile("games/yuyuhakusho/verification/client-terminal-preview/sample-legacy-four/prior-checkpoint.json");if err!=nil{return match,err}
  var checkpoint yuyu.Checkpoint;if err=json.Unmarshal(prior,&checkpoint);err!=nil{return match,err}
  state.Frame=checkpoint.Frame;state.World=checkpoint.World;state.Presentation=checkpoint.Presentation;state.Loading=nil
  match.Data,err=json.Marshal(state);match.SystemDeadline=ctx.Now.UnixMilli()+50
  return match,err
 }` : ""}
 app,err:=server.New(server.Config{ClientLoading:true,Modules:[]gameapi.Module{module},DataDir:os.Getenv("WIRE_BOARD_TEST_DATA"),InviteCode:"gamepad-browser",AssetsBaseURL:"https://assets.example.test/bga"},os.DirFS("web/dist"));if err!=nil{panic(err)};defer app.Close()
 listener,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{panic(err)}
 fmt.Println("http://"+listener.Addr().String())
 service:=&http.Server{Handler:app.Handler()};go service.Serve(listener)
 stop:=make(chan os.Signal,1);signal.Notify(stop,os.Interrupt,syscall.SIGTERM);<-stop;service.Close()
}`;
await writeFile(path.join(temporary, "fixture.go"), source);
await promisify(execFile)(
  "go",
  [
    "build",
    "-o",
    path.join(temporary, "fixture"),
    path.join(temporary, "fixture.go"),
  ],
  { cwd: root, env },
);
const backend = spawn(path.join(temporary, "fixture"), [], {
  cwd: root,
  env,
  stdio: ["ignore", "pipe", "inherit"],
});
const base = await new Promise((resolve, reject) => {
  let output = "";
  backend.once("error", reject);
  backend.once("exit", (code) => reject(Error("Fixture exited: " + code)));
  backend.stdout.on("data", (data) => {
    output += data;
    if (output.includes("\n")) resolve(output.split("\n")[0]);
  });
});
const require = createRequire(
  new URL("../../../web/package.json", import.meta.url),
);
const manifest = JSON.parse(
  await readFile(path.join(root, "web/dist/game-load-manifest.json"), "utf8"),
);
const business = JSON.parse(await readFile(path.join(root,".local/business.json"),"utf8"));
manifest.assetBase = (process.env.CDN_ORIGIN || business.CDN_ORIGIN).replace(/\/$/, "") + "/bga/";
const external = JSON.parse(await readFile(path.join(root,"assets/external-resources.json"),"utf8"));
for (const [file,record] of Object.entries(external.files)) manifest.media[file] ??= record.bytes;
const basketballRelease = basketball ? JSON.parse(await readFile(path.join(root,"games/basketball/assets/release-manifest.json"),"utf8")) : undefined;
const plugin = {
  transform(source,id) {
    if (!observe || new URLSearchParams(id.split("?")[1] || "").has("url") || !id.split("?")[0].endsWith("/game-api/resident-worker.ts")) return;
    return `const observedFetch=globalThis.fetch.bind(globalThis);globalThis.fetch=async(input,options)=>{const target=new URL(typeof input==="string"?input:input.url,self.location.href);if(!target.pathname.startsWith("/api/")&&!target.pathname.startsWith("/__resident-"))await observedFetch("/__resident-observe",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({url:target.href})});return observedFetch(input,options)};\n`+source;
  },
  name: "recovery-fixture-assets",
  configureServer(server) {
    server.middlewares.use((request, response, next) => {
      if (request.url === "/__resident-observe" && request.method === "POST") {
        let body="";request.on("data",chunk=>body+=chunk);request.on("end",()=>{resourceRequests.push(JSON.parse(body));response.writeHead(204);response.end()});return;
      }
      if (request.url === "/__resident-stats") { response.setHeader("Content-Type","application/json");response.end(JSON.stringify({count:resourceRequests.length,requests:resourceRequests}));return; }
      if (basketball && (request.url.startsWith("/bga/basketball/") || request.url.startsWith("/basketball/"))) {
        const logical=new URL(request.url,"http://127.0.0.1").pathname.replace(/^\/bga/,"");
        const record=basketballRelease.files[logical];
        if(record){void fetch(record.url).then(source=>{response.statusCode=source.status;response.setHeader("Content-Type",source.headers.get("Content-Type")||"application/octet-stream");if(!source.headers.get("Content-Encoding")&&source.headers.get("Content-Length"))response.setHeader("Content-Length",source.headers.get("Content-Length"));Readable.fromWeb(source.body).pipe(response)}).catch(()=>{response.statusCode=502;response.end()});return;}
      }
      if (request.url === "/api/version") {
        response.setHeader("Content-Type", "application/json");
        response.end(JSON.stringify({ version: "dev", assets: [] }));
        return;
      }
      if (request.url === "/game-load-manifest.json") {
        response.setHeader("Content-Type", "application/json");
        response.end(
          JSON.stringify({
            ...manifest,
            entry: "/src/main.tsx",
            home: { files: [], bytes: 0, modules: [] },
            games: Object.fromEntries(
              Object.keys(manifest.games).map((kind) => [kind, []]),
            ),
          }),
        );
        return;
      }
      if (
        (request.url?.startsWith("/bga/yuyuhakusho/") ||
          request.url?.startsWith("/yuyuhakusho/") ||
          basketball && (request.url?.startsWith("/basketball/") || request.url?.startsWith("/bga/basketball/"))) &&
        !request.url.includes("..")
      ) {
        const relative = decodeURIComponent(
          request.url
            .split("?")[0]
            .slice(request.url.startsWith("/bga/") ? 5 : 1),
        );
        const file = path.join(resources, relative);
        if (!existsSync(file)) {
          response.statusCode = 404;
          response.end();
          return;
        }
        const mime = {
          ".json": "application/json",
          ".wasm": "application/wasm",
          ".js": "application/javascript",
          ".gz": "application/gzip",
          ".webp": "image/webp",
          ".png": "image/png",
          ".ogg": "audio/ogg",
        };
        response.setHeader(
          "Content-Type",
          mime[path.extname(file)] ?? "application/octet-stream",
        );
        createReadStream(file).pipe(response);
        return;
      }
      next();
    });
  },
};
const terminationHandlers = process.listeners("SIGTERM");
const inputEndHandlers = process.stdin.listeners("end");
process.once("exit", () => {
  if (backend.exitCode === null) backend.kill("SIGTERM");
});
const server = await require("vite").createServer({
  configFile: false,
  root: path.join(root, "web"),
  publicDir: false,
  cacheDir: path.join(temporary, "vite-cache"),
  plugins: [
    require("@vitejs/plugin-react").default({ include: /\.[jt]sx$/ }),
    plugin,
  ],
  resolve: {
    dedupe: ["react", "react-dom", "lucide-react", "three", "fflate"],
  },
  server: {
    headers: { "Service-Worker-Allowed": "/" },
    host: "127.0.0.1",
    port: 0,
    hmr: false,
    proxy: { "/api": { target: base, ws: true } },
    fs: { allow: [root] },
  },
});
for (const handler of process.listeners("SIGTERM"))
  if (!terminationHandlers.includes(handler)) process.off("SIGTERM", handler);
for (const handler of process.stdin.listeners("end"))
  if (!inputEndHandlers.includes(handler)) process.stdin.off("end", handler);
await server.listen();
console.log(server.resolvedUrls.local[0]);
await new Promise((resolve) => {
  process.once("SIGINT", resolve);
  process.once("SIGTERM", resolve);
});
await server.close();
backend.kill("SIGTERM");
await new Promise((resolve) => backend.once("exit", resolve));
await promisify(execFile)("go", ["clean", "-modcache"], { cwd: root, env });
await rm(temporary, { recursive: true, force: true });
