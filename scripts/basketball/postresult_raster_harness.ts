import { rasterizePostResult, rasterizePostResultRows } from "../../games/basketball/web/postresult-raster";

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
  const assets={pages:manifest.indexAtlas.pages,indices:atlas.data,rgb:manifest.rgbPalette};
  const render=fixture.rows?rasterizePostResultRows:rasterizePostResult;
  const results=[];
  for(const item of fixture.cases??[fixture]) {
    const expected=await pixels(item.expected),actual=render(assets,item.frame);
    const figure=document.createElement("figure"),caption=document.createElement("figcaption"),canvas=document.createElement("canvas");
    caption.textContent=item.name??"Final picture";canvas.width=256;canvas.height=240;
    canvas.getContext("2d")!.putImageData(new ImageData(actual,256,240),0,0);
    figure.append(caption,canvas);document.querySelector("main")!.append(figure);
    let different=0;const first:number[]=[];
    for(let i=0;i<actual.length;i+=4)if(actual.slice(i,i+4).some((v,j)=>v!==expected.data[i+j])){different++;if(first.length<10)first.push(i/4);}
    const bad=structuredClone(item.frame);
    if(fixture.rows)bad.rows[0].segments[0].page=255;else bad.bands[0].backgroundCHR[0]=255;
    let rejectsMissingPage=false;try{render(assets,bad)}catch{rejectsMissingPage=true}
    results.push({name:item.name,different,first,pixels:61440,rejectsMissingPage});
  }
  (window as any).result={cases:results,different:results.reduce((n,r)=>n+r.different,0),pixels:results.length*61440,rejectsMissingPage:results.every(r=>r.rejectsMissingPage),scope:"actual original source-frame references; not live ending playback"};
})().catch(error=>{(window as any).result={error:String(error)}});
