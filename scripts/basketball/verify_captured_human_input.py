"""Original 805C input in captured contexts; no invented escape strategy.

These are controlled function calls, not natural matches. The independent World
integration test starts with a real Kind10 contact and uses production input.
"""
import gzip, hashlib, itertools, json
from verify_held_auxiliary_input import ROOT, ROM, SOURCE, BASE, machine, input_call, call

def captured_machine(actor,action,held,phase):
    cpu=machine(actor,0,action,action in (0x54,0x55),False,held=False)
    m=cpu.memory
    m[0x56b]=actor;m[0x56c]=4;m[0x53f]=actor if held else 6
    m[0x516]=0x86;m[0x3da]=10;m[0x577+(actor-1)]=phase
    # Input grouping depends on SpecialHolder, not the actor being an owner of
    # aux4. Keep the actual captured object unowned and non-attached.
    m[0x53d]=255;m[0x646]=64
    return cpu

SCENARIOS=[('idle',[0]*3),('right',[1,1,0]),('left',[2,2,0]),('up',[8,8,0]),('down',[4,4,0]),
 ('double-right',[1,0,1,1,0]),('double-up',[8,0,8,8,0]),
 ('A',[128,128,0]),('B',[64,64,0]),('jump',[192,192,0]),
 ('combine',[128,192,0]),('short-jump',[192,0,0]),('repeat-jump',[192,0,192,0]),('opposite',[3,3,0])]
rows=[]
for actor,action,held,phase,(name,sequence) in itertools.product(range(4),range(0x50,0x57),(False,True),(0,128),SCENARIOS):
    cpu=captured_machine(actor,action,held,phase);last=0
    label=f'{name}-actor{actor}-action{action:02x}-held{held}-phase{phase}'
    for tick,buttons in enumerate(sequence):
        rows.append(input_call(cpu,actor,buttons,last,label,tick));last=buttons
p=ROOT/'games/basketball/engine/testdata/captured-human-input.json.gz'
p.write_bytes(gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0))
p.with_name('captured-human-input-source.json').write_text(json.dumps(dict(records=len(rows),original_function='bank6:805C',original_instructions_unchanged=True,natural_gameplay=False,scope='50..56, all four actors, ball ownership priority, jump phase gate, 14 controller sequences; state remains continuous within each input sequence, no physics',sound_scope='C000/FBCC preserve07FF sound latch and return; no audio driver',ram_source=str(SOURCE.relative_to(ROOT)),ram_frame=1501,ram_sha256=hashlib.sha256(BASE).hexdigest(),rom_sha256=hashlib.sha256(ROM).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n')
print('captured input original calls',len(rows))
