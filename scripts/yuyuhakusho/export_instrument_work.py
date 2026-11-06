"""Bind logical instrument choices to immutable sound/work identities, static only."""
import argparse
import gzip
import hashlib
import json
from pathlib import Path

from score_parameter_tables import read_fm_patch

ROOT=Path(__file__).resolve().parents[2]


def sha(raw):return hashlib.sha256(raw).hexdigest()
def canonical(value):return json.dumps(value,sort_keys=True,separators=(',',':')).encode()


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)and not out.exists();out.mkdir(parents=True)
    scorepath=ROOT/'games/yuyuhakusho/data/audio-score.json.gz';raw=gzip.decompress(scorepath.read_bytes());catalog=json.loads(raw)
    proofpath=ROOT/'.local/md-yuyuhakusho/audio-score-source/typed-boundary-corrected/provenance.json';proof=json.loads(proofpath.read_text());rompath=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rompath.read_bytes()
    source={(r['bank'],r['index']):r['id']for r in proof['patches']};profiles={};selectors=[];private=[]
    def profile(patch,skip):
        value={'patch_id':patch,'records_skipped':skip};identity='instrument-'+sha(canonical(value))[:24];profiles[identity]={'id':identity,**value};return identity
    def add(sid,vid,section,index,patch,bank,source_index,address=None):
        actual=read_fm_patch(rom,bank,source_index).to_score_data();actual.pop('id',None)
        assert 'patch-'+sha(canonical(actual))[:20]==patch,(sid,vid,section,index)
        if (bank,source_index)in source:assert source[(bank,source_index)]==patch
        work=profile(patch,source_index);key={'score_id':sid,'voice_id':vid,'section':section,'event_index':index}
        selectors.append({'key':key,'reference':work});private.append({'key':key,'reference':work,'source_bank':bank,'source_index':source_index,'source_address':address})
    for score in catalog['scores']:
        sid=score['id'];sp=proof[str(sid)];bank=sp['instrument_bank'];pv={v['id']:v for v in sp['voices']}
        for voice in score['voices']:
            vid=voice['id'];patch=voice['initial'].get('patch_reference')
            if patch:add(sid,vid,'initial',0,patch,bank,0)
            trace=pv[vid]['events'];flat=0
            assert len(trace)==len(voice['timeline']['prefix'])+len(voice['timeline']['cycle'])
            for section in ['prefix','cycle']:
                for index,event in enumerate(voice['timeline'][section]):
                    row=trace[flat];flat+=1
                    if not event.get('patch_id'):continue
                    assert event['kind']in['fm-patch','fm-patch-select']and row['token']in[0xef,0xf5]
                    address=row['address'];assert rom[address]==row['token'];source_index=rom[address+1]
                    add(sid,vid,section,index,event['patch_id'],bank,source_index,address)
    fallback=catalog['fallback_patch_id'];data=read_fm_patch(rom,0,0).to_score_data();data.pop('id',None);assert 'patch-'+sha(canonical(data))[:20]==fallback
    fallback_ref=profile(fallback,0)
    assert len(selectors)==960 and sum(x['key']['section']=='initial'for x in selectors)==68
    document={'schema':1,'format':'typed-instrument-work-v1','catalog_json_sha256':sha(raw),'fallback_reference':fallback_ref,'profiles':[profiles[k]for k in sorted(profiles)],'selectors':selectors}
    encoded=canonical(document)+b'\n';target=out/'audio-instrument-work.json.gz';target.write_bytes(gzip.compress(encoded,mtime=0))
    provenance={'pass':True,'scope':'Static logical instrument identity/workload binding; source bank/index/address confined to this research file','raw_json_sha256':sha(encoded),'gzip_sha256':sha(target.read_bytes()),'selectors':private,'profiles':len(profiles),'initials':68,'events':892,'sources':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in [scorepath,proofpath,rompath,Path(__file__)]}}
    (out/'provenance.json').write_text(json.dumps(provenance,indent=2)+'\n');print(json.dumps({'pass':True,'profiles':len(profiles),'selectors':len(selectors),'gzip_bytes':target.stat().st_size,'gzip_sha256':sha(target.read_bytes())}))


if __name__=='__main__':main()
