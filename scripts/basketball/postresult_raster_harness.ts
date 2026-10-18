import { rasterizePostResult } from "../../games/basketball/web/postresult-raster";

async function pixels(url: string) {
  const b=await createImageBitmap(await (await fetch(url)).blob(),{colorSpaceConversion:"none"});
  const c=document.createElement("canvas");c.width=b.width;c.height=b.height;
  const x=c.getContext("2d",{willReadFrequently:true})!;x.drawImage(b,0,0);b.close();
  return x.getImageData(0,0,c.width,c.height);
}
(async()=>{
  const fixture=await (await fetch("fixture.json")).json();
  const manifest=await (await fetch("../../.local/postresult-assets/manifest.json")).json();
  const atlas=await pixels("../../.local/postresult-assets/chr-indices.png");
  const expected=await pixels(fixture.expected);
  const assets={pages:manifest.indexAtlas.pages,indices:atlas.data,rgb:manifest.rgbPalette};
  const actual=rasterizePostResult(assets,fixture.frame);
  const canvas=document.querySelector("canvas")!;canvas.getContext("2d")!.putImageData(new ImageData(actual,256,240),0,0);
  let different=0;const first:number[]=[];
  for(let i=0;i<actual.length;i+=4)if(actual.slice(i,i+4).some((v,j)=>v!==expected.data[i+j])){different++;if(first.length<10)first.push(i/4);}
  const bad={...fixture.frame,bands:[{...fixture.frame.bands[0],backgroundCHR:[255,21,22,23]}]};
  let rejectsMissingPage=false;try{rasterizePostResult(assets,bad)}catch{rejectsMissingPage=true}
  (window as any).result={different,first,pixels:61440,rejectsMissingPage,scope:"actual original final-picture frame; not whole ending playback"};
})().catch(error=>{(window as any).result={error:String(error)}});
