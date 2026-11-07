"""Observe complete AA85 generation and the actual last writer of burst seed.

Natural input only. Failure cases: timing/scan order drift, missing carry into
RNG selection, first-stage forced right side, lower-slot cap, three-body burst
versus screen generation counter, and replacing inherited work parity with zero.
"""
import gzip,hashlib,json,sys
from pathlib import Path
import enemies_capture as original
from capture_reference import run
ROOT,BASE=original.ROOT,original.BASE

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1'
    world=original.WORLD+[('weaponStrength',0x2f),('burstSeed',6)]
    hook=original.HOOK.replace('FIELDS','{'+','.join(hex(a)for _,a in original.FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in world)+'}')
    tracking=r'''
local writer={pc=-1,bank=-1,frame=-1,value=memory.readbyte(6),phase='initial-state'}
local phase='before-first-phase';local f=assert(io.open(out..'/burst-seed-source.jsonl','w'))
for _,entry in ipairs({{0xd296,'players-projectiles'},{0xfa3a,'scroll'},{0xfa1a,'enemies'},{0xfa22,'scripted-spawn'},{0xfa32,'random-spawn'},{0xc83b,'palette'},{0x919f,'sprite-output'}})do
 local name=entry[2];memory.registerexec(entry[1],1,function()phase=name end)
end
memory.registerwrite(6,1,function(a,n,v)
 writer={pc=memory.getregister('pc'),bank=memory.readbyte(0x8000),frame=emu.framecount(),value=v,phase=phase}
end)
-- FA32 runs immediately before AA85 and does not write $06 itself.
memory.registerexec(0xfa32,1,function()
 f:write(string.format('{"frame":%d,"seed":%d,"lastWriter":{"pc":%d,"bank":%d,"frame":%d,"value":%d,"phase":"%s"}}\n',emu.framecount(),memory.readbyte(6),writer.pc,writer.bank,writer.frame,writer.value,writer.phase))
 phase='random-spawn'
end)
local prior=finish_probe;function finish_probe()prior();f:close()end
'''
    timelines=[ROOT/'.local/contra-enemies/oracle/natural-enemies-v1.json',ROOT/'.local/contra-weapons/pickup/runs/natural-machinegun-01/scenario.json']
    records=[];maps=[];sources=[];offset=0;writer_counts={}
    for i,timeline in enumerate(timelines):
        spec=json.loads(timeline.read_text());spec['name']=f'enemy-generators-{label}-{i}';spec['hooks_lua']=hook+'\n'+tracking;spec['load_state']='coop-play'
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
        own=[json.loads(l)for l in (out/'enemy-maps.jsonl').read_text().splitlines()];ids={m['id']:offset+j+1 for j,m in enumerate(own)};offset+=len(own)
        maps.extend({'id':ids[m['id']],'bytes':m['bytes']}for m in own)
        provenance={r['frame']:r for r in (json.loads(l)for l in (out/'burst-seed-source.jsonl').read_text().splitlines())}
        rows=[json.loads(l)for l in (out/'enemies.jsonl').read_text().splitlines()];rows=[r for r in rows if r['kind']=='soldierSpawn']
        for r in rows:
            for side in ('before','after'):r[side]['map']=ids[r[side]['map']]
            r['scenario']=i;r['burst_source']=provenance[r['frame']]
            assert r['before']['world'][30]==r['burst_source']['seed']
            writer=r['burst_source']['lastWriter'];key=f"{writer['phase']}:{writer['bank']:02x}:{writer['pc']:04x}";writer_counts[key]=writer_counts.get(key,0)+1
        records.extend(rows);sources.append({'scenario':spec,'source':json.loads((out/'source.json').read_text()),'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()})
    result={'source_is_original_execution':True,'interventions':'none; original natural controller timelines','sources':sources,'world_fields':[{'name':n,'address':hex(a)}for n,a in world],'records':records,'maps':maps,'burst_writer_counts':writer_counts}
    packed=gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0);path=ROOT/'games/contra/engine/testdata/original-enemy-generators.json.gz'
    if path.exists():
        old=json.loads(gzip.decompress(path.read_bytes()));assert old['records']==records and old['maps']==maps, 'generation replay differs'
    else:path.write_bytes(packed)
    print(json.dumps({'calls':len(records),'writers':writer_counts,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}))
if __name__=='__main__':main()
