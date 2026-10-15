import {mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {coverSVG,boardSVG} from '../../games/chess/assets/art.ts';
import {soundWav} from '../../web/src/audio-tones.ts';
const work=resolve('.local/chess/asset-build'),out=resolve('games/chess/assets/public/chess/v1');
for(const p of [work,join(out,'images'),join(out,'audio'),'games/chess/assets/licenses'])mkdirSync(p,{recursive:true});
const {Resvg}=createRequire(resolve('.local/chess-family/assets-tools/package.json'))('@resvg/resvg-js');
const hash=b=>createHash('sha256').update(b).digest('hex'),files=[],originals=[];
function record(file,source){const b=readFileSync(join(out,file));files.push({file,bytes:b.length,sha256:hash(b),source});}
function render(name,svg,width,source){const png=join(work,name+'.png');writeFileSync(png,new Resvg(svg,{fitTo:{mode:'width',value:width}}).render().asPng());execFileSync('cwebp',['-quiet','-lossless','-exact',png,'-o',join(out,'images',name+'.webp')]);record('images/'+name+'.webp',source);}
for(const c of 'wb')for(const p of 'KQRBNP'){
 const name=c+p,path='games/chess/assets/pieces/'+name+'.svg',data=readFileSync(path);
 originals.push({file:path,sha256:hash(data),url:'https://raw.githubusercontent.com/LexLuengas/chessnut-pieces/2b8eaf14a31edad7e9deb53b1473e1d4857868a9/'+name+'.svg'});
 render(name,data,192,'Chessnut Apache-2.0; original SVG rendered unchanged');
}
render('cover',coverSVG,960,'Original SVG: games/chess/assets/art.ts');render('board',boardSVG,1024,'Original SVG: games/chess/assets/art.ts');
const archive=resolve('.local/chess-research/kenney-boardgame.zip');
if(hash(readFileSync(archive))!=='3b6a7dd5af658d1ffa0071429d5dde10ff63121e88bd06d4904e721e60b2c398')throw Error('Source changed');
execFileSync('python3',['-B','-c',`import zipfile,sys
with zipfile.ZipFile(sys.argv[1]) as z:
 for name,member in [('move','chipsCollide1'),('capture','chipsCollide2')]:
  open(sys.argv[2]+'/'+name+'.ogg','wb').write(z.read('Bonus/'+member+'.ogg'))
 open(sys.argv[3],'wb').write(z.read('license.txt'))`,archive,work,resolve('games/chess/assets/licenses/kenney-boardgame.txt')]);
for(const name of ['move','capture']){execFileSync('ffmpeg',['-v','error','-y','-i',join(work,name+'.ogg'),'-ac','1','-ar','44100','-c:a','pcm_s16le','-af','volume=0.7',join(out,'audio',name+'.wav')]);record('audio/'+name+'.wav','Kenney Boardgame Pack / Bonus/chipsCollide'+(name==='move'?'1':'2')+'.ogg; mono 44.1kHz PCM volume=0.7');}
for(const [name,tone] of [['check','turn'],['win','finish'],['draw','reserve']]){writeFileSync(join(out,'audio',name+'.wav'),soundWav(tone));record('audio/'+name+'.wav','Original Wire Board melody: web/src/audio-tones.ts#'+tone);}
writeFileSync('games/chess/assets/sources.json',JSON.stringify({version:'chess/v1',pieces:{repository:'LexLuengas/chessnut-pieces',commit:'2b8eaf14a31edad7e9deb53b1473e1d4857868a9',license:'Apache-2.0',licenseEvidence:'games/chess/assets/licenses/chessnut-LICENSE.txt',copyrightEvidence:'games/chess/assets/licenses/chessnut-COPYRIGHT.txt',originals},images:{source:'games/chess/assets/art.ts',method:'Original SVG; @resvg/resvg-js 2.6.2 and cwebp lossless'},audioSource:{url:'https://kenney.nl/media/pages/assets/boardgame-pack/a1656828d2-1677667644/kenney_boardgame-pack.zip',sha256:hash(readFileSync(archive)),license:'CC0-1.0',licenseEvidence:'games/chess/assets/licenses/kenney-boardgame.txt'},generator:'node --experimental-strip-types scripts/chess/prepare-assets.mjs',files},null,2)+'\n');
console.log('Prepared',files.length,'Chess assets');
