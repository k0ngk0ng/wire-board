"""Validate compressed delivery, original masters and exact decoded duration."""
from pathlib import Path
import hashlib,json,wave

ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/basketball/audio/export'


def sha(path):
    h=hashlib.sha256()
    with path.open('rb') as f:
        while chunk:=f.read(1024*1024):h.update(chunk)
    return h.hexdigest()


def verify_music76(playback):
    evidence=ROOT/'.local/music76-source/assets'
    path=OUT/'music76-delivery-manifest.json';entry=json.loads(path.read_text());p=playback['76']
    assert sha(path)==sha(evidence/path.name),'staged76 manifest differs from original verification'
    assert entry['status']=='offline-asset-verified' and entry['id']==76
    assert entry['source']['settings']['romSHA256']=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    assert p['file']==entry['file']=='music-076-intro-loop.mp3' and p['mime_type']=='audio/mpeg'
    assert p['wav']==entry['master']=='music-076-intro-loop.wav' and p['loop'] is True
    assert p['source_manifest']==p['encoding_manifest']=='music76-delivery-manifest.json'
    assert sha(OUT/p['file'])==sha(evidence/p['file'])==entry['sha256']==entry['distribution_sha256']==p['distribution_sha256']
    assert sha(OUT/p['wav'])==sha(evidence/p['wav'])==entry['master_sha256']==p['sha256']
    for key in ['sample_rate','loop_start_sample','loop_end_sample','loop_samples','decoded_samples']:
        assert type(p[key]) is int and p[key]>0 and p[key]==entry[key]
    assert p['sample_rate']==44100 and p['loop_start_sample']<p['loop_end_sample']==p['decoded_samples']==entry['source_samples']
    assert p['loop_samples']==p['loop_end_sample']-p['loop_start_sample']
    assert entry['source_frames']==[7449,44313] and entry['period_frames']==36864 and entry['musical_loops']==35
    with wave.open(str(OUT/p['wav'])) as f:assert (f.getnframes(),f.getnchannels(),f.getsampwidth(),f.getframerate())==(p['decoded_samples'],1,2,44100)
    assert all(sha(evidence/r['file'])==r['sha256']==p['sha256'] for r in entry['independent_masters'])
    enc=entry['encoding'];assert sha(evidence/enc['repeat_file'])==entry['sha256']
    assert enc['metrics']['decoded_pcm_sha256']==enc['repeat_metrics']['decoded_pcm_sha256']
    for metrics in [enc['metrics'],enc['repeat_metrics']]:
        assert metrics['passed'] and all(metrics['checks'].values())
        assert metrics['decoded_samples']==metrics['source_samples']==p['decoded_samples'] and metrics['sample_rate']==44100
        assert metrics['si_sdr_db']>=24 and metrics['decoded_peak_db']<0
        assert abs(metrics['container_duration']-p['decoded_samples']/44100)<=1/44100
        seam=metrics['seam'];assert seam['from_sample']==p['decoded_samples']-1 and seam['to_sample']==p['loop_start_sample']
        assert seam['decoded_jump']<=seam['adjacent_range_peak']*1.05+2
        assert all(not r['criterion_applies'] or abs(r['difference_db'])<=1.5 for r in seam['edge_rms'])
        meta=metrics['mp3_metadata'];assert meta['marker'] in ['Info','Xing']
        assert meta['mpeg_audio_frames']*1152-meta['encoder_delay_samples']-meta['encoder_padding_samples']==meta['metadata_decoded_samples']==p['decoded_samples']
        assert sha(evidence/metrics['decode_log'])==metrics['decode_log_sha256']
    assert not entry['accepted_for_release'] and not entry['listening_accepted'] and not entry['browser_accepted']
    assert (OUT/p['file']).stat().st_size<(OUT/p['wav']).stat().st_size*.3
    return {'status':'passed','manifest':str(path.relative_to(ROOT)),'manifest_sha256':sha(path),
            'file':p['file'],'sha256':entry['sha256'],'sample_rate':p['sample_rate'],
            'loop_start_sample':p['loop_start_sample'],'loop_end_sample':p['loop_end_sample'],
            'loop_samples':p['loop_samples'],'decoded_samples':p['decoded_samples'],
            'si_sdr_db':enc['metrics']['si_sdr_db'],'scope':'separate76 compound source and offline MP3 delivery; browser acceptance separate'}


def main():
    manifest=json.loads((OUT/'music-distribution-manifest.json').read_text())
    playback=json.loads((ROOT/'games/basketball/assets/audio-playback.json').read_text())['music']
    historical={59,60,61,62,63,64,65,66,75}
    assert set(map(int,playback))==historical|{76}
    assert {r['id'] for r in manifest['tracks']}==historical
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
    music76=verify_music76(playback)
    report={'status':'passed','scope':'immutable historical nine-track batch plus separately verified76 compound; no recapture/re-encode or human/device acceptance',
        'tracks':len(manifest['tracks']),'source_bytes':total_source,'encoded_bytes':total_encoded,
        'reduction_percent':100*(1-total_encoded/total_source),'registered_tracks':len(playback),'music76':music76,
        'historical_batch_ids':sorted(historical),'accepted_for_release':False,'listening_accepted':False}
    (OUT/'music-distribution-verification.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))


if __name__=='__main__':main()
