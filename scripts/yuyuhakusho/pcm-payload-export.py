"""Export address-free DPCM resources; preserve six music payloads in score JSON."""
from pathlib import Path
import argparse, base64, gzip, hashlib, json
from extract_audio_assets import sample_descriptor

ROOT=Path(__file__).resolve().parents[2]
ROM_SHA='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def canonical(value):return json.dumps(value,sort_keys=True,separators=(',',':')).encode()
def descriptor(source):
    fields={'sample_id':source['encoded_sha256'],'native_rate':source['rate_byte'],'flags':source['flags'],'protocol':'ordinary','routing_or_pan':source['routing_byte']}
    return {'id':'descriptor-'+sha(canonical(fields))[:20],**fields}
def remapped_f0(rom):
    address=0x38478;raw=rom[address:address+8];assert raw.hex()=='0b80008014814000'
    bank=(raw[0]<<16)|((raw[1]&128)<<8);head=bank+(int.from_bytes(raw[2:4],'little')&0x7fff);header=rom[head:head+4]
    begin=bank+(int.from_bytes(header[:2],'little')&0x7fff);length=int.from_bytes(header[2:],'little')
    assert length==6670 and begin+length<=bank+32768
    return {'descriptor_address':address,'descriptor_bytes':raw.hex(),'header_address':head,'header_bytes':header.hex(),'sample_address':begin,'encoded_bytes':length,'encoded_sha256':sha(rom[begin:begin+length]),'rate_byte':raw[4],'flags':raw[5],'routing_byte':raw[6],'source_index':122,'route':'remapped-ordinary'}
def main(out,proof):
    out=out.resolve();proof=proof.resolve();assert out.is_relative_to(ROOT)and proof.is_relative_to(ROOT)
    out.mkdir(parents=True,exist_ok=True);proof.mkdir(parents=True,exist_ok=True)
    rom_path=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rom_path.read_bytes();assert sha(rom)==ROM_SHA
    score_path=ROOT/'games/yuyuhakusho/data/audio-score.json.gz';score_packed=score_path.read_bytes();score_raw=gzip.decompress(score_packed);score=json.loads(score_raw)
    assert score['schema']==2 and score['format']=='typed-audio-score-v2'
    ordinary=[];diagnostics=[];payloads={};provenance=[]
    def add(source,code,route,domain):
        payload=rom[source['sample_address']:source['sample_address']+source['encoded_bytes']];identity=sha(payload)
        assert identity==source['encoded_sha256'] and identity not in payloads
        payloads[identity]={'id':identity,'codec':'dpcm4','data_base64':base64.b64encode(payload).decode()}
        row={'code':code,'route':route,'descriptor':descriptor(source)}
        (diagnostics if domain=='diagnostic'else ordinary).append(row)
        provenance.append({'command':code,'domain':domain,'route':route,'descriptor_id':row['descriptor']['id'],'native_source':source})
    for code in range(6,127):add(sample_descriptor(rom,code),code,'direct','ordinary')
    # F0 is the original dispatcher's extended-ordinary remap to table index122,
    # not the direct code7A entry (index116). Keep addresses only in provenance.
    f0=remapped_f0(rom)
    add(f0,0xf0,'remapped-ordinary','ordinary')
    add(sample_descriptor(rom,127),127,'internal-silent','diagnostic')
    music_payloads={row['id']:row for row in score['dac_payloads']};assert len(music_payloads)==len(score['dac_descriptors'])==6
    assert not set(music_payloads).intersection(payloads)
    data={'schema':1,'format':'typed-pcm-payloads-v1','ordinary':ordinary,'diagnostics':diagnostics,'payloads':[payloads[key]for key in sorted(payloads)],'music_import':{'format':'typed-audio-score-v2','logical_path':'/yuyuhakusho/data/audio-score.json.gz','raw_json_sha256':sha(score_raw),'raw_json_bytes':len(score_raw),'descriptors':score['dac_descriptors']}}
    raw=canonical(data)+b'\n';packed=gzip.compress(raw,mtime=0)
    path=out/'audio-pcm-payloads.json.gz';path.write_bytes(packed)
    manifest={'schema':1,'format':'typed-pcm-payloads-v1','logical_path':'/yuyuhakusho/data/audio-pcm-payloads.json.gz','raw_json_bytes':len(raw),'raw_json_sha256':sha(raw),'gzip_bytes':len(packed),'gzip_sha256':sha(packed),'ordinary_commands':len(ordinary),'internal_silent_commands':len(diagnostics),'resident_payloads':len(payloads),'imported_music_payloads':len(music_payloads),'combined_unique_payloads':len(payloads)+len(music_payloads),'music_resource':{'logical_path':'/yuyuhakusho/data/audio-score.json.gz','raw_json_bytes':len(score_raw),'raw_json_sha256':sha(score_raw),'gzip_bytes':len(score_packed),'gzip_sha256':sha(score_packed)},'server_embedded':False,'uploaded':False,'limits':['7f is an internal-silent descriptor, not part of the finite WAV list','F1..FF are not opened by this profile','No unproven prefetch padding is stored','Original playback timing and platform output are separate validations']}
    (out/'audio-pcm-payloads-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    record={'pass':True,'rom_sha256':sha(rom),'exporter_sha256':sha(Path(__file__).read_bytes()),'original_descriptor_helper_sha256':sha((ROOT/'scripts/yuyuhakusho/extract_audio_assets.py').read_bytes()),'score_gzip_sha256':sha(score_packed),'resource_manifest':manifest,'source_records':provenance,'music_payloads_imported_without_copy':sorted(music_payloads)}
    (proof/'export-provenance.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps({'pass':True,'resident_payloads':len(payloads),'imported':len(music_payloads),'raw_json_bytes':len(raw),'gzip_bytes':len(packed),'gzip_sha256':sha(packed)}))
if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);parser.add_argument('--proof',type=Path,required=True);args=parser.parse_args();main(args.out,args.proof)
