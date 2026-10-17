"""Original contact feedback followed by unchanged attack-animation calls."""
import gzip, hashlib, itertools, json
from verify_attack_handlers import ROOT, ROM, PRG, initial, machine, call, byte, word
from verify_ai_mode5_plan11 import call as original_call
from capture_player_attack import decode

rows=[];contacts=[];geometry=[]
choices=[(0x62,0,3,94,1),(0x64,3,1,57,2),(0x63,7,0,87,1),(0x66,0,3,98,0),(0x60,6,2,27,2)]
for action,team,character,variant,cursor in choices:
    for actor,facing in itertools.product(range(4),(False,True)):
        r=initial(actor,(action,team,character,variant),True,facing,((8192,True,0) if variant==98 else (0,False,0)),cursor)
        r[0x52c+actor]=0;r[0x4ec+actor]=192 if facing else 64
        r[0x53f]=6;r[0x3c:0x42]=bytes([128]*6);r[0x3c+actor]=actor^2;r[0x625+actor]=3
        cpu=machine(r);before=decode(bytes(cpu.memory[:2048]),actor)
        original_call(cpu,0x8653,actor,bank=5)
        after=decode(bytes(cpu.memory[:2048]),actor)
        assert after['cursor']==cursor+2,(variant,after['cursor'])
        contacts.append(dict(actor=actor,variant=variant,before=before,after=after))
        for tick in range(40):
            row=call(cpu,actor,f'contact-{variant}-{actor}-{facing}-{tick}');rows.append(row)
            if row['after']['action']&127!=action:break
        else:raise AssertionError('tail did not finish')
for variant,start,count in [(57,4,3),(87,3,3),(94,3,3),(98,2,3),(15,5,1),(24,2,1),(27,4,1)]:
    for cursor in range(start,start+count):
        cpu=machine(initial(0,(0x64,3,1,variant),True,False,(0,False,0),0))
        cpu.memory[0x57a]=variant;cpu.memory[0x444]=cursor
        original_call(cpu,0x8318,0,bank=5)
        ptr=cpu.WordAt(0x2c)
        geometry.append(dict(variant=variant,frame=cursor,pointer=ptr,shape=list(cpu.memory[ptr:ptr+8])))
out=ROOT/'games/basketball/engine/testdata'
packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0)
(out/'attack-contact-tails.json.gz').write_bytes(packed)
(out/'attack-contact-tails-source.json').write_text(json.dumps(dict(scope='isolated original contact-to-attack chains; not natural full matches',romSHA256=hashlib.sha256(ROM).hexdigest(),sha256=hashlib.sha256(packed).hexdigest(),records=len(rows),contacts=contacts,geometry=geometry),indent=2)+'\n')
print('original chained attacks',len(rows),'contacts',len(contacts),'geometry',len(geometry))
