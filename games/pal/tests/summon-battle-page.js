(() => {
  const palette=async()=>fetch("/pal/v1/palettes.json").then(response=>response.json()).then(data=>data[0][0]);
  let run, witnesses;
  const header=()=>document.querySelector("[data-pal-board]");
  const makeWitnesses=async(group,keys,colors)=>{
    const id=keys[0][0], base="/pal/v1/atlases/"+group+"/"+id;
    const [atlas,image]=await Promise.all([
      fetch(base+".json").then(response=>response.json()),
      fetch(base+".png").then(response=>response.blob()).then(blob=>createImageBitmap(blob,{colorSpaceConversion:"none"})),
    ]);
    const buffer=new OffscreenCanvas(atlas.width,atlas.height).getContext("2d",{willReadFrequently:true});
    buffer.drawImage(image,0,0);
    image.close();
    return keys.map(([source,number,left,top])=>{
      if(source!==id)throw Error("mixed summon atlases");
      const rect=atlas.frames[number], data=buffer.getImageData(rect.x,rect.y,rect.width,rect.height).data;
      const points=[];
      for(let y=0;y<rect.height;y++)for(let x=0;x<rect.width;x++){
        const p=(y*rect.width+x)*4, ax=x+left, ay=y+top;
        if(data[p+3]!==255||ax<0||ax>=320||ay<0||ay>=160)continue;
        const [r,g,b]=colors[data[p]];
        points.push([ay*320+ax,(0xff000000|b<<16|g<<8|r)>>>0]);
      }
      if(points.length<12)throw Error("summon effect has too few visible witness pixels");
      return {number,points:points.filter((_,i)=>i%Math.max(1,Math.floor(points.length/256))===0)};
    });
  };
  const classify=(pixels,markers)=>{
    const matches=markers.map(({number,points})=>[number,Math.round(1000*points.reduce((n,[i,color])=>n+(pixels[i]===color),0)/points.length)]);
    matches.sort((a,b)=>b[1]-a[1]);
    const [id,score]=matches[0];
    return {id:score>=840&&score-matches[1][1]>=55?id:null,score,second:matches[1][1]};
  };
  window.palSummonBattle={
    async prepare(sprites,effects){
      const colors=await palette();
      witnesses=await Promise.all([makeWitnesses("f",sprites,colors),makeWitnesses("fire",effects,colors)]);
      return witnesses.map(group=>group.map(marker=>[marker.number,marker.points.length]));
    },
    start(){
      if(!witnesses||header()?.dataset.palReady!=="true")throw Error("summon probe is not ready");
      run={wall:performance.timeOrigin,start:performance.now(),samples:[],inputs:[]};
    },
    stop(){const observed=run;run=undefined;return observed},
  };
  function record(event){
    if(!run)return;
    run.inputs.push({at:performance.now(),type:event.type,code:event.code,pointer:event.pointerType,trusted:event.isTrusted,save:event.target instanceof Element&&!!event.target.closest("[data-save-game]")});
  }
  for(const type of ["keydown","pointerdown","pointerup"])window.addEventListener(type,record,true);
  function tick(){
    if(run){
      const at=performance.now(),board=header(), canvas=board?.querySelector("canvas");
      const row={at,ready:board?.dataset.palReady==="true",phase:board?.dataset.palPhase,focused:document.hasFocus()&&!document.hidden};
      if(canvas&&row.ready){
        const pixels=new Uint32Array(canvas.getContext("2d").getImageData(0,0,320,200).data.buffer);
        let hash=2166136261;
        for(const value of pixels)hash=Math.imul(hash^value,16777619);
        row.hash=hash>>>0;
        [row.sprite,row.effect]=witnesses.map(markers=>classify(pixels,markers));
      }
      run.samples.push(row);
    }
    requestAnimationFrame(tick);
  }
  requestAnimationFrame(tick);
})();
