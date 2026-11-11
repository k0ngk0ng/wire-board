#!/usr/bin/env python3
"""Self-created horn E2E; this never claims an NFS original-bank mapping.

Failure cases before implementation: falsely labelled original audio, lossy
encoding, wrong PCM format/count, clipping/DC, wrong fundamentals, a discontinuous
loop boundary, changed bytes after FLAC decoding, unverified manifest entries,
and non-reproducible synthesis. Output is confined to this repository.
"""
import hashlib,json,math
from pathlib import Path
import shutil,struct,subprocess,sys,unittest

ROOT=Path(__file__).resolve().parents[2]
WORK=ROOT/'.local/racing-nfs-reference/synthetic-horn-e2e'
OUT=WORK/'output'

def run_generator(out):
 return subprocess.run([sys.executable,'-B',str(ROOT/'scripts/racing/nfs-horn.py'),'--out',str(out)],cwd=ROOT,capture_output=True,text=True)

class SyntheticHorn(unittest.TestCase):
 @classmethod
 def setUpClass(cls):
  if WORK.exists():shutil.rmtree(WORK)
  WORK.mkdir(parents=True)
  result=run_generator(OUT);(WORK/'generation.log').write_text(result.stdout+result.stderr)
  if result.returncode:raise AssertionError(result.stderr)
  cls.meta=json.loads((OUT/'audio/horn/horn.json').read_text())
  cls.pcm=subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(OUT/'audio/horn/horn.flac'),'-f','s16le','-acodec','pcm_s16le','pipe:1'],capture_output=True,check=True).stdout

 def test_self_created_dual_tone_pcm_is_recovered_exactly(self):
  m=self.meta
  self.assertEqual(m['origin'],'original-synthesis')
  self.assertIn('非极品飞车原声',m['description'])
  self.assertEqual((m['sampleRate'],m['channels'],m['bits'],m['frames']),(22050,1,16,4410))
  self.assertEqual(m['frequenciesHz'],[350,440])
  samples=[round(32767*.7*(.55*math.sin(2*math.pi*350*i/22050)+.45*math.sin(2*math.pi*440*i/22050)))for i in range(4410)]
  expected=struct.pack('<4410h',*samples)
  self.assertEqual(self.pcm,expected)
  self.assertEqual(m['pcmSha256'],hashlib.sha256(expected).hexdigest())
  self.assertEqual((m['loopStartFrame'],m['loopEndFrameExclusive']),(0,4410))
  probe=json.loads(subprocess.run(['ffprobe','-v','error','-show_entries','stream=sample_rate,channels,bits_per_raw_sample','-of','json',str(OUT/'audio/horn/horn.flac')],capture_output=True,text=True,check=True).stdout)['streams'][0]
  self.assertEqual((int(probe['sample_rate']),probe['channels'],int(probe['bits_per_raw_sample'])),(22050,1,16))

 def test_loop_is_periodic_has_both_tones_and_does_not_clip(self):
  s=struct.unpack('<4410h',self.pcm);n=len(s)
  self.assertLess(max(map(abs,s)),32767)
  self.assertLess(abs(sum(s)/n),1)
  derivative=[s[(i+1)%n]-s[i]for i in range(n)]
  self.assertLessEqual(abs(derivative[-1]),max(map(abs,derivative[:-1])))
  for i in range(64):
   continued=round(32767*.7*(.55*math.sin(2*math.pi*350*(n+i)/22050)+.45*math.sin(2*math.pi*440*(n+i)/22050)))
   self.assertEqual(continued,s[i])
  def amplitude(f):
   return math.hypot(sum(v*math.cos(2*math.pi*f*i/22050)for i,v in enumerate(s)),sum(v*math.sin(2*math.pi*f*i/22050)for i,v in enumerate(s)))*2/n
  self.assertGreater(amplitude(350),10000)
  self.assertGreater(amplitude(440),9000)
  self.assertLess(amplitude(1000),1)

 def test_manifest_metadata_and_repeated_output_are_byte_identical(self):
  manifest=json.loads((OUT/'horn-manifest.json').read_text())
  self.assertEqual({a['path']for a in manifest['assets']},{'audio/horn/horn.flac','audio/horn/horn.json'})
  self.assertLess(manifest['totalBytes'],16000)
  for a in manifest['assets']:
   data=(OUT/a['path']).read_bytes();self.assertEqual((len(data),hashlib.sha256(data).hexdigest()),(a['bytes'],a['sha256']))
  self.assertEqual(self.meta['sha256'],hashlib.sha256((OUT/'audio/horn/horn.flac').read_bytes()).hexdigest())
  result=run_generator(WORK/'repeat');self.assertEqual(result.returncode,0,result.stderr)
  for path in OUT.rglob('*'):
   if path.is_file():self.assertEqual(path.read_bytes(),(WORK/'repeat'/path.relative_to(OUT)).read_bytes())

if __name__=='__main__':
 result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(SyntheticHorn))
 if WORK.exists():(WORK/'result.json').write_text(json.dumps({'passed':result.wasSuccessful(),'tests':result.testsRun,'failures':[str(t)for t,_ in result.failures],'errors':[str(t)for t,_ in result.errors]},indent=2)+'\n')
 sys.exit(0 if result.wasSuccessful()else 1)
