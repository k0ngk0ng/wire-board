"""Reuse existing original recordings for the authorized event-driven release.

No native executable, new capture or hardware timing reconstruction is needed.
The long BGM captures loop as media; exact hardware loop phase is not claimed.
"""
from pathlib import Path
import hashlib, json, subprocess, wave

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT/'.local/md-yuyuhakusho/playable-audio/media'
OUT.mkdir(parents=True, exist_ok=True)
sha = lambda raw: hashlib.sha256(raw).hexdigest()
catalog = json.loads((ROOT/'games/yuyuhakusho/data/audio-assets.json').read_bytes())
rows = []
for entry in catalog['assets']:
    source = ROOT/entry['media_file']
    assert sha(source.read_bytes()) == entry['wav_sha256']
    rows.append((entry['code'], 'effects', source, False))
for code in [0x82,0x83,0x85,0x86,0x88,0x8a,0x8d,0x95]:
    if code == 0x82:
        proof = json.loads((ROOT/'.local/md-yuyuhakusho/audio-loop-verified/82-control-summary.json').read_bytes())
        source = ROOT/proof['pcm_wav_file']
        expected = proof['wav_sha256']
    else:
        proof = json.loads((ROOT/f'.local/md-yuyuhakusho/audio-music-survey/extended-{code:02x}/survey.json').read_bytes())['songs'][0]
        source = ROOT/proof['files']['wav']['path']
        expected = proof['files']['wav']['sha256']
    assert sha(source.read_bytes()) == expected, source
    rows.append((code, 'music', source, True))
# F0 is a separate original remapped request; ordinary7A is not substituted.
rows.append((0xf0, 'effects', ROOT/'.local/md-yuyuhakusho/audio-dispatch-study/special-f0-original.wav', False))
manifest = {'schema':1,'mode':'event-driven-media','music':{},'effects':{}}
provenance = []
for code, kind, source, loop in rows:
    with wave.open(str(source)) as w:
        assert (w.getframerate(),w.getnchannels(),w.getsampwidth()) == (44100,2,2)
        frames = w.getnframes(); pcm = w.readframes(frames)
    file = f'{code:02x}.flac'
    target = OUT/file
    subprocess.run(['flac','--silent','--force','--best','--no-padding','--no-seektable','-o',str(target),str(source)], cwd=ROOT, check=True)
    encoded = target.read_bytes()
    # A few very short samples can be smaller as raw WAV. These original
    # accepted clips all compress below their source; the checker enforces it.
    manifest[kind][str(code)] = {'file':file,'mime':'audio/flac','bytes':len(encoded),'sha256':sha(encoded),'frames':frames,'pcm_sha256':sha(pcm),'loop':loop}
    provenance.append({'code':code,'source':str(source.relative_to(ROOT)),'wav_sha256':sha(source.read_bytes()),'pcm_sha256':sha(pcm)})
target = ROOT/'games/yuyuhakusho/assets/audio-playback.json'
target.parent.mkdir(parents=True,exist_ok=True)
target.write_text(json.dumps(manifest,indent=2)+'\n')
(ROOT/'games/yuyuhakusho/verification/playable-audio/media-provenance.json').write_text(json.dumps({'source':'existing original captures','fidelity':'Gameplay events select media; original channel stealing, precise start phase and BGM loop phase can differ.','entries':provenance},indent=2)+'\n')
print(json.dumps({'clips':len(rows),'flac_bytes':sum(r['bytes']for k in ['music','effects'] for r in manifest[k].values()),'manifest':str(target)}))
