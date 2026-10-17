"""Whole original action scheduler, controlled special-shot initial states."""
import gzip,hashlib,itertools,json
from verify_ball17 import ROOT, ROM, PRG, BASE, SoundMemory, initial
from verify_pass_floor_original import MPU
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context

def main():
    rows=[]
    for action,left,entered in itertools.product([16,17,19,22],[False,True],[False,True]):
        ram=initial(left,entered);ram[0x518]=action|(128 if entered else 0)
        # Component probes replace the shooter action for launch-context checks.
        # A whole phase retains the recorded player action/cursor pair instead.
        ram[0x512]=BASE[0x512]
        # Only action17 has an animation next-action/countdown at this origin.
        if action!=17:ram[0x532]=255;ram[0x457]=0
        m=SoundMemory(PRG,ram);cpu=MPU(memory=m,pc=0xd345);sounds=[];calls=[]
        for _ in range(100000):
            if cpu.pc==0xe18b:calls.append(cpu.x)
            if cpu.pc==0xfbcc:sounds.append(cpu.a)
            if cpu.pc==0xd348:break
            cpu.step()
        else:raise RuntimeError((action,left,entered,hex(cpu.pc)))
        rows.append(dict(label=f'{action}-{left}-{entered}',before=decode_world_action_state(ram),after=decode_world_action_state(bytes(m[:2048])),context=context(ram)['actions'],calls=calls,sounds=sounds))
    output=ROOT/'games/basketball/engine/testdata'
    packed=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0)
    (output/'recovered-special-world.json.gz').write_bytes(packed)
    meta=dict(scope='controlled original D345..D348 whole action phase; not natural menu gameplay',actions=[16,17,19,22],calls=len(rows),romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest())
    (output/'recovered-special-world-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
