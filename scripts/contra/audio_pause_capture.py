"""Verify natural shooting/Start pause/resume audio phase ordering.

Existing effect noise must freeze; pause cue74 alone keeps advancing. The
resume-world tick may enqueue the first shot while the previous audio pause
byte still reflects the preceding audio update; those phases must not merge.
"""
import argparse
import hashlib
import json
from audio_trace import BASE,ROOT,HOOK
from capture_reference import run


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--label',default='v1');args=parser.parse_args()
    spec={'name':f'audio-pause-shot-{args.label}','load_state':'coop-play','frames':160,'captures':[],
          'segments':[{'start':5,'end':5,'keys':['B']},{'start':10,'end':12,'keys':['start']},{'start':100,'end':100,'keys':['start','B']}], 'hooks_lua':HOOK}
    p=BASE/(spec['name']+'.json');p.write_text(json.dumps(spec,indent=2)+'\n')
    out=BASE/'runs'/spec['name']
    if out.exists():assert json.loads((out/'scenario.json').read_text())==spec
    else:out=run(p)
    raw=(out/'ram.bin').read_bytes();rows=[]
    for n in range(160):
        r=raw[n*2048:(n+1)*2048];rows.append({'frame':n+1,'gamePaused':r[0x25],'audioPaused':r[0x12f],
            'slots':list(r[0x106:0x10c]),'commandTimers':list(r[0x100:0x106])})
    paused=[r for r in rows if r['audioPaused']]
    assert [r['frame']for r in paused]==list(range(11,101))
    stable=[r for r in paused if r['gamePaused']]
    assert all(r['slots'][5]==26 for r in stable)
    assert len({r['commandTimers'][5]for r in stable})==1
    assert all(r['slots'][4]==0 for r in stable if r['frame']>=42)
    assert rows[99]['gamePaused']==0 and rows[99]['audioPaused']==1 and rows[99]['slots'][4:6]==[26,26]
    assert rows[100]['audioPaused']==0 and rows[100]['slots'][4:6]==[26,26]
    report={'passed':True,'scope':'Natural Start/shoot inputs; no ROM/RAM writes in hooks','source':json.loads((out/'source.json').read_text()),
        'scenario':spec,'ramSHA256':hashlib.sha256(raw).hexdigest(),'requests':[json.loads(r)for r in (out/'sound-requests.jsonl').read_text().splitlines()],
        'rows':rows,'conclusion':'Only pause cue74 progresses while paused; prior noise26 timer freezes; cue ends during pause and first resumed shot is accepted in the next audio phase.'}
    target=ROOT/'docs/research/nes-contra/evidence/audio-natural-pause.json'
    if target.exists():assert json.loads(target.read_text())['rows']==rows
    else:target.write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'passed':True,'pausedAudioFrames':len(paused),'noiseTimer':stable[0]['commandTimers'][5]}))


if __name__=='__main__':main()
