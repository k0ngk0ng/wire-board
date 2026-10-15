// Run from repo root after sourcing scripts/env.sh. Renderer installation:
// npm install --prefix .local/chess-family/assets-tools --no-save --ignore-scripts @resvg/resvg-js@2.6.2
import {existsSync,mkdirSync,readFileSync,writeFileSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {createRequire} from 'node:module';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {boardSVG,blackStoneSVG,whiteStoneSVG,coverSVG} from '../../games/weiqi/assets/art.ts';
import {soundWav} from '../../web/src/audio-tones.ts';
const root=process.cwd(),work=resolve('.local/weiqi/asset-build'),out=resolve('games/weiqi/assets/public/weiqi/v1');
for(const p of [work,join(out,'images'),join(out,'audio'),'games/weiqi/assets/licenses'])mkdirSync(p,{recursive:true});
const requireTool=createRequire(resolve('.local/chess-family/assets-tools/package.json'));
const {Resvg}=requireTool('@resvg/resvg-js');
const files=[];const hash=b=>createHash('sha256').update(b).digest('hex');
function record(file,source){const b=readFileSync(join(out,file));files.push({file,bytes:b.length,sha256:hash(b),source});}
for(const [name,svg,width] of [['board',boardSVG,960],['stone-black',blackStoneSVG,160],['stone-white',whiteStoneSVG,160],['cover',coverSVG,960]]){
 const png=join(work,name+'.png');writeFileSync(png,new Resvg(svg,{fitTo:{mode:'width',value:width}}).render().asPng());
 execFileSync('cwebp',['-quiet','-lossless','-exact',png,'-o',join(out,'images',name+'.webp')]);record('images/'+name+'.webp','Original SVG: games/weiqi/assets/art.ts');
}
const archiveURL='https://kenney.nl/media/pages/assets/boardgame-pack/a1656828d2-1677667644/kenney_boardgame-pack.zip';
const inventory={sha256:"3b6a7dd5af658d1ffa0071429d5dde10ff63121e88bd06d4904e721e60b2c398"};
const cache=resolve('.local/chess-research/kenney-boardgame.zip');
let archive;if(existsSync(cache))archive=readFileSync(cache);else{const response=await fetch(archiveURL);if(!response.ok)throw Error('Asset source download failed');archive=Buffer.from(await response.arrayBuffer());}if(hash(archive)!==inventory.sha256)throw Error('Source archive changed');
const zip=join(work,'kenney.zip'),ogg=join(work,'place.ogg');writeFileSync(zip,archive);
execFileSync('python3',['-B','-c',`import zipfile,sys
with zipfile.ZipFile(sys.argv[1]) as z:
 open(sys.argv[2],'wb').write(z.read('Bonus/chipsCollide1.ogg'))
 open(sys.argv[3],'wb').write(z.read('license.txt'))`,zip,ogg,resolve('games/weiqi/assets/licenses/kenney-boardgame.txt')]);
execFileSync('ffmpeg',['-v','error','-y','-i',ogg,'-ac','1','-ar','44100','-c:a','pcm_s16le','-af','volume=0.7',join(out,'audio/place.wav')]);record('audio/place.wav','Kenney Boardgame Pack / Bonus/chipsCollide1.ogg; mono 44.1 kHz PCM; volume=0.7');
for(const [name,tone] of [['win','finish'],['draw','reserve'],['pass','turn'],['capture','take']]){writeFileSync(join(out,'audio',name+'.wav'),soundWav(tone));record('audio/'+name+'.wav','Original Wire Board melody: web/src/audio-tones.ts#'+tone);}
writeFileSync('games/weiqi/assets/sources.json',JSON.stringify({version:'weiqi/v1',images:{author:'Wire Board project',source:'games/weiqi/assets/art.ts',method:'Original SVG rendered with @resvg/resvg-js 2.6.2 and cwebp lossless WebP'},audioSource:{url:archiveURL,sha256:inventory.sha256,license:'CC0-1.0',licenseEvidence:'games/weiqi/assets/licenses/kenney-boardgame.txt',member:'Bonus/chipsCollide1.ogg'},generator:'node --experimental-strip-types scripts/weiqi/prepare-assets.mjs',files},null,2)+'\n');
console.log('Prepared',files.length,'Weiqi assets');
