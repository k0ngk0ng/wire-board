(() => {
  let run;
  let witnesses;
  const api = window.palDenseBattle = {
    async prepareEffect(frames) {
      const [atlas,palette,image]=await Promise.all([
        fetch("/pal/v1/atlases/fire/50.json").then(r=>r.json()),
        fetch("/pal/v1/palettes.json").then(r=>r.json()).then(p=>p[0][0]),
        fetch("/pal/v1/atlases/fire/50.png").then(r=>r.blob()).then(b=>createImageBitmap(b,{colorSpaceConversion:"none"})),
      ]);
      const ctx=new OffscreenCanvas(atlas.width,atlas.height).getContext("2d",{willReadFrequently:true});
      ctx.drawImage(image,0,0);
      image.close();
      witnesses=frames.map(([id,number,left,top])=>{
        if(id!==50)throw Error("Missing original effect atlas");
        const r=atlas.frames[number], source=ctx.getImageData(r.x,r.y,r.width,r.height).data;
        const points=[];
        for(let y=0;y<r.height;y++)
          for(let x=0;x<r.width;x++){
            const p=(y*r.width+x)*4;
            if(source[p+3]!==255 || left+x<0 || left+x>=320 || top+y<0 || top+y>=200)continue;
            const [red,green,blue]=palette[source[p]];
            points.push([(top+y)*320+left+x,(red | green<<8 | blue<<16 | 0xff000000)>>>0]);
          }
        if(points.length<8)throw Error("Too few original effect pixels");
        return {number,points:points.filter((_,i)=>i%Math.max(1,Math.floor(points.length/192))===0)};
      });
      return witnesses.map(v=>[v.number,v.points.length]);
    },
    begin(canvas) {
      if (!canvas || canvas.width !== 320 || canvas.height !== 200 || !witnesses)
        throw Error("Original battle canvas not ready");
      run = {canvas,context:canvas.getContext("2d"),start:performance.now(),wall:performance.timeOrigin,samples:[],controls:[]};
    },
    end() {
      const {canvas,context,...result} = run;
      run = undefined;
      return {...result,stop:performance.now()};
    }
  };
  function control(e) {
    if (run) run.controls.push({type:e.type,code:e.code,pointer:e.pointerType,trusted:e.isTrusted,at:performance.now()});
  }
  for (const type of ["keydown","keyup","pointerdown","pointerup"])
    window.addEventListener(type,control,true);
  function tick() {
    if (run) {
      const at=performance.now(), started=performance.now();
      const pixels=new Uint32Array(run.context.getImageData(0,0,320,200).data.buffer);
      const regions=[[0,0,320,200],[40,24,125,145],[160,36,304,150],[15,4,305,24],[120,163,319,197]].map(([left,top,right,bottom])=>{
        let hash=2166136261;
        for(let y=top;y<bottom;y++)
          for(let x=left;x<right;x++)hash=Math.imul(hash^pixels[y*320+x],16777619);
        return hash>>>0;
      });
      const scores=witnesses.map(({number,points})=>[number,Math.round(1000*points.reduce((n,[i,color])=>n+(pixels[i]===color),0)/points.length)]);
      scores.sort((a,b)=>b[1]-a[1]);
      const effect=scores[0][1]>=900 && scores[0][1]-scores[1][1]>=40 ? scores[0][0] : null;
      run.samples.push({at,hash:regions[0],regions,scores,effect,captureMS:performance.now()-started,focused:document.hasFocus()&&!document.hidden,phase:run.canvas.closest("[data-pal-board]")?.dataset.palPhase});
    }
    requestAnimationFrame(tick);
  }
  requestAnimationFrame(tick);
})();
