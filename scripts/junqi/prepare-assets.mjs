import {mkdirSync,existsSync,readFileSync,writeFileSync} from 'node:fs';
import {resolve,join} from 'node:path';import {createRequire} from 'node:module';import {createHash} from 'node:crypto';import {execFileSync} from 'node:child_process';
import {coverSVG,boardSVG,pieceSVG,names} from '../../games/junqi/assets/art.ts';import {soundWav} from '../../web/src/audio-tones.ts';
const work=resolve('.local/junqi/asset-build'),out=resolve('games/junqi/assets/public/junqi/v1');
for(const p of [work,join(out,'images'),join(out,'audio'),'games/junqi/assets/licenses'])mkdirSync(p,{recursive:true});
const {Resvg}=createRequire(resolve('.local/chess-family/assets-tools/package.json'))('@resvg/resvg-js');const hash=b=>createHash('sha256').update(b).digest('hex'),files=[];
const fontSource=JSON.parse(readFileSync('games/junqi/assets/font-source.json','utf8')),fontMeta=fontSource.files.find(f=>f.name.endsWith('.ttf')),fontFile=join(work,'NotoSerifSC.ttf');
let font=existsSync(join(work,'variable.ttf'))?readFileSync(join(work,'variable.ttf')):Buffer.from(await(await fetch(fontMeta.url)).arrayBuffer());if(hash(font)!==fontMeta.sha256)throw Error('Font changed');writeFileSync(join(work,'variable.ttf'),font);
execFileSync('python3',['-B','-c',`import sys
sys.path.insert(0,sys.argv[1])
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont
font=TTFont(sys.argv[2]);instantiateVariableFont(font,{'wght':700},inplace=True);font.save(sys.argv[3])`,resolve('.local/chess-family/font-tools'),join(work,'variable.ttf'),fontFile]);
function record(file,source){const b=readFileSync(join(out,file));files.push({file,bytes:b.length,sha256:hash(b),source});}
function render(name,svg,width){const png=join(work,name+'.png');writeFileSync(png,new Resvg(svg,{font:{fontFiles:[fontFile],loadSystemFonts:false,defaultFontFamily:'Noto Serif SC'},fitTo:{mode:'width',value:width}}).render().asPng());execFileSync('cwebp',['-quiet','-lossless','-exact',png,'-o',join(out,'images',name+'.webp')]);record('images/'+name+'.webp','Original SVG: games/junqi/assets/art.ts; Noto Serif SC OFL at wght=700');}
for(let seat=0;seat<4;seat++)for(const kind of [...Object.keys(names),'back'])render(`${seat}-${kind}`,pieceSVG(seat,kind),200);
render('board-duel',boardSVG(false),720);render('board-teams',boardSVG(true),1440);render('cover',coverSVG,960);
const archive=join(work,'kenney-boardgame.zip');
if(!existsSync(archive)){const cache=resolve('.local/chess-research/kenney-boardgame.zip');const data=existsSync(cache)?readFileSync(cache):Buffer.from(await(await fetch('https://kenney.nl/media/pages/assets/boardgame-pack/a1656828d2-1677667644/kenney_boardgame-pack.zip')).arrayBuffer());writeFileSync(archive,data)}
if(hash(readFileSync(archive))!=='3b6a7dd5af658d1ffa0071429d5dde10ff63121e88bd06d4904e721e60b2c398')throw Error('Source changed');
execFileSync('python3',['-B','-c',`import zipfile,sys
with zipfile.ZipFile(sys.argv[1]) as z:
 for name,member in [('move','chipsCollide1'),('capture','chipsCollide3')]:open(sys.argv[2]+'/'+name+'.ogg','wb').write(z.read('Bonus/'+member+'.ogg'))
 open(sys.argv[3],'wb').write(z.read('license.txt'))`,archive,work,resolve('games/junqi/assets/licenses/kenney-boardgame.txt')]);
for(const name of ['move','capture']){execFileSync('ffmpeg',['-v','error','-y','-i',join(work,name+'.ogg'),'-ac','1','-ar','44100','-c:a','pcm_s16le','-af','volume=0.7',join(out,'audio',name+'.wav')]);record('audio/'+name+'.wav','Kenney Boardgame Pack / Bonus/chipsCollide'+(name==='move'?'1':'3')+'.ogg; mono 44.1kHz PCM volume=0.7');}
for(const [name,tone] of [['reveal','turn'],['flag','buy'],['win','finish'],['draw','reserve'],['ready','move']]){writeFileSync(join(out,'audio',name+'.wav'),soundWav(tone));record('audio/'+name+'.wav','Original Wire Board melody: web/src/audio-tones.ts#'+tone);}
// Original short, deterministic low-frequency blast. No external recordings.
const rate=44100,count=Math.floor(rate*.3),wav=Buffer.alloc(44+count*2);wav.write('RIFF');wav.writeUInt32LE(36+count*2,4);wav.write('WAVEfmt ',8);wav.writeUInt32LE(16,16);wav.writeUInt16LE(1,20);wav.writeUInt16LE(1,22);wav.writeUInt32LE(rate,24);wav.writeUInt32LE(rate*2,28);wav.writeUInt16LE(2,32);wav.writeUInt16LE(16,34);wav.write('data',36);wav.writeUInt32LE(count*2,40);let seed=7193,low=0;for(let i=0;i<count;i++){seed=(Math.imul(seed,1664525)+1013904223)>>>0;low=.92*low+.08*(seed/2147483648-1);const t=i/rate,fade=Math.exp(-18*t)*Math.min(1,t/.004);wav.writeInt16LE(Math.round((low*.65+Math.sin(2*Math.PI*(95-80*t)*t)*.12)*fade*26000),44+i*2)}writeFileSync(join(out,'audio/bomb.wav'),wav);record('audio/bomb.wav','Original deterministic filtered noise and decaying sine in scripts/junqi/prepare-assets.mjs');
writeFileSync('games/junqi/assets/sources.json',JSON.stringify({version:'junqi/v1',font:{...fontSource,license:'OFL-1.1',licenseEvidence:'games/junqi/assets/licenses/NotoSerifSC-OFL.txt'},images:{source:'games/junqi/assets/art.ts',method:'Original SVG; fontTools 4.60.1 wght=700, @resvg/resvg-js 2.6.2 and lossless cwebp'},audioSource:{url:'https://kenney.nl/media/pages/assets/boardgame-pack/a1656828d2-1677667644/kenney_boardgame-pack.zip',sha256:hash(readFileSync(archive)),license:'CC0-1.0',licenseEvidence:'games/junqi/assets/licenses/kenney-boardgame.txt'},generator:'node --experimental-strip-types scripts/junqi/prepare-assets.mjs',files},null,2)+'\n');console.log('Prepared',files.length,'Junqi assets');
