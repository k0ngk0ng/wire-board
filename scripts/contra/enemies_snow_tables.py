"""Generate snow rule tables only from verified Japanese ROM data ranges."""
import hashlib,json,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
TABLES={'snowTankWait':(0xa50b,8),'snowTankWheels':(0xa604,4),'snowTankTurrets':(0xa5c5,3),'snowTankShots':(0xa5c8,9),'snowTankDestruction':(0xa687,30),'snowTankPalettes':(0xa6ba,5),
        'snowCarrierX':(0xa707,4),'snowCarrierParts':(0xa73c,4),'snowCarrierOffsets':(0xa740,8),'snowCarrierTop':(0xa77e,8),'snowCarrierChildren':(0xa80e,16),'snowCarrierThrusters':(0xa81e,4),'snowCarrierExplosions':(0xa8d0,10),'snowCarrierDoor':(0xa908,9)}
def main():
    with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:r=z.read('Contra (J).nes')
    sha=hashlib.sha256(r).hexdigest();assert sha=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    p=r[16:131088];assert p[0x26ad:0x26b0].hex()=='b9baa6'
    text='// Code generated from pinned Japanese ROM by scripts/contra/enemies_snow_tables.py.\npackage contra\n\n';records={}
    for name,(address,count)in TABLES.items():
        data=p[address-0x8000:address-0x8000+count];assert len(data)==count
        text+=f'var {name} = [...]uint8'+'{'+', '.join(f'0x{x:02x}'for x in data)+'}\n'
        records[name]={'cpu':hex(address),'prg_offset':address-0x8000,'rom_offset':16+address-0x8000,'bytes':list(data),'sha256':hashlib.sha256(data).hexdigest()}
    (ROOT/'games/contra/engine/enemies_snow_tables.go').write_text(text)
    (ROOT/'docs/research/nes-contra/evidence/enemies-snow-tables.json').write_text(json.dumps({'rom_sha256':sha,'tables':records},indent=2)+'\n')
    print(json.dumps({'tables':len(records)}))
if __name__=='__main__':main()
