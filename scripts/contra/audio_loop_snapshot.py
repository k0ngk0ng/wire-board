"""Copy and hash only the immutable music inputs used by loop derivation."""
import argparse
import json
from pathlib import Path
import shutil
from audio_loop_verify import ROOT,BASE,sha


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--ids');args=parser.parse_args()
    ids={int(n,0)for n in args.ids.split(',')}if args.ids else None
    full=json.loads((ROOT/'.local/contra-audio/export/manifest.json').read_text())
    stems=json.loads((ROOT/'.local/contra-audio/stems/manifest.json').read_text())
    byid={s['id']:s for s in stems['sounds']}
    out=BASE/'inputs';out.mkdir(parents=True,exist_ok=True)
    def snapshot(source,dest,expected=None):
        data=source.read_bytes()
        import hashlib
        digest=hashlib.sha256(data).hexdigest()
        if expected is not None:assert digest==expected,source
        if dest.exists():assert dest.read_bytes()==data
        else:dest.write_bytes(data)
        return str(dest.relative_to(ROOT)),digest
    songs=[]
    for sound in full['sounds']:
        if sound['kind'] not in ('music','loop-effect') or (ids is not None and sound['id']not in ids):continue
        source=ROOT/'.local/contra-audio/export'/sound['runs'][0]
        driver,dhash=snapshot(source/'driver.bin',out/f'driver-{sound["id"]}.bin')
        cursors,chash=snapshot(source/'samples.csv',out/f'samples-{sound["id"]}.csv')
        start,end=sound['loop']['sampleRange'];first,last=sound['loop']['driverFrames']
        song={'id':sound['id'],'label':sound['label'],'kind':sound['kind'],'originSample':sound['sampleRange'][0],
              'baseDriverFrames':[first,last],'baseSampleRange':[start,end],
              'periodTicks':last-first,'periodSamples':end-start,'captureFrames':sound['captureFrames'],
              'driverSource':str((source/'driver.bin').relative_to(ROOT)),'driverSnapshot':driver,'driverSHA256':dhash,
              'cursorSource':str((source/'samples.csv').relative_to(ROOT)),'cursorSnapshot':cursors,'cursorSHA256':chash,'stems':[]}
        for stem in byid[sound['id']]['stems']:
            if stem['channel']not in ('pulse1','pulse2','triangle'):continue
            source_path=ROOT/'.local/contra-audio/stems'/stem['file']
            target,digest=snapshot(source_path,out/stem['file'],stem['sha256'])
            song['stems'].append({'channel':stem['channel'],'source':str(source_path.relative_to(ROOT)),'snapshot':target,'sha256':digest})
        assert {s['channel']for s in song['stems']}==({'pulse1','pulse2'}if sound['id']==20 else {'pulse1','pulse2','triangle'});songs.append(song)
    if ids is not None:
        assert {s['id']for s in songs}==ids
        previous=json.loads((out/'index.json').read_text())
        songs=[s for s in previous['songs']if s['id']not in ids]+songs
    assert {s['id']for s in songs}=={20,70,74,78,82,86,90,94}
    index={'romSHA256':full['romSHA256'],'sourceManifestSHA256':sha(ROOT/'.local/contra-audio/export/manifest.json'),'songs':songs}
    (out/'index.json').write_text(json.dumps(index,indent=2)+'\n')
    print(json.dumps({'songs':len(songs),'stems':sum(len(s['stems'])for s in songs)}))


if __name__=='__main__':main()
