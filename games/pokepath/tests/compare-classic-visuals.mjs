import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { createHash } from "node:crypto";
const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = join(root, ".local/pokepath/reference-visual");
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: join(root, ".local/pokepath/classic-fixture/profile"), AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", AGENT_BROWSER_SCREENSHOT_DIR: output, AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const session = execFileSync("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "ppvis"], { cwd: root, env: environment, encoding: "utf8" }).trim();
const regions = { scene: [0, 0, 1238, 674], trainer: [0, 0, 259, 674], central: [259, 25, 979, 649], monitor: [979, 0, 1238, 674] };
const report = { command: "node games/pokepath/tests/compare-classic-visuals.mjs", scope: "Raw same-size screenshots; animation and differing live state are deliberately not hidden", measured: true, pixelEquivalent: true, cases: [] };
for (const panel of ["initial", "profile", "inventory", "routes", "dex", "settings"]) {
  const paths = [`stable-${panel}-chinese.png`, `native-${panel}.png`];
  const sources = paths.map(path => readFileSync(join(output, path)));
  const code = `(async()=>{const sources=${JSON.stringify(sources.map(value => "data:image/png;base64," + value.toString("base64")))};const images=await Promise.all(sources.map(src=>new Promise((resolve,reject)=>{const image=new Image();image.onload=()=>resolve(image);image.onerror=reject;image.src=src})));if(images.some(image=>image.width!==1238||image.height!==674))throw Error('Incompatible screenshot geometry');const canvases=images.map(image=>{const canvas=document.createElement('canvas');canvas.width=1238;canvas.height=674;canvas.getContext('2d').drawImage(image,0,0);return canvas});const buffers=canvases.map(canvas=>canvas.getContext('2d').getImageData(0,0,1238,674));const difference=new ImageData(1238,674);const metrics={};for(const [name,[left,top,right,bottom]]of Object.entries(${JSON.stringify(regions)})){let changed=0,visible=0,error=0;for(let y=top;y<bottom;y++)for(let x=left;x<right;x++){const index=(y*1238+x)*4;let largest=0;for(let channel=0;channel<3;channel++){const delta=Math.abs(buffers[0].data[index+channel]-buffers[1].data[index+channel]);largest=Math.max(largest,delta);error+=delta;difference.data[index+channel]=Math.min(255,delta*4)}difference.data[index+3]=255;if(largest>0)changed++;if(largest>16)visible++}const pixels=(right-left)*(bottom-top);metrics[name]={pixels,changedPixels:changed,changedPercent:Math.round(changed/pixels*1e7)/1e5,over16Percent:Math.round(visible/pixels*1e7)/1e5,meanAbsoluteChannelError:Math.round(error/pixels/3*1e6)/1e6}}canvases[0].getContext('2d').putImageData(difference,0,0);return{metrics,difference:canvases[0].toDataURL('image/png')}})()`;
  const result = JSON.parse(execFileSync("agent-browser", ["--session", session, "eval", "--stdin", "--json"], { input: code, cwd: root, env: environment, encoding: "utf8", timeout: 45000, maxBuffer: 8 << 20 }));
  if (!result.success) throw Error(JSON.stringify(result));
  const value = result.data.result;
  const difference = `diff-${panel}.png`;
  writeFileSync(join(output, difference), Buffer.from(value.difference.split(",")[1], "base64"));
  report.pixelEquivalent &&= value.metrics.scene.changedPixels === 0;
  report.cases.push({ panel, reference: paths[0], actual: paths[1], sha256: sources.map(value => createHash("sha256").update(value).digest("hex")), difference, metrics: value.metrics });
}
writeFileSync(join(output, "pixel-comparison.json"), JSON.stringify(report, null, 2) + "\n");
process.stdout.write(JSON.stringify({ measured: true, pixelEquivalent: report.pixelEquivalent, cases: report.cases.map(value => ({ panel: value.panel, ...value.metrics.scene })) }) + "\n");
