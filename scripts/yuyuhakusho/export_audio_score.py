"""Research extractor: native score bytes -> address-free typed note timelines."""
from pathlib import Path
import argparse,base64,copy,gzip,hashlib,json,math
from reference import ROOT,sha
from score_parameter_tables import read_fm_patch,read_pitch_envelopes,read_psg_volume_envelopes,read_score_pan_patterns,decode_score_pan_automation

MUSIC=[0x82,0x83,0x84,0x85,0x86,0x87,0x88,0x8a,0x8d,0x95,0x8f]
CODES=MUSIC+list(range(0xa0,0xac))
CHANNELS={0:'fm1',1:'fm2',2:'fm3',4:'fm4',5:'fm5',6:'fm6',0x80:'psg1',0xa0:'psg2',0xc0:'psg3',0xe0:'psg-noise'}
FLOW={0xf6,0xf7,0xf8,0xf9}

def signed(v):return v-256 if v>=128 else v

def canonical(value):return json.dumps(value,sort_keys=True,separators=(',',':')).encode()

class Registry:
    def __init__(self,rom):self.rom=rom;self.patches={};self.patch_sources={};self.descriptors={};self.payloads={};self.descriptor_sources={}
    def patch(self,bank,index):
        data=read_fm_patch(self.rom,bank,index).to_score_data();data.pop('id',None)
        identity='patch-'+sha(canonical(data))[:20]
        self.patches[identity]={'id':identity,**data};self.patch_sources[(bank,index)]=identity
        return identity
    def descriptor(self,note):
        address=0x37724+(note-0x81)*8
        if not 0x37724<=address<0x377ec:raise ValueError(('DAC note outside the recovered descriptor table',note))
        data=self.rom[address:address+8];bank=(data[0]<<16)|((data[1]&128)<<8)
        header=bank+(int.from_bytes(data[2:4],'little')&0x7fff)
        start=int.from_bytes(self.rom[header:header+2],'little');size=int.from_bytes(self.rom[header+2:header+4],'little');begin=bank+(start&0x7fff)
        if size<=0 or begin+size>len(self.rom)or begin+size>bank+32768:raise ValueError('DAC payload outside its native bank')
        payload=self.rom[begin:begin+size];sample_id=sha(payload)
        self.payloads[sample_id]={'id':sample_id,'codec':'dpcm4','data_base64':base64.b64encode(payload).decode()}
        descriptor={'sample_id':sample_id,'native_rate':data[4],'flags':data[5],'routing':'track-pan'}
        identity='descriptor-'+sha(canonical(descriptor))[:20];self.descriptors[identity]={'id':identity,**descriptor};self.descriptor_sources[note]={'id':identity,'address':address,'bytes':data.hex(),'header':header,'payload':begin,'payload_sha256':sample_id}
        return identity


def header(rom,code):
    music=code in MUSIC;entry=0x3a492+(code-0x81)*4 if music else 0x3a60c+(code-0xa0)*4
    base=int.from_bytes(rom[entry:entry+4],'big');bank=base+int.from_bytes(rom[base:base+2],'big');voices=[]
    if music:
        fm,psg=rom[base+2:base+4];mult=rom[base+4];tempo=rom[base+5];at=base+6
        for index in range(fm+psg):
            size=4 if index<fm else 6;raw=rom[at:at+size];channel='dac'if index==0 else'fm'+str(index)if index<fm else'psg'+str(index-fm+1)
            voices.append({'id':index,'channel':channel,'sequence':base+int.from_bytes(raw[:2],'big'),'duration_multiplier':mult,'bank':bank,'initial':{'transpose':signed(raw[2]),'level':raw[3],'pitch_control':raw[4]if size==6 else 0,'volume_envelope':raw[5]if size==6 else 0},'record_address':at,'record_hex':raw.hex()});at+=size
    else:
        mult,count=rom[base+2:base+4];tempo=0;at=base+4
        for index in range(count):
            raw=rom[at:at+6];channel=CHANNELS[raw[1]]
            voices.append({'id':index,'channel':channel,'sequence':base+int.from_bytes(raw[2:4],'big'),'duration_multiplier':mult,'bank':bank,'initial':{'transpose':signed(raw[4]),'level':raw[5],'pitch_control':0,'volume_envelope':0},'record_address':at,'record_hex':raw.hex()});at+=6
    return {'id':code,'kind':'music'if music else'effect-primary','initial_tempo':tempo,'start_phase':'next-driver-tick'if music else'current-driver-tick','voices':voices,'address':base,'bank':bank}


