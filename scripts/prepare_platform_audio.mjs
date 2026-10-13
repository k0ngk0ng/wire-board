// Reproduce the original platform melodies as native-media WAV assets.
import {mkdirSync,writeFileSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {createHash} from 'node:crypto';
import {soundNames,soundWav} from '../web/src/audio-tones.ts';
if(!process.argv[2])throw Error('Usage: node --experimental-strip-types scripts/prepare_platform_audio.mjs OUTPUT');
const out=resolve(process.argv[2]);mkdirSync(out,{recursive:true});
const files=soundNames.map(name=>{
 const bytes=soundWav(name),file=name+'.wav';writeFileSync(join(out,file),bytes);
 return {file,mime:'audio/wav',bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex')};
});
writeFileSync(join(out,'manifest.json'),JSON.stringify({source:'Original Wire Board GameAudio melodies, rendered to 44.1 kHz mono PCM WAV without Web Audio playback',files},null,2)+'\n');
console.log(`Prepared ${files.length} platform cues.`);
