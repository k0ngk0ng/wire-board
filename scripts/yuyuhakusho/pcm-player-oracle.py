"""Original-only event classification; no playback arithmetic or model.

Raw PC labels exist only here to identify the research observer event source.
"""
def original(body):
 source=[]
 for e in body['events']:
  cycle=e['frame']*896040+e['cycles_before']
  if e['kind']==1:source.append({'cycle':cycle,'kind':'dac','value':e['data']})
  elif e['kind']==2:source.append({'cycle':cycle,'kind':'pan'if e['address']==0x1b6 else'enable','value':e['data']})
  elif e['kind']==3:
   address=e['bank']+(e['address']&0x7fff)
   if body['case']['mode']=='single' and e['pc']==0x149:
    source.append({'cycle':cycle,'kind':'dummy','slot':0,'offset':(e['address']-body['samples'][0]['source']['z80_header'])&65535});continue
   choices=[(i,address-s['source']['sample_address'])for i,s in enumerate(body['samples'])if s and s['source']['sample_address']<=address<s['source']['sample_address']+s['length']+4]
   assert len(choices)==1,(address,choices);slot,offset=choices[0];source.append({'cycle':cycle,'kind':'bank','slot':slot,'offset':offset})
 return source

