// Render the independent scene functions against the retained original oracle.
// This produces verification images, not a playable game or a video substitute.
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { resolve, relative, sep } from "node:path";
import { gunzipSync } from "node:zlib";
import { execFileSync } from "node:child_process";
import { decodeChrPage, rasterizeSpriteOAM } from "../../games/contra/web/sprites.ts";
import { rasterizeBackground, composeScene } from "../../games/contra/web/background.ts";

const root=resolve(import.meta.dirname,"../.."),out=resolve(root,".local/contra-scene-proof");
if(relative(root,out).startsWith(`..${sep}`))throw Error("Output must remain in workspace");
mkdirSync(out,{recursive:true});
const fixture=JSON.parse(gunzipSync(readFileSync(resolve(root,"games/contra/tests/testdata/background-original.json.gz"))));
const bytes=hex=>Uint8Array.from(Buffer.from(hex,"hex"));
const pages=new Map(Object.entries(fixture.pages).map(([id,hex])=>[id,decodeChrPage(bytes(hex))]));
for(const frame of [1,120,320]){
  const sample=fixture.samples.find(s=>s.frame===frame);
  const maps=sample.screens.map(s=>({tiles:bytes(s.tiles),attributes:bytes(s.attributes)}));
  const options={pages,mapping:sample.mapping,palette:bytes(sample.palette),rgb:bytes(fixture.rgb),control:sample.control,mask:sample.mask};
  const background=rasterizeBackground({...options,screens:[maps[0],maps[1],maps[0],maps[1]],scrollX:sample.scrollX,scrollY:sample.scrollY});
  const sprites=rasterizeSpriteOAM(bytes(sample.oam),options);
  writeFileSync(resolve(out,`recovered-${frame}.rgba`),composeScene(background,sprites));
  writeFileSync(resolve(out,`original-${frame}.rgb`),Buffer.from(sample.expectedRGB,"base64"));
}
execFileSync(resolve(root,".local/board-art-env/bin/python"),["-B","-c",`
from pathlib import Path
from PIL import Image, ImageDraw
root=Path.cwd();base=root/'.local/contra-scene-proof'
image=Image.new('RGB',(768,528),(32,32,32));draw=ImageDraw.Draw(image)
for col,frame in enumerate([1,120,320]):
 original=Image.frombytes('RGB',(256,240),(base/f'original-{frame}.rgb').read_bytes())
 recovered=Image.frombytes('RGBA',(256,240),(base/f'recovered-{frame}.rgba').read_bytes()).convert('RGB')
 assert original.tobytes()==recovered.tobytes()
 draw.text((col*256,4),f'Original {frame}',fill='white');image.paste(original,(col*256,24))
 draw.text((col*256,268),f'Independent renderer {frame}',fill='white');image.paste(recovered,(col*256,288))
image.save(root/'docs/research/nes-contra/evidence/scene-comparison.png')
for p in base.iterdir():p.unlink()
base.rmdir()
`],{cwd:root,stdio:"inherit"});
process.stdout.write("scene-comparison.png: all displayed pixels match original\n");
