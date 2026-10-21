"""Read-only design oracle: explicit candidate initial changes, unmodified ROM.
This does not implement a repair or assert that the original selects either
candidate from the stuck state. The unchanged control remains stuck.
"""
import hashlib, json
from probe_rest_stall import ROOT, ROM, PRG, encode, run, SoundMemory
from verify_pass_floor_original import put
from extract_world_actions import decode_world_action_state


def main():
    cases=[('home0','.local/basketball/rea/campaign-home-stall/solo/first-entry/stalled.json'),('newyork1','.local/basketball/rea/ball-rest-stall/first-entry/stalled.json'),('court7','.local/basketball/rea/court-seven-stall/first-entry/stalled.json')]
    results=[]
    for name,path in cases:
        data=(ROOT/path).read_bytes();snapshot=json.loads(data)
        for choice in ['unchanged','gravity256-only','original-action10']:
            raw=encode(snapshot);before=decode_world_action_state(raw)['ball'];patch=[]
            if choice=='gravity256-only':
                put(raw,256,0x4cc,0x4d9);patch=[dict(field='gravity',before=before['body']['gravity'],after=256)]
            elif choice=='original-action10':
                raw[0x518]=10;patch=[dict(field='action/entered',before=before['action']|(128 if before['entered'] else 0),after=10)]
            machine=SoundMemory(PRG,raw);rows=[]
            for n in range(96):
                ball,sounds=run(machine)
                rows.append(dict(call=n+1,ball=ball,sounds=sounds,holder=machine[0x53f]))
            firstGround=next((r['call'] for r in rows if not r['ball']['airborne'] and r['ball']['body']['position']['height']==0),None)
            if choice=='unchanged':assert firstGround is None
            else:assert firstGround is not None
            results.append(dict(court=name,source=path,sourceSHA256=hashlib.sha256(data).hexdigest(),choice=choice,declaredInitialPatch=patch,before=before,firstGround=firstGround,rows=rows))
    doc=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),scope='controlled original ball-only E149 calls after a declared candidate initial patch; proposals only, no production behavior change, no full-world/catch/score proof',naturalRecovery=False,originalInstructionsUnmodified=True,stepsPerCandidate=96,cases=results)
    out=ROOT/'.local/basketball/rea/suspended-repair-proposal';out.mkdir(parents=True,exist_ok=True)
    (out/'paths.json').write_text(json.dumps(doc,indent=2)+'\n')
    for row in results:
        print(row['court'],row['choice'],'firstGround',row['firstGround'],'first',row['rows'][0]['ball']['body'],'lastAction',row['rows'][-1]['ball']['action'])

if __name__=='__main__':main()
