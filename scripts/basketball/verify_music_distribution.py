"""Validate compressed delivery, original masters and exact decoded duration."""
from pathlib import Path
import hashlib,json

ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/basketball/audio/export'


def sha(path):
    h=hashlib.sha256()
    with path.open('rb') as f:
        while chunk:=f.read(1024*1024):h.update(chunk)
    return h.hexdigest()


def main():
    manifest=json.loads((OUT/'music-distribution-manifest.json').read_text())
    playback=json.loads((ROOT/'games/basketball/assets/audio-playback.json').read_text())['music']
    assert set(map(int,playback))=={59,60,61,62,63,64,65,66,75}
    assert {r['id'] for r in manifest['tracks']}==set(map(int,playback))
    total_source=total_encoded=0
    for entry in manifest['tracks']:
        p=playback[str(entry['id'])]
        assert p['file']==entry['file'] and p['file'].endswith('.mp3') and p['mime_type']=='audio/mpeg'
        assert p['wav']==entry['master'] and sha(OUT/entry['master'])==entry['master_sha256']==p['sha256']
        assert sha(OUT/entry['file'])==entry['sha256']==p['distribution_sha256']
        assert sha(OUT/entry['repeat_file'])==entry['sha256']
        assert entry['sample_rate']==44100 and entry['channels']==1
        assert entry['decoded_samples']==entry['source_samples']
        assert abs(entry['container_duration']-entry['source_samples']/44100)<=1/44100
        assert entry['si_sdr_db']>=24
        assert entry['seam']['within_decoded_adjacent_range']
        assert entry['decoded_peak_db']<0
        assert all(abs(v)<=1.5 for v in entry['seam']['edge_rms_difference_db'])
        assert not entry['listening_accepted'] and not entry['accepted_for_release']
        total_source+=(OUT/entry['master']).stat().st_size
        total_encoded+=(OUT/entry['file']).stat().st_size
    assert total_encoded<total_source*.3
    report={'status':'passed','scope':'compressed MP3 delivery and exact decoded timelines; not human listening or device-matrix acceptance',
        'tracks':len(manifest['tracks']),'source_bytes':total_source,'encoded_bytes':total_encoded,
        'reduction_percent':100*(1-total_encoded/total_source),'accepted_for_release':False,'listening_accepted':False}
    (OUT/'music-distribution-verification.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))


if __name__=='__main__':main()
