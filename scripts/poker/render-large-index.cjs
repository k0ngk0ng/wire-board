// Build an integrated large-index deck from the audited originals. The face is
// reframed beside the index; no CSS panel is pasted over a complete small card.
const fs=require('fs'),path=require('path'),cp=require('child_process'),crypto=require('crypto');
const {Resvg}=require(process.cwd()+'/.local/starfort/rasterizer/node_modules/@resvg/resvg-js');
const source=JSON.parse(fs.readFileSync('assets/poker/source-manifest.json','utf8')).filter(e=>e.skin);
const manifest=JSON.parse(fs.readFileSync('assets/poker/manifest.json','utf8'));manifest.entries=manifest.entries.filter(e=>!e.skin);
const tmp='.local/poker-fixes/cards';fs.mkdirSync(tmp,{recursive:true});
const sha=b=>crypto.createHash('sha256').update(b).digest('hex');
for(const item of source){
 const bytes=fs.readFileSync(item.source);let svg=bytes.toString('utf8');
 if(item.source.endsWith('.png')){const w=bytes.readUInt32BE(16),h=bytes.readUInt32BE(20);svg=`<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}"><image width="${w}" height="${h}" href="data:image/png;base64,${bytes.toString('base64')}"/></svg>`}
 const r=new Resvg(svg,{fitTo:{mode:'width',value:600},font:{defaultFontFamily:'Arial'}});if(r.imagesToResolve().length)throw Error('External asset');const rendered=r.render(),pixels=rendered.pixels,colors=new Map();
 for(let y=0;y<rendered.height;y+=4)for(let x=0;x<rendered.width;x+=4){const i=(y*rendered.width+x)*4;if(pixels[i+3]<200||Math.min(...pixels.subarray(i,i+3))<215)continue;const color=Array.from(pixels.subarray(i,i+3)).map(n=>n.toString(16).padStart(2,'0')).join('');colors.set(color,(colors.get(color)||0)+1)}
 const paper='#'+([...colors.entries()].sort((a,b)=>b[1]-a[1])[0]?.[0]??'ffffff');const id=item.id,suit=Math.floor(id/13),joker=id>=52;
 const ink=item.skin==='fourcolor'?['#155da0','#a44117','#ad1951','#742995'][suit]??(id===53?'#a12730':'#202b29'):(suit===1||suit===2||id===53?'#a12730':'#202b29');
 const rank=id%13+2,label=({11:'J',12:'Q',13:'K',14:'A'})[rank]??String(rank);
 const index=joker?Array.from('JOKER').map((c,i)=>`<text x="36" y="${62+i*62}" text-anchor="middle" font-family="Arial" font-size="64" font-weight="700" fill="${ink}">${c}</text>`).join(''):`<text x="36" y="72" text-anchor="middle" font-family="Arial" font-size="64" font-weight="700" fill="${ink}">${label}</text><text x="36" y="135" text-anchor="middle" font-family="Arial" font-size="69" fill="${ink}">${['♣','♦','♥','♠'][suit]}</text>`;
 const w=rendered.width,h=rendered.height;
 const pip=(suit,x,y,scale=1,rotate=0,color=ink)=>{const shapes=[`<circle cx="0" cy="-17" r="15"/><circle cx="-14" cy="5" r="15"/><circle cx="14" cy="5" r="15"/><path d="M-6 10L-10 32H10L6 10Z"/>`,`<path d="M0-32L24 0 0 32-24 0Z"/>`,`<path d="M0-13C-29-44-48 2 0 32 48 2 29-44 0-13Z"/>`,`<path d="M0-32C-8-19-32-5-28 12-25 27-9 27 0 13 9 27 25 27 28 12 32-5 8-19 0-32ZM-5 10L-10 32H10L5 10Z"/>`];return `<g transform="translate(${x} ${y}) rotate(${rotate}) scale(${scale})" fill="${color}">${shapes[suit]}</g>`};
 let art='',defs='';
 if(!joker&&rank<=10){
  let positions=[];
  if(rank===2)positions=[[154,80],[154,256]];
  if(rank===3)positions=[[154,60],[154,168],[154,276]];
  if(rank===4||rank===5){positions=[[111,70],[199,70],[111,266],[199,266]];if(rank===5)positions.push([154,168])}
  if(rank>=6&&rank<=8){positions=[56,168,280].flatMap(y=>[[111,y],[199,y]]);if(rank>=7)positions.push([154,112]);if(rank===8)positions.push([154,224])}
  if(rank>=9){positions=[40,125,211,296].flatMap(y=>[[111,y],[199,y]]);if(rank===9)positions.push([154,168]);else positions.push([154,82],[154,254])}
  art=positions.map(([x,y])=>pip(suit,x,y,rank<6?.9:.62,y>168?180:0)).join('');
 }else if(!joker&&rank===14){art=pip(suit,154,168,1.65)}
 else if(joker&&item.skin==='english'){
  art=pip(2,121,119,1.04,-25)+pip(3,190,118,1.04,25)+pip(0,123,221,1.02,-25)+pip(1,189,221,1.02,25);
 }else{
  const crop=item.skin==='english'?[.20,.08,.60,.84]:item.skin==='illustrated'?(joker?[.28,.22,.46,.56]:[.28,.23,.44,.55]):joker?[.24,.10,.50,.80]:[.23,.24,.54,.52];
  const rect=[w*crop[0],h*crop[1],w*crop[2],h*crop[3]],scale=Math.min(156/rect[2],300/rect[3]),dx=73+(156-rect[2]*scale)/2,dy=18+(300-rect[3]*scale)/2;
  defs=`<clipPath id="face"><rect x="${dx}" y="${dy}" width="${rect[2]*scale}" height="${rect[3]*scale}"/></clipPath>`;
  art=`<g clip-path="url(#face)"><image x="${dx-rect[0]*scale}" y="${dy-rect[1]*scale}" width="${w*scale}" height="${h*scale}" href="data:image/png;base64,${rendered.asPng().toString('base64')}"/></g>`;
 }
 const integrated=`<svg xmlns="http://www.w3.org/2000/svg" width="240" height="336" viewBox="0 0 240 336"><defs>${defs}</defs><rect x="1" y="1" width="238" height="334" rx="15" fill="${paper}" stroke="#b8b5a7" stroke-width="1.3"/>${art}${index}</svg>`;
 const png=new Resvg(integrated,{fitTo:{mode:'width',value:480},font:{defaultFontFamily:'Arial'}}).render().asPng();const dest=`web/public/poker/v4/${item.skin}/${id}.webp`;fs.mkdirSync(path.dirname(dest),{recursive:true});fs.writeFileSync(tmp+'/current.png',png);cp.execFileSync('cwebp',['-quiet','-q','94',tmp+'/current.png','-o',dest]);
 if([0,1,2,3,4,5,6,7,8,9,10,11,12,52,53].includes(id))fs.writeFileSync(`${tmp}/${item.skin}-${id}.png`,png);
 manifest.entries.push({...item,output:dest,sha256:sha(fs.readFileSync(dest)),width:480,height:672,variant:'integrated-large-index-v4',indexTypography:{family:'Arial',size:64,weight:700},pipArt:'Project original vector suit paths; court figures adapted from the credited source',generator:'scripts/poker/render-large-index.cjs'});
}
fs.writeFileSync('assets/poker/manifest.json',JSON.stringify(manifest,null,2)+'\n');fs.copyFileSync('assets/poker/manifest.json','web/public/poker/manifest.json');console.log('Prepared',source.length,'integrated cards.');
let gallery='';for(const [row,skin] of ['english','illustrated','fourcolor'].entries())for(const [col,id] of [0,1,2,3,4,5,6,7,8,9,10,11,12,52,53].entries()){const b=fs.readFileSync(`${tmp}/${skin}-${id}.png`);gallery+=`<image x="${col*124}" y="${row*178}" width="120" height="168" href="data:image/png;base64,${b.toString('base64')}"/>`}
fs.writeFileSync(tmp+'/gallery.png',new Resvg(`<svg xmlns="http://www.w3.org/2000/svg" width="1860" height="534"><rect width="1860" height="534" fill="#d5dbd2"/>${gallery}</svg>`).render().asPng());
