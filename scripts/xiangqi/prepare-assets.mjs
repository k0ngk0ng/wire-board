import {mkdirSync,existsSync,readFileSync,writeFileSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {coverSVG,boardSVG,pieceSVG} from '../../games/xiangqi/assets/art.ts';
import {soundWav} from '../../web/src/audio-tones.ts';
const work=resolve('.local/xiangqi/asset-build'),out=resolve('games/xiangqi/assets/public/xiangqi/v1');
for(const p of [work,join(out,'images'),join(out,'audio'),'games/xiangqi/assets/licenses'])mkdirSync(p,{recursive:true});
const {Resvg}=createRequire(resolve('.local/chess-family/assets-tools/package.json'))('@resvg/resvg-js');
const hash=b=>createHash('sha256').update(b).digest('hex'),files=[];
const fontSource=JSON.parse(readFileSync('games/xiangqi/assets/font-source.json','utf8'));
const fontMeta=fontSource.files.find(f=>f.name.endsWith('.ttf')),fontFile=join(work,'NotoSerifSC.ttf');
let font=existsSync('.local/xiangqi/font/'+fontMeta.name)?readFileSync('.local/xiangqi/font/'+fontMeta.name):Buffer.from(await(await fetch(fontMeta.url)).arrayBuffer());
if(hash(font)!==fontMeta.sha256)throw Error('Font source changed');writeFileSync(join(work,'font-variable.ttf'),font);
execFileSync('python3',['-B','-c',`import sys
sys.path.insert(0,sys.argv[1])
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont
font=TTFont(sys.argv[2])
instantiateVariableFont(font,{'wght':700},inplace=True)
font.save(sys.argv[3])`,resolve('.local/xiangqi/font-tools'),join(work,'font-variable.ttf'),fontFile]);
function record(file,source){const b=readFileSync(join(out,file));files.push({file,bytes:b.length,sha256:hash(b),source});}
function render(name,svg,width){const png=join(work,name+'.png');writeFileSync(png,new Resvg(svg,{font:{fontFiles:[fontFile],loadSystemFonts:false,defaultFontFamily:'Noto Serif SC'},fitTo:{mode:'width',value:width}}).render().asPng());execFileSync('cwebp',['-quiet','-lossless','-exact',png,'-o',join(out,'images',name+'.webp')]);record('images/'+name+'.webp','Original SVG: games/xiangqi/assets/art.ts; Noto Serif SC OFL Chinese glyphs');}
for(const color of [1,-1])for(let p=1;p<=7;p++)render((color>0?'r':'b')+p,pieceSVG(color*p),192);
render('board',boardSVG,900);render('cover',coverSVG,960);
const archive=resolve('.local/chess-research/kenney-boardgame.zip');
if(hash(readFileSync(archive))!=='3b6a7dd5af658d1ffa0071429d5dde10ff63121e88bd06d4904e721e60b2c398')throw Error('Source changed');
execFileSync('python3',['-B','-c',`import zipfile,sys
with zipfile.ZipFile(sys.argv[1]) as z:
 for name,member in [('move','chipsCollide2'),('capture','chipsCollide3')]:
  open(sys.argv[2]+'/'+name+'.ogg','wb').write(z.read('Bonus/'+member+'.ogg'))
 open(sys.argv[3],'wb').write(z.read('license.txt'))`,archive,work,resolve('games/xiangqi/assets/licenses/kenney-boardgame.txt')]);
for(const name of ['move','capture']){execFileSync('ffmpeg',['-v','error','-y','-i',join(work,name+'.ogg'),'-ac','1','-ar','44100','-c:a','pcm_s16le','-af','volume=0.7',join(out,'audio',name+'.wav')]);record('audio/'+name+'.wav','Kenney Boardgame Pack / Bonus/chipsCollide'+(name==='move'?'2':'3')+'.ogg; mono 44.1kHz PCM volume=0.7');}
for(const [name,tone] of [['check','turn'],['win','finish'],['draw','reserve']]){writeFileSync(join(out,'audio',name+'.wav'),soundWav(tone));record('audio/'+name+'.wav','Original Wire Board melody: web/src/audio-tones.ts#'+tone);}
writeFileSync('games/xiangqi/assets/sources.json',JSON.stringify({version:'xiangqi/v1',font:{...fontSource,license:'OFL-1.1',licenseEvidence:'games/xiangqi/assets/licenses/NotoSerifSC-OFL.txt'},images:{source:'games/xiangqi/assets/art.ts',method:'Original SVG rendered with @resvg/resvg-js 2.6.2 using Noto Serif SC instantiated at wght=700 with fontTools 4.60.1; lossless cwebp'},audioSource:{url:'https://kenney.nl/media/pages/assets/boardgame-pack/a1656828d2-1677667644/kenney_boardgame-pack.zip',sha256:hash(readFileSync(archive)),license:'CC0-1.0',licenseEvidence:'games/xiangqi/assets/licenses/kenney-boardgame.txt'},generator:'node --experimental-strip-types scripts/xiangqi/prepare-assets.mjs',files},null,2)+'\n');console.log('Prepared',files.length,'Xiangqi assets');
