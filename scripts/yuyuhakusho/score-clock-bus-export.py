"""Project original bus+YM service into source-free integration outcomes."""
import argparse,gzip,json
from collections import Counter
from pathlib import Path
from reference import ROOT,sha
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
 raw=a.source.read_bytes();body=json.loads(gzip.decompress(raw));cases=[];payload=None
 for row in body['cases']:
  source=row['resource']
  if payload is None:payload={'ID':source['id'],'EncodedHex':source['encoded_hex'],'Descriptor':{'ID':'ordinary073','PayloadID':source['id'],'NativeRate':source['rate'],'Flags':source['flags'],'RoutingOrPan':source['ordinary_routing'],'Protocol':1}}
  else:assert payload['ID']==source['id']and payload['EncodedHex']==source['encoded_hex']
  main=[];request=None;main_data=Counter()
  for e in row['port_bus_events']:
   kind=e['kind']
   if kind in[1,2]and e['owner']!=1:continue
   if kind==4:assert request is None;request=e;continue
   if kind==5:
    assert request is not None;value=request['value']>>8 if request['width']==2 else request['value'];main.append({'Kind':'request','At':request['at'],'After':e['after'],'Asserted':bool(value&1)});request=None
   elif kind==6:main.append({'Kind':'ack','At':e['at'],'Granted':not bool(e['value']&1)})
   elif kind==3:main.append({'Kind':'mailbox','At':e['after'],'Value':e['value']})
   elif kind in[1,2]:
    main.append({'Kind':'status'if kind==1 else'write','At':e['at'],'Port':e['address'],'Value':e['value']})
    if kind==1:assert e['value']&0x7f==0,'Selected private profile must not hide timer status flags'
    if kind==2 and e['address']&1:main_data[(e['at'],e['address'],e['value'])]+=1
   else:raise AssertionError(('Unexpected main event',e))
  assert request is None
  aux=[]
  for e in row['PCM_events']:
   kind=e['kind']
   if kind not in[1,2,3,9]:continue
   if kind in[1,2,9]:
    port=((e['address']>>8)*2)+1;key=(e['cycles_before'],port,e['data'])
    if main_data[key]:main_data[key]-=1;continue
   n={'Cycle':e['cycles_before'],'Kind':'dac'if kind==1 else'pan'if e['address']==0x1b6 else'enable','Value':e['data']if kind in[1,2]else 0,'PayloadID':'','Offset':0,'Register':0,'Bus':None}
   if kind==9:n.update(Kind='ym-write',Register=e['address'],Value=e['data'])
   if kind==3:
    address=e['bank']+(e['address']&32767)
    if e['pc']in[0xa5,0xa8,0xc3,0xc6]:k='metadata';offset=address-source['source']['header_address']
    elif e['pc']==0x149:k='dummy';offset=(address-source['source']['header_address'])&65535
    else:k='bank';offset=address-source['source']['sample_address']
    n.update(Kind=k,PayloadID=source['id'],Offset=offset,Value=0,Bus={'At':e['z80_after']-45,'Z80After':e['z80_after'],'M68KDelay':e['m68k_after']-e['m68k_before'],'SetWaitFlag':False})
   aux.append(n)
  assert not any(main_data.values()),('Main data exclusion failed',main_data)
  cases.append({'Input':row['input'],'Before':{'Frontier':row['before_service']['aux_frontier'],'M68KDelay':row['before_service']['startup_main_steal_before_declared_origin'],'Busy':row['before_service']['busy']},'Main':main,'Aux':aux,'End':{'Clock':{'Cycle':row['end']['cycle'],'Refresh':row['end']['refresh']},'Frontier':row['end']['aux_frontier'],'Status':row['end']['status']}})
 payload_body={'Version':1,'Scope':'Given conditional service origin after real empty-to-ordinary073 startup; main IO/real mailbox/aux semantic data only, not aux raw-address pin equivalence or full PCM state','Resource':payload,'Cases':cases};content=(json.dumps(payload_body,separators=(',',':'))+'\n').encode();target=out/'bus-native-services.json.gz';target.write_bytes(gzip.compress(content,mtime=0))
 manifest={'pass':True,'cases':len(cases),'main_accesses':sum(len(c['Main'])for c in cases),'aux_semantic_events':sum(len(c['Aux'])for c in cases),'source':{'path':str(a.source.resolve().relative_to(ROOT)),'sha256':sha(raw)},'fixture':{'path':str(target.relative_to(ROOT)),'gzip_sha256':sha(target.read_bytes()),'json_sha256':sha(content)},'source_PC_ROM_addresses_removed':True,'producer_sha256':sha(Path(__file__).read_bytes())};(out/'bus-native-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest))
if __name__=='__main__':main()
