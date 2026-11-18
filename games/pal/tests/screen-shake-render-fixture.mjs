import fs from "node:fs/promises";
import { createRequire } from "node:module";

const root = new URL("../../../", import.meta.url);
const out = new URL(process.argv.includes("--game-over") ? ".local/pal/game-over-browser/" : ".local/pal/screen-shake-browser/", root);
await fs.mkdir(out, { recursive: true });
const { build } = createRequire(new URL("web/package.json", root))("esbuild");
const source = `import React,{useRef,useEffect} from "react";
import {createRoot} from "react-dom/client";
import {PalStage} from "./games/pal/web/Stage";
import {SceneRenderer} from "./games/pal/web/scene-renderer";
const renderer=new SceneRenderer("/pal/v1");
function App(){const canvas=useRef(null);useEffect(()=>{window.paintShake=async(index)=>{const pages=(await(await fetch('/authority.json')).json()).pages,value=pages[index].view;await renderer.prepare(value);await renderer.paint(canvas.current,value);return true};window.shakeReady=true},[]);return <div style={{width:"100vw",height:"100dvh",display:"grid"}}><PalStage canvasRef={canvas}/></div>}
createRoot(document.getElementById("root")).render(<App/>);`;
await build({ stdin: { contents: source, loader: "tsx", resolveDir: root.pathname }, absWorkingDir: root.pathname, nodePaths: [new URL("web/node_modules", root).pathname], bundle: true, outfile: new URL("fixture.js", out).pathname, format: "esm", jsx: "automatic", loader: { ".css": "css" }, sourcemap: false });
await fs.writeFile(new URL("index.html", out), '<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><style>html,body,#root{margin:0;width:100%;height:100%;overflow:hidden}</style></head><body><div id="root"></div><script type="module" src="/fixture.js"></script></body></html>');
