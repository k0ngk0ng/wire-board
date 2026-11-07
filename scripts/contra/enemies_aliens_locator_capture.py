"""Original mixed E54F producer-to-Guardian locator handoff, before host fix.

Failure inventory: terrain query results discarded; high-Y empty lookup omits
its writes; EB99 overflow replaces both bytes instead of only the probe-X byte;
prior accepted graphics address improperly survives a subsequent query; queued
graphics refusal suppresses Guardian collision clearing; slot order is changed.
Only declared phase inputs are staged. Original execution supplies all outputs.
"""
from pathlib import Path
import argparse,ast,gzip,hashlib,json,os,shutil

ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-aliens-locator'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE/'oracle')
from capture_reference import run

def assignment(path,name):
    """Read string/list declarations and their literal replacements via AST.

    Never import/execute the parent recorder or change its output directories.
    The closed evaluator admits only its literal(), path join, + and replace.
    """
    values={'ROOT':ROOT}
    def value(node):
        if isinstance(node,ast.Constant):return node.value
        if isinstance(node,(ast.List,ast.Tuple)):return ast.literal_eval(node)
        if isinstance(node,ast.Name):return values[node.id]
        if isinstance(node,ast.BinOp):
            a,b=value(node.left),value(node.right)
            if isinstance(node.op,ast.Div):return a/b
            if isinstance(node.op,ast.Add):return a+b
        if isinstance(node,ast.Call):
            if isinstance(node.func,ast.Name)and node.func.id=='literal':return assignment(value(node.args[0]),value(node.args[1]))
            if isinstance(node.func,ast.Attribute)and node.func.attr=='replace':return value(node.func.value).replace(*(value(a)for a in node.args))
        raise ValueError(ast.dump(node))
    for node in ast.parse(path.read_text()).body:
        if isinstance(node,ast.Assign)and any(isinstance(t,ast.Name)and t.id==name for t in node.targets):values[name]=value(node.value)
        elif isinstance(node,ast.AugAssign)and isinstance(node.target,ast.Name)and node.target.id==name and isinstance(node.op,ast.Add):values[name]+=value(node.value)
    return values[name]

PARENT=ROOT/'scripts/contra/enemies_all_tick_capture.py'
PLAYER=assignment(PARENT,'PLAYER');ENEMY=assignment(PARENT,'ENEMY');WORLD=assignment(PARENT,'WORLD')

def cases():
    specs=[]
    def add(category,x,y,scroll=(0,0,0),seed=0x2168,draw=False):
        for routine in (10,11):
            specs.append(dict(category=category,x=x,y=y,scroll=scroll,seed=seed,draw=draw,guardianRoutine=routine))
    for x in (80,147):
        for y in (32,100,208):
            for scroll in ((0,0,0),(17,23,1),(240,20,0)):add('query-normal',x,y,scroll)
    for x in (80,147):
        for y in (224,231):
            for scroll in ((0,0,0),(17,23,1)):add('query-high-y',x,y,scroll)
    for x in (64,67):
        for y in (240,255):
            for low in (0x60,0x68):add('probe-overflow',x,y,seed=0x2100|low)
    for y in (100,224):add('graphics-overwritten',147,y,draw=True)
    add('graphics-overflow',67,240,draw=True)
    out=[]
    for index,c in enumerate(specs):
        writes={str(a+i):0 for _,a in ENEMY for i in range(16)}
        def enemy(slot,**values):
            for name,address in ENEMY:writes[str(address+slot)]=values.get(name,0)
        enemy(13,kind=16,routine=c['guardianRoutine'],sprite=1,x=176,y=96,stateWidth=0x8d,scoreCollision=0x83,hp=100,variable2=1)
        overflow='overflow'in c['category']
        enemy(14,kind=5 if overflow else 1,routine=2,sprite=1,x=c['x'],y=c['y'],stateWidth=0x81,scoreCollision=1,hp=1,animationDelay=1)
        if c['draw']:enemy(15,kind=22,routine=6,sprite=1,x=176,y=96,stateWidth=0x81,scoreCollision=0x83,hp=100)
        for address,v in [(0x30,7),(0x31,0),(0x2f,0),(0x40,0),(0x41,0),(0x68,0),(0x64,0),(0x65,0),(0x21,10 if c['draw']else 64),(0x23,0),(0x59,0x80),(0x8e,1),(0xdf,2),(0x12,c['seed']&255),(0x13,c['seed']>>8),(0xfd,c['scroll'][0]),(0xfc,c['scroll'][1]),(0xff,c['scroll'][2])]:writes[str(address)]=v
        for actor in (0,1):
            for address,v in [(0x90,1),(0x38,0),(0xae,255),(0xb0,0),(0x334,64+128*actor),(0x31a,176)]:writes[str(address+actor)]=v
        for address in range(0x368,0x4b8):writes[str(address)]=0
        for address in range(0x680,0x700):writes[str(address)]=255
        out.append({'name':f"{c['category']}-r{c['guardianRoutine']}-{index+1}",'setup':c,'writes':writes})
    return out

