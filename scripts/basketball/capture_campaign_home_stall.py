"""Declared home-campaign origin then original full-machine execution.
All files stay in the repository research directories. This is not a natural
menu/campaign completion claim and does not repair the original suspended ball.
"""
import argparse, hashlib, json
from capture_reference import ROOT, BASE, run
from capture_rest_stall_original import CONFIG
from encode_world_probe import encode_world_probe
from extract_world_loop import loop_state
from compare_rest_stall_worlds import normalized, differences
from probe_rest_stall import encode, run as ball_run, SoundMemory, PRG, ROM
from verify_court_profiles import image, profiles


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--mode',choices=['solo','coop'],default='solo');parser.add_argument('--cold',action='store_true');args=parser.parse_args()
    base=ROOT/'.local/basketball/rea/campaign-home-stall'/args.mode
    rows=json.loads((base/'first-entry/history.json').read_text())
    out=base/'cold' if args.cold else base;out.mkdir(parents=True,exist_ok=True)
    origin=json.loads((base/'cold-origin.json').read_text()) if args.cold else rows[0]['Before'];assert origin['config']['match']['Court']==0
    assert origin['config']['match']['Mode']==(0 if args.mode=='solo' else 1)
    (out/'controlled-origin.json').write_text(json.dumps(origin)+'\n')
    patch,names=encode_world_probe(origin)
    (out/'controlled-patch.json').write_text(json.dumps([dict(address=f'{a:04X}',value=v,field=names[a]) for a,v in sorted(patch.items())],indent=2)+'\n')
    calls=[];chain=[];m=SoundMemory(PRG,encode(rows[0]['Before']));grid=profiles(image())[0]
    for row in rows:
        actual,sounds=ball_run(SoundMemory(PRG,encode(row['Before'])));want=row['After']['playing']['world']['Preparation']['Actions']['ball']
        calls.append(dict(video=row['Video'],match=actual==want,actual=actual,want=want,sounds=sounds))
        actual,sounds=ball_run(m);chain.append(dict(video=row['Video'],match=actual==want,actual=actual,want=want,sounds=sounds))
    assert all(r['match'] for r in calls+chain)
    support=[]
    for row in rows:
        b=row['After']['playing']['world']['Preparation']['Actions']['ball'];p=b['body']['position'];cell=grid[(p['depth']//256)//8][(p['x']//256)//8]
        support.append(dict(video=row['Video'],ball=b,support=cell&248,cell=cell))
    (out/'original-approach.json').write_text(json.dumps(dict(romSHA256=hashlib.sha256(ROM).hexdigest(),scope='isolated complete original ball calls and one continuous ball-only chain; not full-world proof',calls=calls,chain=chain,support=support),indent=2)+'\n')
    config=CONFIG.replace('memory.writebyte(0x587,5)','memory.writebyte(0x587,1)').replace('"teams":[0,5]','"teams":[0,1]').replace('memory.writebyte(0x58b,1)','memory.writebyte(0x58b,0)').replace('"court":1','"court":0')
    writes=';'.join(f'memory.writebyte({a},{v})' for a,v in sorted(patch.items()))
    hook=r'''
local aligned=false;local count=0
local trace=assert(io.open(out..'/world-after.ram','wb'));local index=assert(io.open(out..'/world-after.jsonl','w'))
local function raw()local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i))end;return table.concat(t)end
memory.registerexec(0xd355,1,function()
 if not aligned and court and memory.readbyte(0x589)==129 and memory.readbyte(0x58b)==0 then
  writefile('alignment-before.ram',raw());PATCH
  aligned=true;writefile('alignment-after.ram',raw());writefile('alignment-frame.json',string.format('{"frame":%d}',emu.framecount()))
 end
 if aligned and OBSERVE then trace:write(raw());index:write(string.format('{"frame":%d,"index":%d}\n',emu.framecount(),count));count=count+1 end
end)
function finish_probe()assert(aligned);trace:close();index:close()end
'''.replace('PATCH',writes)
    paths=[]
    for label,observe in [('reference','false'),('observed','true')]:
        spec=json.loads((BASE/'opening-from-menu.json').read_text());spec.pop('save_slot',None)
        spec.update(name='campaign-home-stall-'+args.mode+('-cold' if args.cold else '')+'-'+label,frames=13000,captures=[1500,3000,8000,12000,13000],hooks_lua=config+'\n'+hook.replace('OBSERVE',observe))
        path=out/(label+'-spec.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path);paths.append(BASE/'runs'/spec['name'])
    reference,observed=paths
    assert(reference/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes()
    for n in spec['captures']:assert(reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
    first=(observed/'alignment-after.ram').read_bytes();initial=loop_state(first,origin['playing']['world']['Updates'])
    ds=list(differences(normalized(initial),normalized(origin['playing']['world'])));assert not ds,ds
    raw=(observed/'world-after.ram').read_bytes();index=[json.loads(x) for x in (observed/'world-after.jsonl').read_text().splitlines()];assert len(raw)==2048*len(index)
    stalls=[]
    for i,row in enumerate(index):
        r=raw[i*2048:(i+1)*2048]
        if r[0x589]&127==1 and r[0x53f]==6 and r[0x518]&127==0 and r[0x5fd]&128 and (r[0xbc] or r[0xc9]) and not any(r[a] for a in [0x47e,0x48b,0x498,0x4a5,0x4b2,0x4bf,0x4cc,0x4d9]):
            stalls.append(dict(**row,x=r[0x6e]+256*r[0x7b]+65536*r[0x88],depth=r[0x95]+256*r[0xa2],height=r[0xaf]+256*r[0xbc]+65536*r[0xc9]))
    r=raw[-2048:]
    result=dict(scope='one declared canonical world origin, no later gameplay writes; not natural menu coverage',mode=args.mode,originGoVideo=origin['videoFrames'],initialUpdates=origin['playing']['world']['Updates'],initialSemanticRoundTrip=True,referenceRAMAndScreenshotsMatch=True,worlds=len(index),stalledWorlds=len(stalls),firstStalls=stalls[:5],lastStalls=stalls[-5:],lastPhase=r[0x589],lastClock=list(r[0x6c2:0x6c6]),lastScores=[r[0x166]+256*r[0x168],r[0x167]+256*r[0x169]],source=json.loads((observed/'source.json').read_text()),traceSHA256=hashlib.sha256(raw).hexdigest(),ramSHA256=hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest())
    (out/'original-world-result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))

if __name__=='__main__':main()
