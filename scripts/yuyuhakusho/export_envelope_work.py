"""Bind finite envelope navigation workloads to existing logical tick cursors."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path

from reference import ROOT
from score_parameter_tables import compile_pitch_envelope, compile_psg_volume_envelope


def sha(raw):return hashlib.sha256(raw).hexdigest()


def walk(data,pitch):
    position=0;boundary={};records=[]
    while position not in boundary:
        boundary[position]=len(records);start=position;visited=set();navigation=[]
        while True:
            assert position not in visited,'same-tick source cycle';visited.add(position)
            at=position;value=data[at];position=(position+1)&255
            if value==128:navigation.append('restart');position=0;continue
            if value==130:navigation.append('retarget');position=data[at+1];continue
            if pitch and value==129:navigation.append('repeat');position=(position-2)&255;continue
            if pitch and value==132:navigation.append('scale');position=(position+1)&255;continue
            if not pitch and value in[129,131]:position=(position-2)&255
            records.append({'work':{'navigation':navigation},'source_cursor':start,'terminal':bool(pitch and value==131)})
            if pitch and value==131:return records,[]
            break
    first=boundary[position]
    return records[:first],records[first:]


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT/'.local')and not out.exists();out.mkdir(parents=True)
    raw_path=ROOT/'games/yuyuhakusho/data/audio-score.json.gz';raw=gzip.decompress(raw_path.read_bytes());catalog=json.loads(raw);rom_path=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rom_path.read_bytes()
    assert sha(rom)=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
    rows=[];provenance=[]
    for kind,table,count,end,compiler,key in[('pitch',0x3a256,8,0x3a31a,compile_pitch_envelope,'pitch_envelopes'),('volume',0x3a31a,11,0x3a492,compile_psg_volume_envelope,'psg_volume_envelopes')]:
        formal={r['id']:r for r in catalog[key]}
        for number in range(1,count+1):
            address=int.from_bytes(rom[table+(number-1)*4:table+number*4],'big');assert table+count*4<=address<end
            data=rom[address:end];typed=compiler(data,number).to_data();assert typed==formal[number],('typed envelope changed',kind,number)
            prefix,cycle=walk(data,kind=='pitch');assert len(prefix)==len(typed['prefix'])and len(cycle)==len(typed['cycle'])
            for section,items in[('prefix',prefix),('cycle',cycle)]:
                for index,item in enumerate(items):
                    assert len(item['work']['navigation'])<=32
                    logical={'family':kind,'envelope_id':number,'section':section,'tick_index':index}
                    # The formal closure needs only zero/one restart. Keep a
                    # scalar workload instead of retaining a navigation tape.
                    assert item['work']['navigation']in[[],['restart']]
                    work={'restarts':len(item['work']['navigation'])}
                    rows.append({'key':logical,'work':work})
                    provenance.append({'key':logical,'source_address':address,'source_cursor':item['source_cursor'],'work':work,'tick':typed[section][index]})
    assert len(rows)==580
    resource={'schema':1,'format':'typed-envelope-work-v1','catalog_json_sha256':sha(raw),'records':rows}
    encoded=(json.dumps(resource,sort_keys=True,separators=(',',':'))+'\n').encode();target=out/'audio-score-envelope-work.json.gz';target.write_bytes(gzip.compress(encoded,mtime=0))
    evidence={'schema':1,'scope':'Original envelope navigation work per existing logical tick; source fields only in research provenance','catalog_json_sha256':sha(raw),'records':provenance,'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in[Path(__file__),ROOT/'scripts/yuyuhakusho/score_parameter_tables.py',raw_path,rom_path]}}
    (out/'provenance.json.gz').write_bytes(gzip.compress((json.dumps(evidence,sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0))
    report={'pass':True,'records':len(rows),'pitch':sum(r['key']['family']=='pitch'for r in rows),'volume':sum(r['key']['family']=='volume'for r in rows),'resource_bytes':target.stat().st_size,'resource_sha256':sha(target.read_bytes()),'resource_json_sha256':sha(encoded),'source_fields_in_production_resource':False,'original_sound_ticks_unchanged':True}
    (out/'export.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))


if __name__=='__main__':main()
