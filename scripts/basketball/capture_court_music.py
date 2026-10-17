"""Source-bound, resumable offline captures for the seven additional court tracks.

Never recaptures completed matching runs or NY66. A live process must be checked
before rerunning this script; incomplete captures have no accepted metadata.
"""
from pathlib import Path
import csv
import fcntl
import hashlib
import json
import os
import sys
import wave
from extract_audio import BASE,OUT,ROOT,SOURCE_SHA,carrier,record

IDS=[59,60,61,62,63,64,65,75,76]


def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()


def main():
    lock=(OUT/'court-capture.lock').open('w')
    try:fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
    except BlockingIOError:raise RuntimeError('A live court audio capture holds the workspace lock; do not restart it.')
    frames=int(sys.argv[1]) if len(sys.argv)>1 else 20000
    selected=[int(i) for i in sys.argv[2:]] or IDS
    assert set(selected)<=set(IDS) and 10000<=frames<=300000
    rom=(BASE/'basketball.nes').read_bytes();assert hashlib.sha256(rom).hexdigest()==SOURCE_SHA
    journal=OUT/'court-capture-journal.json'
    data=json.loads(journal.read_text()) if journal.exists() else {'schema':1,'rom_sha256':SOURCE_SHA,'runs':{}}
    for sound_id in selected:
        nsf=OUT/f'sound-{sound_id:03}.nsf'
        content=carrier(rom,sound_id)
        if nsf.exists():assert nsf.read_bytes()==content
        else:nsf.write_bytes(content)
        for repeat in [1,2]:
            name=f'court-music-{sound_id:03}-{frames}-run-{repeat}'
            folder=OUT/name;info=folder/'capture-info.json'
            expected={'id':sound_id,'frames':frames,'volume':192,'rom_sha256':SOURCE_SHA,'nsf_sha256':sha(nsf)}
            if info.exists():
                saved=json.loads(info.read_text());assert saved['settings']==expected
                assert all(sha(OUT/path)==digest for path,digest in saved['sha256'].items())
                data['runs'][name]=saved
                print('reused',name,flush=True)
                continue
            print('capturing',name,flush=True)
            run=record(nsf,folder,OUT/'wave_bridge.dylib',frames,192)
            assert (folder/'driver-ram.bin').stat().st_size==frames*231
            calls=list(csv.DictReader((folder/'calls.csv').open()))
            assert [int(r['value']) for r in calls if r['kind']=='init']==[0,sound_id]
            with wave.open(str(folder/'full.wav')) as w:assert abs(w.getnframes()/44100-frames/60.0988)<.1
            saved={'settings':expected,'files':run,'sha256':{path:sha(OUT/path) for path in run.values()}}
            info.write_text(json.dumps(saved,indent=2)+'\n')
            data['runs'][name]=saved;journal.write_text(json.dumps(data,indent=2)+'\n')
            print('completed',name,flush=True)
    journal.write_text(json.dumps(data,indent=2)+'\n')


if __name__=='__main__':main()