def compile_voice(rom,voice,registry,kind):
    pc=voice['sequence'];mult=voice['duration_multiplier'];duration=0;words=[0,0];stack=0x30;quanta=0;events=[];proof=[];seen={};notes=0
    def byte(index):return words[index//4]>>((3-index%4)*8)&255
    def store(index,value):
        shift=(3-index%4)*8;owner=index//4;words[owner]=(words[owner]&~(255<<shift))|((value&255)<<shift)
    def read(n):
        if not 0<=pc<=len(rom)-n:raise ValueError('Truncated score input')
        return rom[pc:pc+n]
    def emit(event,address,token):
        events.append({'at':quanta,**event});proof.append({'at':quanta,'address':address,'token':token,'event':len(events)-1})
    for _ in range(200000):
        if not 0<=pc<len(rom):raise ValueError('Score outside ROM')
        key=(pc,mult,duration,tuple(words),stack)
        if key in seen:
            first,index=seen[key];period=quanta-first
            if period<=0:raise ValueError('Zero-time score cycle')
            return {'prefix_quanta':first,'cycle_quanta':period,'prefix':events[:index],'cycle':[{**event,'at':event['at']-first}for event in events[index:]],'ended':False},proof
        seen[key]=(quanta,len(events));address=pc;token=rom[pc];pc+=1
        if token<0xe0:
            if token>=0x80:
                if pc<len(rom)and rom[pc]<0x80:duration=(rom[pc]*mult)&255;pc+=1
                if token==0x80:pitch={'kind':'rest'}
                elif voice['channel']=='dac':pitch={'kind':'dac','descriptor_id':registry.descriptor(token)}
                else:pitch={'kind':'chromatic','semitone':token-0x81}
            else:duration=(token*mult)&255;pitch={'kind':'previous'}
            emit({'kind':'note','pitch':pitch,'duration':duration or 256},address,token);notes+=1;quanta+=duration or 256
            continue
        if token in(0xf6,0xf8):
            data=read(2)
            if token==0xf8:
                stack-=4
                if stack<0x28:raise ValueError('Score return stack leaves recovered shared words')
                words[(stack-0x28)//4]=address+1
            pc=address+2+int.from_bytes(data,'big',signed=True);continue
        if token==0xf7:
            data=read(4);index,count=data[:2]
            if index>=8:raise ValueError('Score loop counter leaves recovered shared words')
            left=byte(index)or count;left=(left-1)&255;store(index,left)
            pc=address+4+int.from_bytes(data[2:4],'big',signed=True)if left else pc+4;continue
        if token==0xf9:
            if stack>=0x30:raise ValueError('Score return underflow')
            pc=words[(stack-0x28)//4]+2;stack+=4;continue
        if token==0xe0:
            value=read(1)[0];pc+=1
            if value&0x3f:raise ValueError('Pan operand changes unmodeled lower bits')
            emit({'kind':'pan','left':bool(value&0x80),'right':bool(value&0x40)},address,token)
        elif token==0xe1:emit({'kind':'detune','value':signed(read(1)[0])},address,token);pc+=1
        elif token==0xe4:
            mode=read(1)[0]
            if mode:data=read(5);definition=decode_score_pan_automation(*data);pc+=5
            else:definition=decode_score_pan_automation(0);pc+=1
            emit({'kind':'pan-automation','definition':definition.to_data()},address,token)
        elif token in(0xe5,0xe6,0xec):
            if token==0xe5:
                data=read(2);value=data[0]if voice['channel'].startswith('psg')else data[1];pc+=2
            else:value=read(1)[0];pc+=1
            emit({'kind':'level-add','delta':signed(value),'immediate_fm':token!=0xec},address,token)
        elif token==0xe7:emit({'kind':'tie'},address,token)
        elif token==0xe8:emit({'kind':'gate','ticks':read(1)[0]},address,token);pc+=1
        elif token==0xea:emit({'kind':'tempo','period':read(1)[0]},address,token);pc+=1
        elif token==0xeb:emit({'kind':'enqueue-replace-first','sound_id':read(1)[0]},address,token);pc+=1
        elif token==0xef:emit({'kind':'fm-patch','patch_id':registry.patch(voice['bank'],read(1)[0])},address,token);pc+=1
        elif token==0xf0:
            data=read(4);pc+=4;emit({'kind':'modulation','delay':data[0],'rate':data[1],'delta':signed(data[2]),'steps':data[3]},address,token)
        elif token in(0xe3,0xf2):
            if token==0xe3:emit({'kind':'all-key-off'},address,token)
            emit({'kind':'track-end','release_policy':'forced-rest'if kind=='effect-primary'else'none','clear_primary_priority':kind=='effect-primary','ends_driver_tick':voice['channel']=='dac'},address,token)
            return {'prefix_quanta':quanta,'cycle_quanta':0,'prefix':events,'cycle':[],'ended':True},proof
        elif token==0xf3:
            value=read(1)[0];pc+=1
            if value&0xf8!=0xe0:raise ValueError('Noise operand is not a PSG noise latch')
            emit({'kind':'psg-noise','white':bool(value&4),'clock':value&3},address,token)
        elif token==0xf5:emit({'kind':'volume-envelope','envelope_id':read(1)[0]},address,token);pc+=1
        elif token in(0xf1,0xf4):
            if token==0xf1:
                data=read(2);value=data[0]if voice['channel'].startswith('psg')else data[1];pc+=2
            else:value=read(1)[0];pc+=1
            emit({'kind':'pitch-control','envelope_id':value&127,'modulation_enabled':bool(value&128)},address,token)
        elif token==0xfa:mult=read(1)[0];pc+=1
        elif token==0xfb:emit({'kind':'transpose-add','semitones':signed(read(1)[0])},address,token);pc+=1
        elif token in(0xfc,0xfd):emit({'kind':'modulation-enable','enabled':token==0xfc},address,token)
        else:raise ValueError(('Unrestored score control semantic',hex(token),hex(address)))
    raise ValueError('Score control search bound exhausted')


def extended_psg_dividers(rom,scores):
    result={}
    for score in scores:
        for voice in score['voices']:
            if not voice['channel'].startswith('psg'):continue
            transpose=voice['initial']['transpose']&255
            timeline=voice['timeline']
            if sum(event['semitones']for event in timeline['cycle']if event['kind']=='transpose-add')%256:
                raise ValueError('PSG cycle changes transpose; reachable pitch set needs a longer parameter cycle')
            for event in timeline['prefix']+timeline['cycle']:
                if event['kind']=='transpose-add':transpose=(transpose+event['semitones'])&255
                elif event['kind']=='note'and event['pitch']['kind']=='chromatic':
                    index=(event['pitch']['semitone']+transpose)&127
                    if index>=70:result[index]=int.from_bytes(rom[0x3a1a6+index*2:0x3a1a8+index*2],'big')
    return [{'index':index,'divider':result[index]}for index in sorted(result)]


def build(rom):
    registry=Registry(rom);scores=[];provenance={}
    for code in CODES:
        source=header(rom,code);voices=[];voice_proof=[]
        for voice in source['voices']:
            timeline,trace=compile_voice(rom,voice,registry,source['kind'])
            initial=copy.deepcopy(voice['initial'])
            if voice['channel'].startswith('fm'):initial['patch_reference']=registry.patch(voice['bank'],0)
            voices.append({'id':voice['id'],'channel':voice['channel'],'initial':initial,'timeline':timeline})
            voice_proof.append({'id':voice['id'],'sequence':voice['sequence'],'record_address':voice['record_address'],'record_hex':voice['record_hex'],'events':trace})
        scores.append({'id':code,'kind':source['kind'],'initial_tempo':source['initial_tempo'],'start_phase':source['start_phase'],'voices':voices})
        provenance[str(code)]={'header_address':source['address'],'instrument_bank':source['bank'],'voices':voice_proof}
    library={'schema':1,'format':'typed-audio-score-v1','dsp_implemented':False,'scores':scores,'fm_patches':[registry.patches[key]for key in sorted(registry.patches)],
             'pitch_envelopes':[item.to_data()for item in read_pitch_envelopes(rom)],'psg_volume_envelopes':[item.to_data()for item in read_psg_volume_envelopes(rom)],'pan_patterns':[item.to_data()for item in read_score_pan_patterns(rom)],
             'pitch_tables':{'fm_fnums':[int.from_bytes(rom[0x38d96+i*2:0x38d98+i*2],'big')for i in range(12)],'psg_dividers':[int.from_bytes(rom[0x3a1a6+i*2:0x3a1a8+i*2],'big')for i in range(70)],'psg_extended_dividers':extended_psg_dividers(rom,scores)},
             'dac_descriptors':[registry.descriptors[key]for key in sorted(registry.descriptors)],'dac_payloads':[registry.payloads[key]for key in sorted(registry.payloads)]}
    provenance['patches']=[{'bank':bank,'index':index,'id':identity}for(bank,index),identity in sorted(registry.patch_sources.items())]
    provenance['dac_descriptors']=registry.descriptor_sources
    provenance['psg_extended_dividers']=[{'index':row['index'],'address':0x3a1a6+2*row['index'],'word':row['divider']}for row in library['pitch_tables']['psg_extended_dividers']]
    return library,provenance

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT);out.mkdir(parents=True,exist_ok=True)
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes();library,provenance=build(rom)
    raw=(json.dumps(library,separators=(',',':'))+'\n').encode();(out/'catalog.json.gz').write_bytes(gzip.compress(raw,mtime=0))
    (out/'provenance.json').write_text(json.dumps(provenance,indent=2)+'\n')
    print(json.dumps({'definitions':len(library['scores']),'voices':sum(len(score['voices'])for score in library['scores']),'patches':len(library['fm_patches']),'payloads':len(library['dac_payloads']),'bytes':len(raw)},indent=2))
