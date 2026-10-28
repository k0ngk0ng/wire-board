"""Eight original stage entries: complete world calls with controller input only.

Failure inventory: losing the source state's declared stage-index intervention;
injecting lives/HP/outcomes; closing at an internal F8DB during a blocking copy;
missing sound/reset order; silently accepting fewer than800 complete calls;
overwriting a primary recording; or mistaking these stage entries for a natural
eight-stage campaign completion. No memory-write API is used by this observer.
"""
from pathlib import Path
import argparse,gzip,hashlib,json,os,shutil
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-world-eight-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

HOOK=r'''
local log=assert(io.open(out..'/world-frames.jsonl','w'))
local states=assert(io.open(out..'/world-frames.bin','wb'))
local pending=nil;local ordinal=0;local skipped=0;local internalEnds=0
memory.registerexec(0xc001,1,function()
 if ordinal>=800 then return end
 if memory.readbyte(0x18)~=5 then skipped=skipped+1;return end
 assert(not pending,'nested outer C001')
 local sp=memory.getregister('s')
 pending={frame=emu.framecount(),sp=AND(sp+2,255),beforeSP=sp,before=memory.readbyterange(0,2048),commands={},internalEnds=0}
end)
memory.registerexec(0xf9bc,1,function()
 if pending then pending.commands[#pending.commands+1]=string.format('{"kind":"sound","id":%d}',memory.getregister('a'))end
end)
memory.registerexec(0xf9d4,1,function()
 if pending then pending.commands[#pending.commands+1]='{"kind":"reset"}'end
end)
memory.registerexec(0xf8db,1,function()
 if not pending then return end
 if memory.getregister('s')~=pending.sp then pending.internalEnds=pending.internalEnds+1;internalEnds=internalEnds+1;return end
 local p=pending;pending=nil
 states:write(p.before);states:write(memory.readbyterange(0,2048))
 log:write(string.format('{"index":%d,"emulatorFrame":%d,"afterEmulatorFrame":%d,"beforeSP":%d,"afterSP":%d,"ignoredInternalF8DB":%d,"commands":[%s]}\n',ordinal,p.frame,emu.framecount(),p.beforeSP,memory.getregister('s'),p.internalEnds,table.concat(p.commands,',')))
 ordinal=ordinal+1
end)
function finish_probe()
 assert(not pending,'unfinished outer C001');log:close();states:close()
 writefile('observer.json',string.format('{"completeCalls":%d,"skippedNonGameCalls":%d,"ignoredInternalF8DB":%d}',ordinal,skipped,internalEnds))
end
'''

def main():
    p=argparse.ArgumentParser();p.add_argument('--label',required=True);p.add_argument('--stage',type=int,choices=range(1,9));args=p.parse_args()
    origin=ROOT/'docs/research/nes-contra/evidence/campaign-controlled-entries.json'
    entries={e['stage']:e for e in json.loads(origin.read_text())['entries']};reports=[]
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    for stage in ([args.stage]if args.stage else range(1,9)):
        src=ROOT/f'.local/contra-campaign/states/stage-{stage}-v2.fcs';dst=BASE/f'states/stage-{stage}.fcs'
        if dst.exists():assert src.read_bytes()==dst.read_bytes()
        else:shutil.copyfile(src,dst)
        # A complete C001 can span an internal display wait. Keep ordinary
        # controller input running long enough to observe800 complete returns;
        # the observer stops after those800 calls without changing the game.
        frames=1000;segments=[]
        if stage in [2,4]:
            # Both players approach the central lane, then periodically press
            # forward; the original decides whether the room is already clear.
            segments=[{'start':1,'end':16,'player':1,'keys':['right']},{'start':1,'end':16,'player':2,'keys':['left']}]
            segments += [{'start':n,'end':min(n+30,frames),'player':actor,'keys':['up']}for actor in [1,2]for n in [180,380,580,780,980]]
        else:segments=[{'start':1,'end':frames,'player':actor,'keys':['right']}for actor in [1,2]]
        segments += [{'start':n,'end':n,'player':actor,'keys':['B']}for actor in [1,2]for n in range(1,frames+1,4)]
        segments += [{'start':n,'end':n+1,'player':actor,'keys':['A']}for actor in [1,2]for n in range(60,frames,90)]
        spec={'name':f'world-eight-stage-{stage}-{args.label}','load_state':f'stage-{stage}','frames':frames,'segments':segments,'captures':[1,400,800],'hooks_lua':HOOK,'timeout_seconds':120}
        assert 'memory.write'not in HOOK and 'memory.setregister'not in HOOK
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
        rows=[json.loads(line)for line in(out/'world-frames.jsonl').read_text().splitlines()];raw=(out/'world-frames.bin').read_bytes()
        assert len(raw)==len(rows)*4096
        phases={};levels={}
        for i,row in enumerate(rows):
            before=raw[i*4096:i*4096+2048];after=raw[i*4096+2048:(i+1)*4096]
            assert row['afterSP']==(row['beforeSP']+2)&255
            row['before']=before.hex();row['after']=after.hex()
            phase=str(before[0x2c]);phases[phase]=phases.get(phase,0)+1
            level=str(before[0x30]);levels[level]=levels.get(level,0)+1
        observer=json.loads((out/'observer.json').read_text())
        summary={'stage':stage,'frames':len(rows),'phases':phases,'levels':levels,**observer}
        print(json.dumps(summary),flush=True)
        assert len(rows)==800,(stage,'expected800 complete original calls',summary)
        data={'source_is_original_execution':True,'stage':stage,'scope':'Existing original stage-N-v2 state with its declared stage-index initialization override; this run only reads state and supplies fixed dual-controller inputs. No added lives, HP, enemy or outcome writes; not natural campaign completion.',
              'source':json.loads((out/'source.json').read_text()),'initialStateSHA256':hashlib.sha256(src.read_bytes()).hexdigest(),
              'initializationProvenance':entries[stage],'initializationManifestSHA256':hashlib.sha256(origin.read_bytes()).hexdigest(),
              'scenario':spec,'snapshot':'Full2KiB RAM immediately before C001 and at F8DB with stack equal to entryS+2; nested F8DB calls are ignored.',
              'inputTraceSHA256':hashlib.sha256((out/'inputs.csv').read_bytes()).hexdigest(),'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest(),
              'observer':observer,'phases':phases,'levels':levels,'frames':rows}
        target=ROOT/f'docs/research/nes-contra/evidence/world-eight-stage-{stage}-original.json.gz';repeated=target.exists()
        if repeated:
            old=json.loads(gzip.decompress(target.read_bytes()));assert old['frames']==rows and old['ramSHA256']==data['ramSHA256']and old['initialStateSHA256']==data['initialStateSHA256']
        else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
        reports.append(dict(summary,fixture=str(target.relative_to(ROOT)),sha256=hashlib.sha256(target.read_bytes()).hexdigest(),inputTraceSHA256=data['inputTraceSHA256'],ramSHA256=data['ramSHA256'],initialStateSHA256=data['initialStateSHA256'],repeatedIdentically=repeated))
    if args.stage is None:
        target=ROOT/'docs/research/nes-contra/evidence/world-eight-original-verification.json'
        target.write_text(json.dumps({'passed':True,'scope':'Native full-frame acquisition and optional independent repeat only; no independent World acceptance claim.','frames':sum(r['frames']for r in reports),'stages':reports},indent=2)+'\n')

if __name__=='__main__':main()
