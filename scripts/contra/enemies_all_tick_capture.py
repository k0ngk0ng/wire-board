"""Record complete native enemy phases for stage dispatch integration.

The selected original entity initialization, graphics budget, player poses and
room counters are declared inputs. The complete E54F pool iteration, child
creation, contact, score, graphics and sound output is observed, never injected.
These controlled calls do not establish natural campaign completion.
"""
import ast,gzip,hashlib,json,os,shutil,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-all/tick-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
from enemies_all_data import inventory

def literal(path,name):
    for node in ast.parse(path.read_text()).body:
        if isinstance(node,ast.Assign)and any(isinstance(t,ast.Name)and t.id==name for t in node.targets):return ast.literal_eval(node.value)
    raise ValueError(name)

PLAYER=literal(ROOT/'scripts/contra/static_capture_playerstep.py','PLAYER_FIELDS')
ENEMY=literal(ROOT/'scripts/contra/enemies_capture.py','FIELDS')
WORLD=literal(ROOT/'scripts/contra/enemies_collision_tick_capture.py','WORLD')+[
 ('delayLow',0x2a),('delayHigh',0x2b),('generationBias',6),('screenCleared',0x6a),
 ('paletteTimer',0x74),('autoScrollA',0x75),('autoScrollB',0x76),('tankScroll',0x77),('palettePause',0x78),('jointScroll',0x7f),
 ('bossScrollComplete',0x84),('bossParts',0x85),('cores',0x86),('plating',0x87),('attackRounds',0x88),('redSoldier',0x89),('grenadeLauncher',0x8a),('alienAimIndex',0x8b),
 ('presentationPace',0x5d),('locatorLow',0x12),('locatorHigh',0x13),('paletteRequest',0x36),('paletteCycle',0x72)]
HOOK=literal(ROOT/'scripts/contra/enemies_collision_tick_capture.py','HOOK')
HOOK=HOOK.replace("hex(0x680,128)..'\"}'", "hex(0x680,128)..'\",\"header\":\"'..hex(0x49,15)..'\",\"palette\":\"'..hex(0x7c0,32)..'\"}'")
HOOK=HOOK.replace('biasWrites={}}','biasWrites={},workWrites={}}')
HOOK=HOOK.replace("if captureBias then audio=audio..", "audio=audio..',\"terrainWorkWrites\":['..table.concat(p.workWrites,',')..']'\n   if captureBias then audio=audio..")
HOOK += '''
for address=0x12,0x13 do memory.registerwrite(address,1,function(a,size,value)
 if active then active.workWrites[#active.workWrites+1]=string.format('{"pc":%d,"bank":%d,"slot":%d,"address":%d,"value":%d}',memory.getregister('pc'),read(0x8000),read(0x83),a,value)end
end)end
memory.registerexec(0xc593,1,function()
 if active then active.requests[#active.requests+1]=string.format('{"x":%d,"y":%d,"code":%d,"used":%d,"tile":true}',memory.getregister('a'),memory.getregister('y'),memory.readbyte(0x10),memory.readbyte(0x21))end
end)
'''

def cases(level):
    d=inventory()['stages'][level];common=[[0,1,2,3,4,5,6,7],[0,1,8,10],[0,1,2,3,4,5,6,7,11,12],[0,1,8,10],[0,1,2,3,5,6,11,12,14,15],[0,1,2,3,5,6,14,15],[0,1,2,3,5,6,11,14,15],[0,1,3]][level]
    specific=list(range(16,16+len(d['specificTables'])))
    if level==1:specific=[k for k in specific if k<30]
    out=[]
    for kind in common+specific:
        for variant in range(3):
            p=d['properties'][kind]['values']
            values=[1,100,160,0,1,0,0,0,0,0,0,kind,0,p[3],0,0,p[2],p[1],p[0],0,0,0,0,0]
            writes={str(a+i):0 for _,a in ENEMY for i in range(16)}
            for i,(_,a)in enumerate(ENEMY):writes[str(a+15)]=values[i]
            # Full pools contain valid ordinary bullets, which also receive
            # their own native iteration after the selected highest slot.
            if variant==2:
                for slot in range(15):
                    for a,v in [(0x4b8,1),(0x528,1),(0x30a,1),(0x33e,220),(0x324,80),(0x598,0x81)]:writes[str(a+slot)]=v
            scene=0 if level not in [1,3]else 0x80 if kind in [16,27,28,29]else 1
            for a,v in [(0x30,level),(0x31,0),(0x2f,0),(0x40,scene),(0x41,int(level==2)),(0x68,variant&1),(0x64,0),(0x65,0),(0x21,63 if variant==1 else 0),(0x23,0),(0x84,1),(0x74,0),(0x86,2),(0x87,0),(0x88,0),(0x89,0),(0x8a,0),(0x8b,0),(0x8e,1),(0x80,int(d['specificDispatch'],16)&255),(0x81,int(d['specificDispatch'],16)>>8)]:writes[str(a)]=v
            for slot in range(2):
                for a,v in [(0x90,1),(0x38,0),(0xae,255),(0x334,64+128*slot),(0x31a,176)]:writes[str(a+slot)]=v
            for a in range(0x368,0x4b8):writes[str(a)]=0
            out.append({'name':f'level{level+1}-type{kind:02x}-variant{variant}','writes':writes})
    return out

def main():
    level=int(sys.argv[1]);label=sys.argv[2]if len(sys.argv)>2 else'v1';assert 0<=level<8
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    source=ROOT/('.local/contra/states/coop-play.fcs'if level==0 else f'.local/contra-campaign/states/stage-{level+1}-v2.fcs');target=BASE/f'states/stage-{level+1}.fcs'
    if target.exists():assert source.read_bytes()==target.read_bytes()
    else:shutil.copyfile(source,target)
    declared=cases(level)
    hook=HOOK.replace('PLAYERS','{'+','.join(hex(a)for _,a in PLAYER)+'}').replace('ENEMIES','{'+','.join(hex(a)for _,a in ENEMY)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}').replace('CAPTURE_BIAS','true')
    hook+='\nlocal cases={'+','.join('{'+','.join(f'[{a}]={v}'for a,v in c['writes'].items())+'}'for c in declared)+'}\n'
    hook+='''function configure_phase(index)
 local c=assert(cases[index]);local saved={}
 for a=0,2047 do if a<0x100 or a>=0x200 then saved[a]=memory.readbyte(a)end end
 for a,v in pairs(c)do memory.writebyte(a,v)end
 return saved
end
'''
    spec={'name':f'all-tick-stage-{level+1}-{label}','load_state':f'stage-{level+1}','frames':len(declared),'segments':[],'captures':[],'hooks_lua':hook,'timeout_seconds':150}
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(l)for l in(out/'enemy-ticks.jsonl').read_text().splitlines()];assert len(rows)==len(declared)
    fixture={'source_is_original_execution':True,'scope':f'stage-{level+1}-visual','interventions':__doc__,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':declared,
             'player_fields':[{'name':n,'address':hex(a)}for n,a in PLAYER],'enemy_fields':[{'name':n,'address':hex(a)}for n,a in ENEMY],'world_fields':[{'name':n,'address':hex(a)}for n,a in WORLD],'records':rows}
    target=ROOT/f'games/contra/engine/testdata/original-enemy-collision-tick-stage-{level+1}-visual.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==rows
    else:target.write_bytes(gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'level':level,'phases':len(rows),'entityCalls':sum(len(r['slots'])for r in rows),'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