def main():
    p=argparse.ArgumentParser();p.add_argument('--name',required=True);args=p.parse_args();oracle=BASE/'oracle'
    (oracle/'states').mkdir(parents=True,exist_ok=True)
    initial=ROOT/'.local/contra-campaign/states/stage-8-v2.fcs';state=oracle/'states/stage-8.fcs'
    if state.exists():assert initial.read_bytes()==state.read_bytes()
    else:shutil.copyfile(initial,state)
    declared=cases();hook=assignment(PARENT,'HOOK')
    assert 'terrainWorkWrites'in hook and 'workWrites={}'in hook
    hook=hook.replace('workWrites={}','workWrites={},locatorReads={}')
    hook=hook.replace("audio=audio..',\"terrainWorkWrites\":['", "audio=audio..',\"locatorReads\":['..table.concat(p.locatorReads,',')..']'\n   audio=audio..',\"terrainWorkWrites\":['")
    old="string.format('{\"slot\":%d,\"kind\":%d,\"routine\":%d}',slot,read(0x528+slot),read(0x4b8+slot))"
    new="string.format('{\"slot\":%d,\"kind\":%d,\"routine\":%d,\"locator\":%d,\"workWriteCount\":%d}',slot,read(0x528+slot),read(0x4b8+slot),read(0x12)+256*read(0x13),#active.workWrites)"
    assert old in hook;hook=hook.replace(old,new)
    hook+='''
memory.registerexec(0xb686,1,function()
 if active then active.locatorReads[#active.locatorReads+1]=string.format('{"pc":%d,"bank":%d,"slot":%d,"value":%d,"workWriteCount":%d}',memory.getregister('pc'),read(0x8000),read(0x83),read(0x12)+256*read(0x13),#active.workWrites)end
end)
'''
    hook=hook.replace('PLAYERS','{'+','.join(hex(a)for _,a in PLAYER)+'}').replace('ENEMIES','{'+','.join(hex(a)for _,a in ENEMY)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in WORLD)+'}').replace('CAPTURE_BIAS','true')
    hook+='\nlocal cases={'+','.join('{'+','.join(f'[{a}]={v}'for a,v in c['writes'].items())+'}'for c in declared)+'}\n'
    hook+='''function configure_phase(index)
 local c=assert(cases[index]);local saved={}
 for a=0,2047 do if a<0x100 or a>=0x200 then saved[a]=memory.readbyte(a)end end
 for a,v in pairs(c)do memory.writebyte(a,v)end
 return saved
end
'''
    spec={'name':args.name,'load_state':'stage-8','frames':len(declared),'captures':[],'hooks_lua':hook,'timeout_seconds':150}
    path=oracle/(args.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in(out/'enemy-ticks.jsonl').read_text().splitlines()];assert len(rows)==len(declared)
    fixture={'source_is_original_execution':True,'scope':'aliens-locator','interventions':__doc__,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':declared,
             'parentRecorderSHA256':hashlib.sha256(PARENT.read_bytes()).hexdigest(),'player_fields':[{'name':n,'address':hex(a)}for n,a in PLAYER],'enemy_fields':[{'name':n,'address':hex(a)}for n,a in ENEMY],
             'world_fields':[{'name':n,'address':hex(a)}for n,a in WORLD],'records':rows}
    target=ROOT/'games/contra/engine/testdata/original-enemy-collision-tick-aliens-locator.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==rows,'native locator replay differs'
    else:target.write_bytes(gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0))
    result={'phases':len(rows),'entityCalls':sum(len(r['slots'])for r in rows),'workWrites':sum(len(r['terrainWorkWrites'])for r in rows),'fixtureSHA256':hashlib.sha256(target.read_bytes()).hexdigest(),'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    (out/'verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))

if __name__=='__main__':main()
