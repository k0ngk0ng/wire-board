"""Retain exact natural exceptions and complete source-bound REA records."""
from pathlib import Path
import gzip,json,hashlib,zipfile
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'docs/research/nes-contra/evidence'
def main():
 p=OUT/'enemies-base-4-room.json.gz';d=json.loads(gzip.decompress(p.read_bytes()));ids=[140,8586,8715,9174];rows=[r for r in d['records']if r['id']in ids];assert len(rows)==4
 table=[]
 for r in rows:
  created=[(i,b)for i,(a,b)in enumerate(zip(r['before']['pool'],r['after']['pool']))if a[4]==0 and b[4]==1 and b[11]==17];assert len(created)==1
  slot,enemy=created[0];table.append(dict(id=r['id'],frame=r['frame'],parent_slot=r['slot'],new_slot=slot,original_attributes=enemy[19],normalized_attributes=0))
 exact=dict(exception='BASE-ROLLER-STALE-ATTRIBUTE',source_is_original_execution=True,source_fixture=dict(path=str(p.relative_to(ROOT)),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),source=d['source'],scenario=d['scenario'],world_fields=d['world_fields'],enemy_fields=d['enemy_fields'],summary=table,records=rows)
 (OUT/'enemies-base-roller-originals.json.gz').write_bytes(gzip.compress(json.dumps(exact,separators=(',',':')).encode(),mtime=0))
 paths=[]
 for name in['roller-rea','roller-helper-rea','roller-lane-rea']:
  paths+=sorted((ROOT/'.local/contra-enemies-base'/name).glob('function-*.json'))
 for group,addresses in[
  ('.local/contra-enemies-base/extra-rea',['8f9b','8fa3','e67c','e6d5','e737']),
  ('.local/contra-enemies-base/soldier-helper-rea',['971b']),
  ('.local/contra-enemies-collision/all-rea',['e10a','e276','e309','e342','e949','e8a6','e8d8']),
  ('.local/contra-enemies/enemy-rea',['ed70','edad','ed34','ed3f','ed62']),
  ('.local/contra-sprites/sprite-rea',['9167','919f','9284'])]:
  paths += [ROOT/group/f'function-0x{a}.json'for a in addresses]
 records=[dict(path=str(p.relative_to(ROOT)),record=json.loads(p.read_text()))for p in paths]
 with zipfile.ZipFile('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')as z:rom=z.read('Contra (J).nes')
 assert hashlib.sha256(rom).hexdigest()=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
 prg=rom[16:131088];word=lambda a:int.from_bytes(prg[a:a+2],'little')
 assert word(0x1e5cb)==word(0x1e5cf)==0xe5ff
 assert word(0x1e601)==0x8f8f
 routines=[word(0x8f91-0x8000+2*i)for i in range(5)];assert routines==[0x8f9b,0x8fa3,0xe67c,0xe6d5,0xe737]
 static=dict(exception='BASE-ROLLER-STALE-ATTRIBUTE',format='complete-rea-records',rom_sha256=hashlib.sha256(rom).hexdigest(),base_levels=[1,3],shared_enemy_table='0xe5ff',roller_table_minus_two='0x8f8f',roller_routines=[hex(a)for a in routines],property_bytes=list(prg[0x1ee11+17*4:0x1ee15+17*4]),records=records)
 (OUT/'enemies-base-roller-rea.json.gz').write_bytes(gzip.compress(json.dumps(static,separators=(',',':')).encode(),mtime=0))
 index=dict(rom_sha256=static['rom_sha256'],exception=static['exception'],records=[dict(path=r['path'],evidence_id=r['record']['evidence_id'],procedure=r['record']['parameters']['procedure'],source_sha256=r['record']['subject']['digest']['sha256'])for r in records])
 (OUT/'enemies-base-roller-rea-index.json').write_text(json.dumps(index,indent=2)+'\n')
 print(json.dumps(dict(original_records=len(rows),rea_records=len(records),summary=table)))
if __name__=='__main__':main()
