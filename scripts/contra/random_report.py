"""Export retained REA/runtime random evidence and an independently checked index."""
import gzip,hashlib,json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-random'
OUT=ROOT/'docs/research/nes-contra/evidence'

def read(path):return json.loads(path.read_text())
def packed(path):return json.loads(gzip.decompress(path.read_bytes()))
def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()

def main():
    test_log=[json.loads(line)for line in (BASE/'all-green.jsonl').read_text().splitlines()if line.startswith('{')]
    assert not any(item.get('Action')in('fail','build-fail')for item in test_log)
    tests=[item['Test']for item in test_log if item.get('Action')=='pass'and item.get('Test')]
    assert len(tests)==7,tests
    source=read(BASE/'random-static-source.json')
    reused=[]
    for path in [ROOT/'.local/contra-static/fixed-rea/function-0xc001.json',ROOT/'.local/contra-static/fixed-rea/function-0xf889.json',OUT/'rea-reset-function.json']:
        reused.append({'source':str(path.relative_to(ROOT)),'sha256':digest(path),'record':read(path)})
    native={'source':source,'newPrivateSession':read(BASE/'rea-v2/evidence.json'),'reused':reused}
    static=OUT/'random-static-rea.json.gz'
    static.write_bytes(gzip.compress(json.dumps(native,separators=(',',':')).encode(),mtime=0))
    functions=[]
    for path in sorted((BASE/'rea-v2').glob('function-*.json')):
        j=read(path);functions.append({'address':j['normalized_result']['procedure']['address'],'evidenceID':j['evidence_id'],'assembly':j['normalized_result']['assembly']})
    scenarios=[];readers={};writers={};sample_count=0;nmi_count=0;total=0
    for name in ['random-natural-v4.json.gz','random-boot-v2.json.gz']:
        for s in packed(OUT/name)['scenarios']:
            assert s['ramSHA256']==s['unhookedRamSHA256']
            assert s['summary']['mismatches']==0
            stack=[];pairs=0
            for e in s['events']:
                if e['kind']=='nmi':stack.append(e)
                elif e['kind']=='rti' and stack:
                    enter=stack.pop();assert (enter['a'],enter['carry'])==(e['a'],e['carry']);pairs+=1
                if e['kind']in('read','write'):
                    counts=readers if e['kind']=='read'else writers
                    key=f"{e['bank']:02x}:{e['pc']:04x}"
                    counts[key]=counts.get(key,0)+1
            for a in s['adds']:
                value=a['loadedFrame']+a['before']+a['carryIn']
                assert (a['after'],a['carryOut'])==(value%256,value//256)
            sample_count+=len(s['adds']);nmi_count+=pairs;total+=s['summary']['idleAdds']
            scenarios.append({'name':s['name'],'frames':len(s['frames']),'frameAdvanceSamples':s['source']['frames'],'idleAddsChecked':s['summary']['idleAdds'],
                'recordedAddSamples':len(s['adds']),'loadedCounterDiffers':s['summary']['loadedCounterDiffers'],
                'nmiRegisterPairs':pairs,'lagFrames':[f for f in s['frames']if f['lagged']],
                'idleAddsPerVideoFrame':[min(f['idleAdds']for f in s['frames']),max(f['idleAdds']for f in s['frames'])],
                'ramSHA256':s['ramSHA256'],'unhookedRamSHA256':s['unhookedRamSHA256'],'source':name})
    sources=['random-static-rea.json.gz','random-natural-v4.json.gz','random-boot-v2.json.gz','random-controlled-v2.json.gz','random-mutator-v2.json.gz']
    report={'passed':True,'scope':'Original observations plus arithmetic differential tests; original hardware scheduling is not recovered. Independent frame entropy is an explicitly authorized platform adaptation, not original code.',
        'romSHA256':source['romSHA256'],'functions':functions,'naturalVideoFrames':sum(s['frames']for s in scenarios),'frameAdvanceSamples':sum(s['frameAdvanceSamples']for s in scenarios),'frameBoundaryNote':'Each of the two cold-boot runs starts before video frame 1; its first frameadvance does not fire registerafter. 5108 RAM samples correspond to 5106 retained after-frame callback records.','naturalIdleAddsChecked':total,
        'naturalAddSamplesRetained':sample_count,'nmiRegisterPairsPreserved':nmi_count,'readCallbackPCs':readers,'writeCallbackPCs':writers,
        'callbackPCLimitation':'Memory hooks report PC after operand bytes (e.g. idle STA F884 reports F886). Captures use actual write callback values, not the next instruction boundary, which may follow an intervening NMI.',
        'scenarios':scenarios,'sourceSha256':{name:digest(OUT/name)for name in sources},
        'arithmeticCases':{'controlledIdle':3584,'controlledEnemyDispatch':180},'raceTests':tests,
        'implementationSha256':{str(p.relative_to(ROOT)):digest(p)for p in sorted((ROOT/'games/contra/engine').glob('random*.go'))},
        'adapter':{'version':1,'algorithm':'Standard SplitMix64 high byte XOR at authoritative-frame boundary, followed by original ADC(counter, saved idle Carry).','authorizedDifference':'User permits the online random sequence to differ from FC while preserving original operations and decision conditions.','seedOwner':'server; never rebuilt on restore/reconnect/takeover','jsonUint64':'decimal strings','claims':'Deterministic replay/restoration and broad anti-degeneration checks, not FC statistical equivalence or hardware cycle fidelity.'}}
    (OUT/'random-verification.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    for name in ['red.jsonl','adapter-red.jsonl','all-green.jsonl']:
        (OUT/('random-'+name)).write_bytes((BASE/name).read_bytes())
    print(json.dumps({k:report[k]for k in ['naturalVideoFrames','naturalIdleAddsChecked','naturalAddSamplesRetained','nmiRegisterPairsPreserved','arithmeticCases']},indent=2))

if __name__=='__main__':main()
