#!/usr/bin/env python3
"""Verify an explicitly processed playback copy; original NFS PCM stays intact.

Before implementation: original FLAC/metadata overwritten, hidden normalization
or silence fades, excessive trim, wrong source identity, invalid loop/phase,
lossy encoding, incorrect delta sizes/hashes, and non-reproducible output.
"""
import hashlib,json,math
from pathlib import Path
import shutil,struct,subprocess,sys,unittest

ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'games/racing/assets/public/racing/v3';WORK=ROOT/'.local/racing-nfs-reference/engine-playback-e2e';OUT=WORK/'output'
def sha(data):return hashlib.sha256(data).hexdigest()
def decode(path):return subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(path),'-f','s16le','-acodec','pcm_s16le','pipe:1'],capture_output=True,check=True).stdout
class PlaybackCopy(unittest.TestCase):
 @classmethod
 def setUpClass(cls):
  if WORK.exists():shutil.rmtree(WORK)
  WORK.mkdir(parents=True)
  cls.before={p.relative_to(BASE).as_posix():sha(p.read_bytes())for p in (BASE/'audio').rglob('*')if p.is_file()}
  cls.command=[sys.executable,'-B',str(ROOT/'scripts/racing/nfs-engine-playback.py'),'--source',str(BASE),'--out',str(OUT),'--model','ldia','--patch','42']
  result=subprocess.run(cls.command,cwd=ROOT,capture_output=True,text=True);(WORK/'conversion.log').write_text(result.stdout+result.stderr)
  if result.returncode:raise AssertionError(result.stderr)
 def test_original_files_and_source_metadata_remain_identical(self):
  for path,digest in self.before.items():self.assertEqual(sha((BASE/path).read_bytes()),digest)
  original=json.loads((BASE/'audio/ldia/engine.json').read_text());updated=json.loads((OUT/'audio/ldia/engine.json').read_text())
  source=next(s for s in original['samples']if s['patch']==42);sample=next(s for s in updated['samples']if s['patch']==42)
  self.assertEqual({k:v for k,v in sample.items()if k!='playback'}, {k:v for k,v in source.items()if k!='playback'})
  self.assertEqual(sample['playback']['origin'],'seam-overlap-playback');self.assertEqual(sample['playback']['sourcePcmSha256'],source['pcmSha256'])
 def test_lossless_processed_pcm_matches_the_explicit_overlap_recipe_without_silence_fades(self):
  updated=json.loads((OUT/'audio/ldia/engine.json').read_text());sample=next(s for s in updated['samples']if s['patch']==42);playback=sample['playback'];meta=json.loads((OUT/playback['metadata']).read_text())
  raw=decode(BASE/sample['path']);source=list(struct.unpack('<'+str(len(raw)//2)+'h',raw));k=round(sample['sampleRate']*.005);blend=[]
  for i in range(k):
   t=i/(k-1);weight=t*t*(3-2*t);blend.append(round(source[-k+i]*(1-weight)+source[i]*weight))
  expected=source[k:-k]+blend;rotation=min(range(len(expected)),key=lambda i:abs(expected[i]));expected=expected[rotation:]+expected[:rotation]
  encoded=decode(OUT/playback['path']);self.assertEqual(encoded,struct.pack('<'+str(len(expected))+'h',*expected));self.assertEqual(sha(encoded),playback['pcmSha256'])
  self.assertEqual(meta['recipe']['overlapFrames'],k);self.assertEqual(meta['recipe']['rotationFrames'],rotation);self.assertFalse(meta['recipe']['normalize']);self.assertFalse(meta['recipe']['fadeToSilence'])
  self.assertEqual(playback['frames'],len(source)-k);self.assertLessEqual(abs(expected[0]),3);self.assertGreaterEqual(min(expected),-32768);self.assertLessEqual(max(expected),32767);self.assertLessEqual(max(map(abs,expected)),max(map(abs,source)))
  source_rms=math.sqrt(sum(v*v for v in source)/len(source));output_rms=math.sqrt(sum(v*v for v in expected)/len(expected));self.assertTrue(.98<output_rms/source_rms<1.02)
 def test_delta_bytes_and_repeatability(self):
  delta=json.loads((OUT/'audio-manifest.delta.json').read_text())
  for a in delta['assets']:
   data=(OUT/a['path']).read_bytes();self.assertEqual((len(data),sha(data)),(a['bytes'],a['sha256']))
  self.assertEqual(len(delta['assets']),4)
  cmd=list(self.command);cmd[cmd.index('--out')+1]=str(WORK/'repeat');result=subprocess.run(cmd,cwd=ROOT,capture_output=True,text=True);self.assertEqual(result.returncode,0,result.stderr)
  for p in OUT.rglob('*'):
   if p.is_file():self.assertEqual(p.read_bytes(),(WORK/'repeat'/p.relative_to(OUT)).read_bytes())

if __name__=='__main__':
 result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(PlaybackCopy))
 if WORK.exists():(WORK/'result.json').write_text(json.dumps({'passed':result.wasSuccessful(),'tests':result.testsRun,'failures':[str(t)for t,_ in result.failures],'errors':[str(t)for t,_ in result.errors]},indent=2)+'\n')
 sys.exit(0 if result.wasSuccessful()else 1)
