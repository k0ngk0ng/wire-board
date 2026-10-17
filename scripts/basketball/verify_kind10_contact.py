"""Original bank5 8653/876F Kind10 capture and fallback instruction matrix."""
import gzip,hashlib,itertools,json
from verify_prop_attack_original import ROOT,code,build,state,run,word


def main():
    records=[]
    for index,(source,target,action,height,attached) in enumerate(itertools.product([4,5],range(4),[0,3,6,10,13,16],[0,255,256],range(-1,7))):
        r=build(index,0,source,1,0,False,0)
        r[0x3c:0x42]=bytes([128]*6);r[0x3c+source]=target
        r[0x3d6+source]=10;r[0x512+source]=128|action;r[0x46b+source]=128 if index&1 else 0
        r[0x58b]=4;r[0x56b]=3;r[0x56c]=155
        r[0x642:0x648]=bytes([0]*6)
        if attached>=0:r[0x642+min(attached,5)]=(128 if attached<6 else 192)|target
        word(r,0xa9+target,0xb6+target,height);r[0xc3+target]=0
        after,sounds=run(r,source)
        kinds=[12,11];kinds[source-4]=10
        records.append(dict(source=source,target=target,court=4,auxiliaryKinds=kinds,attachments=list(r[0x642:0x646]),before=state(r),after=state(after),sounds=sounds,changeCaptureObject=after[0x56c]!=r[0x56c],captureObject=after[0x56c]))
    for source,action in itertools.product([4,5],[0,3,10,13,16]):
        target=source^1;r=build(0,0,target,1,0,False,0)
        r[0x3c:0x42]=bytes([128]*6);r[0x3c+source]=target;r[0x3d6+source]=10;r[0x512+source]=128|action;r[0x56b]=3;r[0x56c]=155
        after,sounds=run(r,source);kinds=[12,11];kinds[source-4]=10
        records.append(dict(source=source,target=target,court=4,auxiliaryKinds=kinds,attachments=list(r[0x642:0x646]),before=state(r),after=state(after),sounds=sounds,changeCaptureObject=after[0x56c]!=r[0x56c],captureObject=after[0x56c]))
    out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress(json.dumps(records,separators=(',',':')).encode(),mtime=0)
    (out/'kind10-contact.json.gz').write_bytes(packed)
    manifest=dict(records=len(records),captures=sum(r['changeCaptureObject'] for r in records),source_is_original_execution=True,natural_gameplay=False,entry='bank5 8653 -> 876F',instructions_unchanged=True,interception='C000 sound request sink only',bank5_sha256=hashlib.sha256(code).hexdigest(),sha256=hashlib.sha256(packed).hexdigest(),rea_evidence_id='ev_85a7bb29ede12dea40b0e12334c9754763ff0df7584b2b182200d7f250638b62')
    (out/'kind10-contact-source.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest))


if __name__=='__main__':main()
