#!/usr/bin/env python3
"""Run the original-bank-to-FLAC pipeline and compare against native PCM."""

import hashlib
import json
import math
from pathlib import Path
import shutil
import subprocess
import sys
import unittest

from speedtools.car_data import CarData

ROOT=Path(__file__).resolve().parents[2]
GAME=ROOT/'.local/racing-nfs-reference/additional/game-root'
WORK=ROOT/'.local/racing-nfs-reference/audio-e2e'
OUT=WORK/'output'
EXPECTED={'P911':5,'FM50':8,'LDIA':3,'MCF1':5,'CCOR':6}


class OriginalAudio(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if WORK.exists():shutil.rmtree(WORK)
        WORK.mkdir(parents=True)
        cls.command=[sys.executable,'-B',str(ROOT/'scripts/racing/nfs-audio.py'),
                     '--game-root',str(GAME),'--out',str(OUT)]
        run=subprocess.run(cls.command,cwd=ROOT,text=True,capture_output=True)
        (WORK/'conversion.log').write_text(run.stdout+run.stderr)
        if run.returncode:raise AssertionError(run.stderr)

    def test_original_bank_loop_pcm_and_flac_are_identical(self):
        for ident,count in EXPECTED.items():
            metadata=json.loads((OUT/f'audio/{ident.lower()}/engine.json').read_text())
            samples=metadata['samples']
            native={e.patchnum:e for e in CarData(GAME/'Data/Cars'/ident,GAME).engine_audio if e.patchnum<64}
            self.assertEqual(len(samples),count)
            self.assertEqual({sample['patch'] for sample in samples},set(native))
            for sample in samples:
                with self.subTest(car=ident,patch=sample['patch']):
                    source=native[sample['patch']].streams[0]
                    self.assertEqual(source.compression.name,'PCM')
                    self.assertEqual((source.sample_rate,source.num_channels),(22050,1))
                    self.assertEqual(len(source.audio_samples),source.decompressed_samples*2)
                    start=source.loop_start;end=source.loop_end+1
                    pcm=source.audio_samples[start*2:end*2]
                    target=OUT/sample['path']
                    decoded=subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(target),'-f','s16le','-acodec','pcm_s16le','pipe:1'],capture_output=True,check=True).stdout
                    self.assertEqual(decoded,pcm)
                    self.assertEqual(sample['pcmSha256'],hashlib.sha256(pcm).hexdigest())
                    self.assertEqual(sample['source']['pcmSha256'],hashlib.sha256(source.audio_samples).hexdigest())
                    self.assertEqual((sample['source']['loopStart'],sample['source']['loopEndInclusive']),(start,end-1))
                    self.assertEqual(sample['frames'],end-start)
                    probe=json.loads(subprocess.run(['ffprobe','-v','error','-show_entries','stream=sample_rate,channels,bits_per_raw_sample','-of','json',str(target)],capture_output=True,check=True,text=True).stdout)['streams'][0]
                    self.assertEqual((int(probe['sample_rate']),probe['channels'],int(probe['bits_per_raw_sample'])),(22050,1,16))

    def test_load_coast_tables_and_pitch_metadata_match_native(self):
        for ident in EXPECTED:
            metadata=json.loads((OUT/f'audio/{ident.lower()}/engine.json').read_text())
            self.assertEqual(metadata['mapping']['authority'],'community-reconstruction')
            native={e.patchnum:e for e in CarData(GAME/'Data/Cars'/ident,GAME).engine_audio if e.patchnum<64}
            for sample in metadata['samples']:
                source=native[sample['patch']]
                stream=source.streams[0]
                self.assertEqual((sample['rootKey'],sample['pitchBendRange'],sample['detune']),
                                 (stream.root_key,stream.pitch_bend_range,stream.detune))
                self.assertEqual(len(sample['tables']),len(source.tables))
                for table,expected in zip(sample['tables'],source.tables):
                    self.assertEqual(table['pitch'],expected.pitch)
                    self.assertEqual(table['volume'],expected.volume)
                    self.assertEqual(len(table['rate']),512)
                    self.assertTrue(all(math.isfinite(rate) and rate>0 for rate in table['rate']))

    def test_manifest_budget_and_repeatability(self):
        manifest=json.loads((OUT/'audio-manifest.json').read_text())
        self.assertLess(manifest['totalBytes'],2_000_000)
        for asset in manifest['assets']:
            data=(OUT/asset['path']).read_bytes()
            self.assertEqual((len(data),hashlib.sha256(data).hexdigest()),(asset['bytes'],asset['sha256']))
        original={p.relative_to(OUT).as_posix():hashlib.sha256(p.read_bytes()).hexdigest() for p in OUT.rglob('*')if p.is_file()}
        second=WORK/'repeat';cmd=list(self.command);cmd[cmd.index('--out')+1]=str(second)
        run=subprocess.run(cmd,cwd=ROOT,text=True,capture_output=True)
        self.assertEqual(run.returncode,0,run.stderr)
        repeated={p.relative_to(second).as_posix():hashlib.sha256(p.read_bytes()).hexdigest() for p in second.rglob('*')if p.is_file()}
        self.assertEqual(original,repeated)


if __name__=='__main__':
    result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(OriginalAudio))
    if WORK.exists():(WORK/'result.json').write_text(json.dumps({'passed':result.wasSuccessful(),'tests':result.testsRun,
        'models':list(EXPECTED),'failures':[str(t)for t,_ in result.failures],'errors':[str(t)for t,_ in result.errors]},indent=2)+'\n')
    sys.exit(0 if result.wasSuccessful() else 1)
