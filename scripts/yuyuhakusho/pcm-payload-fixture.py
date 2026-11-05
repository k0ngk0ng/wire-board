"""Freeze independent source expectations before the PCM resource implementation.

Uses the pre-existing native WAV metadata and captured original 256-entry
mother tables, not the production PCM exporter or Go codec, for expectations.
"""
from pathlib import Path
import argparse, base64, gzip, hashlib, json

ROOT=Path(__file__).resolve().parents[2]
ROM_SHA='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def canonical(value):return json.dumps(value,sort_keys=True,separators=(',',':')).encode()
def ordinary_descriptor(row):
    value={'sample_id':row['encoded_sha256'],'native_rate':row['rate_byte'],'flags':row['flags'],'protocol':'ordinary','routing_or_pan':row['routing_byte']}
    return {'id':'descriptor-'+sha(canonical(value))[:20],**value}
def inspect(rom,code,index):
    address=0x380a8+index*8;raw=rom[address:address+8]
    bank=(raw[0]<<16)|((raw[1]&128)<<8)
    header_address=bank+(int.from_bytes(raw[2:4],'little')&0x7fff)
    header=rom[header_address:header_address+4]
    begin=bank+(int.from_bytes(header[:2],'little')&0x7fff);length=int.from_bytes(header[2:],'little')
    assert length>0 and begin+length<=bank+0x8000 and begin+length<=len(rom)
    return {'code':code,'descriptor_address':address,'descriptor_bytes':raw.hex(),'header_address':header_address,'header_bytes':header.hex(),'sample_address':begin,'encoded_bytes':length,'encoded_sha256':sha(rom[begin:begin+length]),'rate_byte':raw[4],'flags':raw[5],'routing_byte':raw[6]}

def main(out):
    out=out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    rom_path=ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin';rom=rom_path.read_bytes();assert sha(rom)==ROM_SHA
    metadata_path=ROOT/'games/yuyuhakusho/data/audio-assets.json';metadata=json.loads(metadata_path.read_bytes())
    music_path=ROOT/'games/yuyuhakusho/data/audio-score.json.gz';packed=music_path.read_bytes();music_raw=gzip.decompress(packed);music=json.loads(music_raw)
    provenance_path=ROOT/'.local/md-yuyuhakusho/audio-score-driver/official-with-fallback/provenance.json';music_provenance=json.loads(provenance_path.read_bytes())['dac_descriptors']
    table_path=ROOT/'games/yuyuhakusho/audio/pcm/testdata/original-values.json.gz';table_metadata=ROOT/'games/yuyuhakusho/audio/pcm/testdata/original-values-manifest.json'
    tm=json.loads(table_metadata.read_bytes());tables_raw=gzip.decompress(table_path.read_bytes());assert sha(table_path.read_bytes())==tm['fixture_gzip_sha256'] and sha(tables_raw)==tm['fixture_json_sha256']
    tables=json.loads(tables_raw)['tables'];mother=bytes(tables['mix']+tables['delta_low']+tables['delta_high']);assert sha(mother)==tables['source_table_sha256']
    rows=[];provenance=[];encoded_total=0;decoded_total=0
    def add(domain,code,descriptor,payload,source):
        nonlocal encoded_total,decoded_total
        assert sha(payload)==descriptor['sample_id'];acc=128;decoded=bytearray();last=None
        for b in payload:
            high=(acc+tables['delta_high'][b])&255;acc=(high+tables['delta_low'][b])&255;last=[high,acc];decoded.extend(last)
        rows.append({'domain':domain,'code':code,'descriptor':descriptor,'encoded_bytes':len(payload),'decoded_sha256':sha(decoded),'final_accumulator':acc,'last_encoded':payload[-1],'last_decoded':last})
        provenance.append({'domain':domain,'code':code,'descriptor_id':descriptor['id'],'source':source})
        encoded_total+=len(payload);decoded_total+=len(decoded)
    ordinary=[r for r in metadata['assets']if r['classification']=='pcm-sample-command'];assert [r['code']for r in ordinary]==list(range(6,127))
    for row in ordinary:
        actual=inspect(rom,row['code'],row['code']-6)
        assert all(actual[k]==row[k]for k in actual),row['code']
        payload=rom[row['sample_address']:row['sample_address']+row['encoded_bytes']]
        add('ordinary',row['code'],ordinary_descriptor(row),payload,actual)
    f0=inspect(rom,0xf0,122)
    assert f0['descriptor_bytes']=='0b80008014814000' and f0['encoded_bytes']==6670 and f0['encoded_sha256']=='c6409ebd28c594205970069fbc3233241cf15dc08ce8b847a5fa118b3e1a5382'
    add('ordinary',0xf0,ordinary_descriptor(f0),rom[f0['sample_address']:f0['sample_address']+f0['encoded_bytes']],f0|{'route':'remapped-ordinary','source_index':122})
    silent=next(r for r in metadata['diagnostics']if r.get('code')==127);actual=inspect(rom,127,121);assert all(actual[k]==silent[k]for k in actual)
    add('diagnostic',127,ordinary_descriptor(silent),rom[silent['sample_address']:silent['sample_address']+silent['encoded_bytes']],actual|{'route':'internal-silent','not_finite_wav':True})
    music_payloads={p['id']:base64.b64decode(p['data_base64'],validate=True)for p in music['dac_payloads']}
    assert len(music['dac_descriptors'])==len(music_payloads)==6
    for descriptor in music['dac_descriptors']:
        source=next(v for v in music_provenance.values()if v['id']==descriptor['id']);payload=music_payloads[descriptor['sample_id']]
        assert rom[source['payload']:source['payload']+len(payload)]==payload
        value={k:v for k,v in descriptor.items()if k!='id'};assert descriptor['id']=='descriptor-'+sha(canonical(value))[:20]
        projected={k:descriptor[k]for k in ['id','sample_id','native_rate','flags']};projected.update(protocol='track-pan',routing_or_pan=0)
        add('music',0,projected,payload,source)
    assert len(rows)==129 and len({r['descriptor']['sample_id']for r in rows})==129
    fixture={'schema':1,'scope':'Original source descriptors and complete decoder bytes using already verified native mother tables, not a new native timing or DSP oracle','rows':rows,'encoded_bytes':encoded_total,'decoded_bytes':decoded_total,'music_raw_sha256':sha(music_raw),'original_table_sha256':sha(mother)}
    fixture_path=out/'expected.json';fixture_path.write_text(json.dumps(fixture,indent=2)+'\n')
    sources={str(p.relative_to(ROOT)):sha(p.read_bytes())for p in [rom_path,metadata_path,music_path,provenance_path,table_path,table_metadata,Path(__file__)]}
    proof={'pass':True,'rows':len(rows),'encoded_bytes':encoded_total,'decoded_bytes':decoded_total,'production_implementation_present':(ROOT/'games/yuyuhakusho/audio/pcmassets/catalog.go').exists(),'fixture_sha256':sha(fixture_path.read_bytes()),'source_sha256':sources,'source_records':provenance,'limits':['Uses complete original lookup mother tables for value projection; does not rerun all 129 native playback flows','Existing finite WAV acquisition covers 121 ordinary sounds; F0 has independent static/remap evidence','No adjacent ROM padding is included in declared payloads']}
    (out/'source-provenance.json').write_text(json.dumps(proof,indent=2)+'\n');print(json.dumps({k:proof[k]for k in ['pass','rows','encoded_bytes','decoded_bytes','production_implementation_present','fixture_sha256']}))

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);main(parser.parse_args().out)
