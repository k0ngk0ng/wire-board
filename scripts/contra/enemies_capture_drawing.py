"""Record enemy state plus real graphics admission/packets during natural play."""
import gzip,hashlib,json,sys
from pathlib import Path
import enemies_capture as original
from capture_reference import run
ROOT,BASE=original.ROOT,original.BASE

def drawing_hook():
    h=original.HOOK.replace('FIELDS','{'+','.join(hex(a)for _,a in original.FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in original.WORLD)+'}')
    marker='local function state()'
    h=h.replace(marker,"local function hx(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end\n"+marker)
    old="..id..'}'"
    new="..id..',\"graphics\":{\"used\":'..read(0x21)..',\"mode\":'..read(0x23)..',\"bytes\":\"'..hx(0x700,256)..'\",\"supertiles\":\"'..hx(0x600,128)..'\"}}'"
    assert old in h;h=h.replace(old,new)
    old='"after":%s}\\n'
    assert old in h;h=h.replace(old,'"after":%s,"sounds":[%s]}\\n')
    old='p.before,state()))'
    assert old in h;h=h.replace(old,"p.before,state(),table.concat(p.sounds,',')))")
    h=h.replace('id=count,kind=name','sounds={},id=count,kind=name')
    h=h.replace('function finish_probe()',"memory.registerexec(0xf9bc,1,function()for _,p in ipairs(pending)do if p.kind=='enemyStep' then p.sounds[#p.sounds+1]=memory.getregister('a')end end end)\nfunction finish_probe()")
    return h

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1'
    scenarios=[ROOT/'.local/contra-enemies/oracle/natural-enemies-v1.json',ROOT/'.local/contra-weapons/pickup/runs/natural-machinegun-01/scenario.json']
    all_rows=[];sources=[];maps=[];next_map=0
    for i,path in enumerate(scenarios):
        spec=json.loads(path.read_text());spec['name']=f'enemy-drawing-{label}-{i}';spec['hooks_lua']=drawing_hook();spec['load_state']='coop-play'
        p=BASE/(spec['name']+'.json');p.write_text(json.dumps(spec,indent=2)+'\n');out=run(p)
        own_maps=[json.loads(l)for l in (out/'enemy-maps.jsonl').read_text().splitlines()];ids={m['id']:next_map+j+1 for j,m in enumerate(own_maps)}
        for m in own_maps:maps.append({'id':ids[m['id']],'bytes':m['bytes']})
        next_map+=len(own_maps)
        rows=[json.loads(l)for l in (out/'enemies.jsonl').read_text().splitlines()]
        rows=[r for r in rows if r['kind']=='enemyStep' and r['before']['enemies'][r['actor']][11]in [2,18,0]]
        for row in rows:
            row['scenario']=i
            for stage in ('before','after'):row[stage]['map']=ids[row[stage]['map']]
        all_rows.extend(rows);sources.append({'scenario':spec,'source':json.loads((out/'source.json').read_text()),'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()})
    fixture={'source_is_original_execution':True,'interventions':'none; copied natural controller timelines and read-only callbacks','sources':sources,'records':all_rows,'maps':maps}
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    path=ROOT/'games/contra/engine/testdata/original-enemy-drawing.json.gz'
    if path.exists():
        old=json.loads(gzip.decompress(path.read_bytes()));assert old['records']==all_rows and old['maps']==maps, 'drawing replay differs'
    else:path.write_bytes(packed)
    counts={}
    for row in all_rows:
        e=row['before']['enemies'][row['actor']];key=f"{e[11]}:{e[4]}";counts[key]=counts.get(key,0)+1
    print(json.dumps({'records':len(all_rows),'states':counts,'fixture_sha256':hashlib.sha256(path.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
