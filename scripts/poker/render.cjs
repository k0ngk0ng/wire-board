// Render the audited sources; no network access and no archive extraction.
const fs=require('fs'),path=require('path'),cp=require('child_process'),crypto=require('crypto');
const root=process.cwd(), {Resvg}=require(root+'/.local/starfort/rasterizer/node_modules/@resvg/resvg-js');
const entries=JSON.parse(fs.readFileSync('assets/poker/source-manifest.json','utf8'));
fs.mkdirSync('.local/poker/render',{recursive:true});const manifest=[];
for(const item of entries){
 if(item.type==='sound'){manifest.push({...item,sha256:item.sourceSHA256});continue}
 const bytes=fs.readFileSync(item.source);let svg;
 if(item.source.endsWith('.svg')) svg=bytes.toString('utf8');
 else {const w=bytes.readUInt32BE(16),h=bytes.readUInt32BE(20);svg=`<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}"><image width="${w}" height="${h}" href="data:image/png;base64,${bytes.toString('base64')}"/></svg>`}
 const r=new Resvg(svg,{fitTo:{mode:'width',value:240},font:{defaultFontFamily:'Arial'}});
 if(r.imagesToResolve().length)throw Error('Unexpected external source '+item.source);
 const png=r.render().asPng();fs.writeFileSync('.local/poker/render/current.png',png);
 const dest='web/public/poker/v1/'+(item.type==='back'?'backs/'+item.name:item.skin+'/'+item.id)+'.webp';fs.mkdirSync(path.dirname(dest),{recursive:true});
 cp.execFileSync('cwebp',['-quiet','-q','92','.local/poker/render/current.png','-o',dest]);
 manifest.push({...item,output:dest,width:png.readUInt32BE(16),height:png.readUInt32BE(20),sha256:crypto.createHash('sha256').update(fs.readFileSync(dest)).digest('hex')});
}
for(const skin of ['english','illustrated','fourcolor']){const faces=manifest.filter(m=>m.skin===skin);if(faces.length!==54||new Set(faces.map(m=>m.id)).size!==54)throw Error('Incomplete '+skin)}
fs.writeFileSync('assets/poker/manifest.json',JSON.stringify({version:1,entries:manifest},null,2)+'\n');
fs.copyFileSync('assets/poker/manifest.json','web/public/poker/manifest.json');
console.log('Rendered and checked '+manifest.length+' assets.');
