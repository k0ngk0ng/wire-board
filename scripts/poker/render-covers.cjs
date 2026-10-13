const fs=require('fs'),path=require('path'),crypto=require('crypto'),cp=require('child_process'),{createRequire}=require('module');
const req=createRequire(path.resolve('web/package.json')),ts=req('typescript'),React=req('react'),{renderToStaticMarkup}=req('react-dom/server');
const {Resvg}=require(process.cwd()+'/.local/starfort/rasterizer/node_modules/@resvg/resvg-js');
const manifest=JSON.parse(fs.readFileSync('assets/poker/manifest.json','utf8'));manifest.entries=manifest.entries.filter(e=>e.type!=='cover');
fs.mkdirSync('web/public/poker/v1/covers',{recursive:true});
for(const id of ['doudizhu','guandan','shengji']){
 const input=`games/${id}/web/CoverArt.tsx`,code=ts.transpileModule(fs.readFileSync(input,'utf8'),{compilerOptions:{jsx:ts.JsxEmit.ReactJSX,module:ts.ModuleKind.CommonJS}}).outputText;
 const mod={exports:{}};new Function('require','module','exports',code)(req,mod,mod.exports);
 const svg=renderToStaticMarkup(React.createElement(mod.exports.CoverArt)).replace('<svg ','<svg xmlns="http://www.w3.org/2000/svg" '),source=`assets/poker/source/covers/${id}.svg`;fs.writeFileSync(source,svg);
 const png=new Resvg(svg,{fitTo:{mode:'width',value:1040}}).render().asPng();fs.writeFileSync('.local/poker/render/cover.png',png);
 const output=`web/public/poker/v1/covers/${id}.webp`;cp.execFileSync('cwebp',['-quiet','-q','94','.local/poker/render/cover.png','-o',output]);
 const sha=b=>crypto.createHash('sha256').update(b).digest('hex');manifest.entries.push({type:'cover',game:id,author:'wire-board contributors',license:'Project original',source,input,output,sourceSHA256:sha(Buffer.from(svg)),sha256:sha(fs.readFileSync(output)),width:1040,height:520});
}
fs.writeFileSync('assets/poker/manifest.json',JSON.stringify(manifest,null,2)+'\n');fs.copyFileSync('assets/poker/manifest.json','web/public/poker/manifest.json');
console.log('Rendered 3 original illustrated covers.');
