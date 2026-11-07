"""Prepare the identified Japanese sound bank for bounded REA analysis.

Failure boundaries: US sound IDs/addresses reused; wrong 8 KiB bank pair; code
and tables silently shifted; analysis output outside the workspace; original
ROM changed. The static slice omits the bank tag/volume-pointer prefix below
$80D5, and explicitly does not claim complete hardware/mapper emulation.
"""
import hashlib
import json
from pathlib import Path
import zipfile

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-audio'
ROM=Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
ROM_SHA='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'


def main():
    before=hashlib.sha256(ROM.read_bytes()).hexdigest()
    with zipfile.ZipFile(ROM) as archive:raw=archive.read('Contra (J).nes')
    assert hashlib.sha256(raw).hexdigest()==ROM_SHA
    prg=raw[16:131088]
    audio=prg[2*8192:4*8192];fixed=prg[14*8192:16*8192]
    assert audio[0]==0x12 and audio[-32:]==bytes([255])*32
    BASE.mkdir(parents=True,exist_ok=True)
    (BASE/'audio-bank.bin').write_bytes(audio)
    target=audio[0xd5:]+fixed
    (BASE/'audio-80d5.bin').write_bytes(target)
    entries={'0x80d5':'sound_frame_update','0x8719':'reset_sound_channels',
             '0x87a2':'request_sound','0xf90b':'initialize_apu_hardware',
             '0xf9bc':'request_sound_wrapper','0xf9d4':'reset_sound_wrapper'}
    (BASE/'seeds.tsv').write_text(''.join(f'{pc}\tfunction\t{name}\n' for pc,name in entries.items()))
    spec={'target':str(BASE/'audio-80d5.bin'),'entry':'0x80d5','seeds':str(BASE/'seeds.tsv'),
          'file_sha256':hashlib.sha256(target).hexdigest(),'addresses':list(entries),'output':str(BASE/'rea'),
          'tmp':str(ROOT/'dist/ca-tmp'),'cache':str(BASE/'swift-cache')}
    (BASE/'spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    extra_target=audio[0x6d:]+fixed
    assert extra_target[:2]==bytes([0xa9,0x30])
    (BASE/'audio-806d.bin').write_bytes(extra_target)
    extra={'0x806d':'mute_pulse_channel','0x809f':'sound_pause_state','0xcc7d':'initialize_stage_music'}
    (BASE/'extra-seeds.tsv').write_text(''.join(f'{pc}\tfunction\t{name}\n' for pc,name in extra.items()))
    extra_spec=dict(spec,target=str(BASE/'audio-806d.bin'),entry='0x806d',seeds=str(BASE/'extra-seeds.tsv'),
                    file_sha256=hashlib.sha256(extra_target).hexdigest(),addresses=list(extra),output=str(BASE/'extra-rea'))
    (BASE/'extra-spec.json').write_text(json.dumps(extra_spec,indent=2)+'\n')
    control={'0x8103':'update_sound_slot','0x865f':'clear_triangle_and_dpcm','0x88da':'start_dpcm_sample'}
    (BASE/'control-seeds.tsv').write_text(''.join(f'{pc}\tfunction\t{name}\n' for pc,name in control.items()))
    control_spec=dict(extra_spec,seeds=str(BASE/'control-seeds.tsv'),addresses=list(control),output=str(BASE/'control-rea'))
    (BASE/'control-spec.json').write_text(json.dumps(control_spec,indent=2)+'\n')
    source={'romSHA256':ROM_SHA,'archiveSHA256':before,'audioBank8K':[2,3],'fixedBank8K':[14,15],
            'audioBankSHA256':hashlib.sha256(audio).hexdigest(),'fixedBankSHA256':hashlib.sha256(fixed).hexdigest(),
            'slice':{'start':'0x80d5','end':'0xffff','sha256':spec['file_sha256']},
            'limitations':['The static volume-pointer prefix $8000..$80D4 is not mapped in this slice. Full source bytes remain available to the offline recorder.']}
    assert hashlib.sha256(ROM.read_bytes()).hexdigest()==before
    (BASE/'source.json').write_text(json.dumps(source,indent=2)+'\n')
    print(BASE/'spec.json')


if __name__=='__main__':main()
