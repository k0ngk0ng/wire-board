#!/usr/bin/env python3
"""Verify retained real-platform evidence; print derived, bounded measurements."""
import argparse, gzip, hashlib, json, re, statistics
from pathlib import Path

parser=argparse.ArgumentParser();parser.add_argument('result');args=parser.parse_args()
path=Path(args.result);raw=path.read_bytes();data=json.loads(gzip.decompress(raw)if path.suffix=='.gz'else raw)
assert data['passed'];assert data['build']['rulesBuild']['id']==data['fixture']['rulesBuildID']
if data['fixture'].get('runtimeBuildID'):assert data['build']['runtimeBuildID']==data['fixture']['runtimeBuildID']
assert len(data['build']['wasmPayloadChecks'])==8 and all(not x['fullPayloadPresent']and not x['uniqueFeaturePresent']for x in data['build']['wasmPayloadChecks'])
def luminance(text):
 values=[int(x)/255 for x in re.findall(r'\d+',text)[:3]]
 return sum(weight*(x/12.92 if x<=.04045 else((x+.055)/1.055)**2.4)for x,weight in zip(values,[.2126,.7152,.0722]))
cases=[]
for case in data['cases']:
 rows=case['visible'];steps=[int(b['revision'])-int(a['revision'])for a,b in zip(rows,rows[1:])];intervals=[b['at']-a['at']for a,b in zip(rows,rows[1:])]
 assert len(steps)>=30 and sum(x==1 for x in steps)/len(steps)>=.9
 assert 13<=statistics.median(intervals)<=21
 contrast=[]
 for hud in case['geometry']['hud']:
  low,high=sorted([luminance(hud['color']),luminance(hud['background'])]);ratio=(high+.05)/(low+.05);assert ratio>=4.5;contrast.append(ratio)
 digest_matches=[]
 for name in ['nativeMove','nativeJump']:
  visible=case[name]['after'];record=next(frame for frame in case['after']['view']['frames']if frame['revision']==visible['revision']);assert record['digest']==visible['digest'];digest_matches.append(visible['revision'])
 sent={};latencies=[];native=case['nativeMove']
 for event in native['network']:
  packet=json.loads(event['params']['response']['payloadData']);nonce=packet.get('nonce')
  if packet.get('type')=='input':sent[nonce]=event['params']['timestamp']
  if packet.get('type')=='ack'and nonce in sent:latencies.append((event['params']['timestamp']-sent[nonce])*1000)
 cases.append({'name':case['name'],'visibleSamples':len(rows),'singleRevisionFraction':sum(x==1 for x in steps)/len(steps),'medianVisibleIntervalMS':statistics.median(intervals),'contrastRatios':contrast,'matchingNativeDigestsAtRevisions':digest_matches,'receivedWebSocketBytesPerSecond':native['receivedBytes']/native['observedMS']*1000,'medianAcknowledgementMS':statistics.median(latencies),'maximumAcknowledgementMS':max(latencies),'canvas':case['geometry']['canvas'],'controller':case['geometry']['controller']})
assert data['hiddenCheckpoint']['hidden']['hidden']and not data['hiddenCheckpoint']['hidden']['media'];assert not data['hiddenCheckpoint']['visible']['hidden'];assert data['streamDelay']['nativeSeeks']
print(json.dumps({'passed':True,'evidenceSHA256':hashlib.sha256(raw).hexdigest(),'rulesBuildID':data['fixture']['rulesBuildID'],'webManifestSHA256':data['fixture']['webManifestSHA256'],'scope':'Loopback actual platform with a byte-preserving delay proxy; ACK latency is not total input-to-display latency or WAN evidence.','cases':cases,'streamDelayNativeSeekEvents':len(data['streamDelay']['nativeSeeks']),'hiddenCheckpointSilent':True},ensure_ascii=False,indent=2))
