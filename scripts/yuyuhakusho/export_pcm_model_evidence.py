"""Keep original tables/native value stages; no clock replay in production."""
from pathlib import Path
import argparse,gzip,json
from reference import ROOT,sha

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--capture',type=Path,required=True);parser.add_argument('--out',type=Path,required=True);args=parser.parse_args()
 capture=args.capture.resolve();out=args.out.resolve();assert capture.is_relative_to(ROOT)and out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 report=json.loads((capture/'capture.json').read_text());assert report['pass']and report['all_original_observed_frames_equal']and report['observer_overflows']==0
 tables=report['tables'];sources=json.loads((capture/'samples.json').read_text());samples={x['code']:x for x in sources['samples']}
 encoded_pcs={0x012b:'single',0x01a4:'single',0x02a2:'a',0x04dd:'a',0x02bb:'b',0x0502:'b'}
 first_mix={0x02dd,0x0518};second_mix={0x0349,0x0584,0x0376,0x05b1};first_single={0x0137,0x01b0};second_single={0x0176,0x01d8}
 cases=[]
 for case in report['cases']:
  path=capture/(case['name']+'-events.json.gz');body=json.loads(gzip.decompress(path.read_bytes()));native={x['encoded_event']:x for x in body['decoder_pairs']};stages=[]
  role_codes={'single':0x73}if case['name']=='single-mode'else{'a':case['commands'][0][1]}|({'b':case['commands'][1][1]}if len(case['commands'])>1 else{})
  for index,event in enumerate(body['events']):
   kind,pc=event['kind'],event['pc']
   base={'source_event':index,'video_step':event['frame'],'reference_master_cycle':event['cycles_before']}
   if kind==4 and pc in encoded_pcs:
    n=native[index];assert n['role']==encoded_pcs[pc]
    stages.append(base|{'kind':'decode','role':n['role'],'encoded':n['encoded'],'native_accumulator_before':n['old_accumulator'],'native_output':[n['native_high'],n['native_low']]})
   elif kind==6:
    assert pc in first_mix|second_mix;stages.append(base|{'kind':'mix-first'if pc in first_mix else'mix-second','native_output':event['data']})
   elif kind==1:
    tag='dac'if case['name']!='single-mode'else('single-first'if pc in first_single else'single-second')
    if case['name']=='single-mode':assert pc in first_single|second_single
    stages.append(base|{'kind':tag,'native_output':event['data']})
  cases.append({'name':case['name'],'roles':{role:{'code':code,'encoded_hex':samples[code]['encoded_hex'],'encoded_sha256':sha(bytes.fromhex(samples[code]['encoded_hex']))}for role,code in role_codes.items()},'stages':stages,'source_events_sha256':sha(path.read_bytes()),'native_dac_events':case['dac_events'],'native_decoder_pairs':case['decoder_pairs']})
 payload={'schema':1,'scope':'Original tables and ordered native semantic value stages; reference cycles are test provenance only, not a production timing algorithm','tables':tables,'cases':cases}
 raw=(json.dumps(payload,separators=(',',':'))+'\n').encode();path=out/'original-values.json.gz';path.write_bytes(gzip.compress(raw,mtime=0))
 manifest={'pass':True,'schema':1,'fixture_path':str(path.relative_to(ROOT)),'fixture_json_sha256':sha(raw),'fixture_gzip_sha256':sha(path.read_bytes()),'capture_path':str((capture/'capture.json').relative_to(ROOT)),'capture_sha256':sha((capture/'capture.json').read_bytes()),'producer_sha256':sha(Path(__file__).read_bytes()),'cases':len(cases),'decoder_pairs':sum(c['native_decoder_pairs']for c in cases),'dac_events':sum(c['native_dac_events']for c in cases),'scope':payload['scope']}
 (out/'original-values-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest))
if __name__=='__main__':main()
