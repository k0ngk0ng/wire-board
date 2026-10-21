import { rasterizePostResult, rasterizePostResultRows, unpackPostResultRasterRows } from "../../games/basketball/web/postresult-raster";
import { loadBasketballWasm } from "../../games/basketball/web/wasm";
import { BasketballRenderer, loadBasketballAssetsWithFallback } from "../../games/basketball/web/renderer";

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
  const engine=fixture.wasm?await loadBasketballWasm("./runtime"):undefined;
  const production=fixture.renderer?await loadBasketballAssetsWithFallback(fixture.assetsBase,fixture.assetsBase,{postResult:true}):undefined;
  if(fixture.renderer&&!production?.postResult)throw Error("post-result artwork was not preloaded");
  const results=[];
  let lastRows:any;
  for(const item of fixture.cases??[fixture]) {
    let frame=item.frame,packedBytes:number|undefined;
    if(engine){
      const packed=engine.request<any>({op:"projectPresentation",projection:item.projection});
      packedBytes=atob(packed.rows).length;
      frame=unpackPostResultRasterRows(packed,frame.oam);
      let rejected=false;try{unpackPostResultRasterRows({...packed,rows:packed.rows.slice(0,-8)},frame.oam)}catch{rejected=true}
      if(!rejected)throw Error("truncated projection accepted");
    }
    const expected=await pixels(item.expected);
    const figure=document.createElement("figure"),caption=document.createElement("figcaption"),canvas=document.createElement("canvas");
    caption.textContent=item.name??"Final picture";canvas.width=256;canvas.height=240;
    let actual:Uint8ClampedArray;
    if(production){
      const renderer=new BasketballRenderer(canvas,production,projection=>engine!.request({op:"projectPresentation",projection}));
      renderer.render({postResult:{...item.projection,oam:frame.oam}} as any);
      actual=canvas.getContext("2d")!.getImageData(0,0,256,240).data;
    }else{
      actual=render(assets,frame);canvas.getContext("2d")!.putImageData(new ImageData(actual,256,240),0,0);
    }
    figure.append(caption,canvas);document.querySelector("main")!.append(figure);
    let different=0;const first:number[]=[];
    for(let i=0;i<actual.length;i+=4)if(actual.slice(i,i+4).some((v,j)=>v!==expected.data[i+j])){different++;if(first.length<10)first.push(i/4);}
    const bad=structuredClone(frame);
    if(fixture.rows)bad.rows[0].segments[0].page=255;else bad.bands[0].backgroundCHR[0]=255;
    let rejectsMissingPage=false;try{render(assets,bad)}catch{rejectsMissingPage=true}
    results.push({name:item.name,different,first,pixels:61440,rejectsMissingPage,packedBytes});
    if(fixture.rows)lastRows=frame;
  }
  let uniformTransition=false,disabledTransition=false,visibleMissingRejected=false;
  if(lastRows){
    const hidden=structuredClone(lastRows);hidden.palette.fill(15);
    for(const row of hidden.rows){row.spriteCHR=[112,113,114,115];for(const segment of row.segments)segment.page=112;}
    const dark=rasterizePostResultRows(assets,hidden),black=assets.rgb[15];
    uniformTransition=dark.every((v,i)=>v===(i%4===3?255:black[i%4]));
    hidden.palette[1]=33;
    try{rasterizePostResultRows(assets,hidden)}catch{visibleMissingRejected=true}
    hidden.palette[0]=33;
    for(const row of hidden.rows){row.mask=0;for(const segment of row.segments)segment.mask=0;}
    const disabled=rasterizePostResultRows(assets,hidden),backdrop=assets.rgb[33];
    disabledTransition=disabled.every((v,i)=>v===(i%4===3?255:backdrop[i%4]));
    if(!uniformTransition||!disabledTransition||!visibleMissingRejected)throw Error("invalid hidden-resource handling");
  }
  (window as any).result={cases:results,different:results.reduce((n,r)=>n+r.different,0),pixels:results.length*61440,rejectsMissingPage:results.every(r=>r.rejectsMissingPage),uniformTransition,disabledTransition,visibleMissingRejected,scope:"actual original source-frame references plus declared hidden-resource cases; not live ending playback"};
})().catch(error=>{(window as any).result={error:String(error)}});
