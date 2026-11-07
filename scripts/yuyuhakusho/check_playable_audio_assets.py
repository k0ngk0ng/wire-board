"""Verify every shipped FLAC object and its losslessly decoded PCM."""
from pathlib import Path
import argparse, hashlib, json, subprocess

ROOT = Path(__file__).resolve().parents[2]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--media', type=Path, default=ROOT/'.local/md-yuyuhakusho/playable-audio/media')
parser.add_argument('--out', type=Path, required=True)
args = parser.parse_args()
manifest = ROOT/'games/yuyuhakusho/assets/audio-playback.json'
sha = lambda raw: hashlib.sha256(raw).hexdigest()
x = json.loads(manifest.read_bytes())
assert x['schema'] == 1
assert set(map(int, x['music'])) == {130,131,133,134,136,138,141,149}
assert set(map(int, x['effects'])) == set(range(6,127)) | set(range(160,172)) | {143,240}
entries = [*x['music'].values(), *x['effects'].values()]
raw_bytes = encoded_bytes = 0
for row in entries:
    p = args.media/row['file']
    b = p.read_bytes()
    assert len(b) == row['bytes'] and sha(b) == row['sha256'], p
    pcm = subprocess.check_output(['ffmpeg','-v','error','-i',str(p),'-f','s16le','-acodec','pcm_s16le','-'], cwd=ROOT)
    meta = json.loads(subprocess.check_output(['ffprobe','-v','error','-select_streams','a:0','-show_entries','stream=sample_rate,channels,bits_per_raw_sample','-of','json',str(p)], cwd=ROOT))['streams'][0]
    assert (int(meta['sample_rate']), meta['channels'], int(meta['bits_per_raw_sample'])) == (44100,2,16), p
    assert len(pcm) == row['frames']*4 and sha(pcm) == row['pcm_sha256'], p
    assert len(b) < len(pcm)+44, p
    raw_bytes += len(pcm)+44; encoded_bytes += len(b)
report = {'pass':True,'clips':len(entries),'wav_bytes':raw_bytes,'flac_bytes':encoded_bytes,'all_pcm_sha256_equal':True,'format':{'sample_rate':44100,'channels':2,'bits':16},'manifest_sha256':sha(manifest.read_bytes())}
assert args.out.resolve().is_relative_to(ROOT)
args.out.parent.mkdir(parents=True,exist_ok=True)
args.out.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report))
