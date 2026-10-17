"""Read original selection labels using controller-only FCEUX menu routes.
No RAM writes or gameplay availability changes. Artifacts remain workspace-local.
"""
from pathlib import Path
import json, hashlib
from capture_reference import ROOT, BASE, run
OUT=ROOT/'.local/basketball/selection-names'

def capture(name,segments,frames,captures):
    spec=dict(name='selection-names-'+name,load_slot=1,frames=frames,segments=segments,captures=captures)
    p=OUT/(name+'.json');p.write_text(json.dumps(spec,indent=2)+'\n')
    target=BASE/'runs'/spec['name']
    if not (target/'capture-complete.json').exists() or json.loads((target/'scenario.json').read_text())!=spec:run(p)
    return target

def key(n,k):return dict(start=n,end=n+1,keys=[k])

def state(path,n):
    b=(path/'ram.bin').read_bytes()[(n-1)*2048:n*2048]
    return dict(teams=list(b[0x586:0x588]),court=b[0x58b],characters=list(b[0x3e3:0x3e7]))

def main():
    OUT.mkdir(parents=True,exist_ok=True)
    prefix=[key(30,'down'),key(60,'A'),key(180,'A')]
    courts=[];teams=[]
    for ordinal in range(8):
        segments=prefix+[key(400,'A'),key(500,'A')]+[key(700+60*j,'right') for j in range(ordinal)]
        choose=760+60*ordinal;segments += [key(choose,'A')]
        end=choose+300;path=capture('court-'+str(ordinal),segments,end,[choose-10,end])
        courts.append(dict(ordinal=ordinal,path=str(path.relative_to(ROOT)),selectedFrame=end,menuFrame=choose-10,state=state(path,end)))
        segments=prefix+[key(300+60*j,'right') for j in range(ordinal)]
        choose=350+60*ordinal;segments += [key(choose,'A'),key(choose+120,'A'),key(choose+300,'A')]
        end=choose+600;path=capture('team-'+str(ordinal),segments,end,[choose-10,choose+60,end])
        teams.append(dict(ordinal=ordinal,path=str(path.relative_to(ROOT)),selectedFrame=end,state=state(path,end)))
    doc=dict(scope='controller-only versus menu from original saved mode menu; no RAM writes',initialState=str((BASE/'states/slot-1.fcs').relative_to(ROOT)),initialStateSHA256=hashlib.sha256((BASE/'states/slot-1.fcs').read_bytes()).hexdigest(),courts=courts,teams=teams)
    (OUT/'natural-menu.json').write_text(json.dumps(doc,indent=2)+'\n')
if __name__=='__main__':main()
