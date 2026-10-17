"""Export evidence-bound original labels, separate from gameplay allowlists.
Japanese transcriptions were read from original menu/HUD pixels. Chinese labels
are editorial suggestions, not an assertion of an official localization.
"""
from pathlib import Path
import hashlib,json
from PIL import Image,ImageDraw
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/selection-names'
NAMES=[['くにお','りき','ごだい','ジョニー'],['マイケル','エディ','セラリ','ルード'],['ミック','ショーン','リック','ライン'],['スナイプ','ホーク','ジャガー','ジミー'],['アッシュ','ウルフ','トライブ','ブレイク'],['きらうえ','へっど','わいいき','ぼーん'],['スロット','ルーク','ポーカー','ギャモン'],['ピエール','ホフマン','エスパ','グリース']]
TEAM_JA=['ねっけつ','ニューヨーク','サンフランシスコ','テキサス','US NAVY','ハワイ','ラスベガス','フロリダ']
TEAM_ZH=['热血','纽约','旧金山','得克萨斯','美国海军','夏威夷','拉斯维加斯','佛罗里达']
COURT_JA=['ナイアガラ',*TEAM_JA[1:]]
COURT_ZH=['尼亚加拉',*TEAM_ZH[1:]]
CHAR_ZH=[['国夫','阿力','五代','约翰尼'],['迈克尔','埃迪','塞拉利','鲁德'],['米克','肖恩','里克','莱恩'],['斯奈普','霍克','贾格','吉米'],['阿什','沃尔夫','特赖布','布雷克'],['基拉韦','海德','威基基','博恩'],['斯洛特','卢克','波克','加蒙'],['皮埃尔','霍夫曼','埃斯帕','格里斯']]
def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
    rom=ROOT/'.local/nes-basket/basketball.nes';raw=rom.read_bytes();assert digest(rom)=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    bank=raw[16+4*16384:16+5*16384];fixed=raw[16+7*16384:16+8*16384];team_bank=raw[16+3*16384:16+4*16384]
    def word(source,address):return int.from_bytes(source[address-0x8000:address-0x8000+2],'little')
    def codes(source,pointer,length):
        result=[]
        for value in source[pointer-0x8000:pointer-0x8000+length]:
            if value==255:return result
            result.append(value)
        return result
    def image_label(encoded,mode):
        glyph=Image.open(ROOT/'.local/basketball/court/hud-glyphs.png');result=Image.new('RGBA',(8*len(encoded),16))
        for x,v in enumerate(encoded):
            mark=[0,229][mode];flag=fixed[0x3f+v]
            if flag: v-=1 if flag==12 else 2;mark=([12,254] if flag==12 else [13,255])[mode]
            for y,tile in enumerate((mark,v)):result.paste(glyph.crop((tile%16*8,tile//16*8,tile%16*8+8,tile//16*8+8)),(x*8,y*8))
        return result
    menu=json.loads((OUT/'natural-menu.json').read_text());courts=[];teams=[]
    for observed in menu['teams']+menu['courts']:
        directory=ROOT/observed['path'];frame=observed['selectedFrame']
        observed['evidenceSHA256']={name:digest(directory/name) for name in ('scenario.json','ram.bin',f'frame-{frame:05d}.png')}
        if 'menuFrame' in observed:
            name=f"frame-{observed['menuFrame']:05d}.png";observed['evidenceSHA256'][name]=digest(directory/name)
    glyph_map={};sheet=Image.new('RGB',(660,8*66),'#ddd');draw=ImageDraw.Draw(sheet)
    for team in range(8):
        observed=next(x for x in menu['teams'] if x['state']['teams'][0]==team)
        pointer=word(team_bank,0xb6a7+2*team);encoded=codes(team_bank,pointer,13)
        entry=dict(id=team,originalLabel=TEAM_JA[team],suggestedChineseLabel=TEAM_ZH[team],translationAuthority='editorial-not-official',nameVerified=True,source=dict(bank=3,pointer=f'0x{pointer:04x}',encodedBytes=encoded,naturalMenu=observed),characters=[])
        table=word(bank,0xb24e+2*team)
        draw.text((2,team*66+3),f'T{team}',fill='black')
        sheet.paste(image_label(encoded,0).resize((len(encoded)*16,32),Image.Resampling.NEAREST),(24,team*66))
        for char in range(4):
            ptr=word(bank,table+2*char);encoded=codes(bank,ptr,4)
            while encoded and encoded[-1]==0:encoded.pop()
            text=NAMES[team][char];assert len(encoded)==len(text),(team,char,encoded,text)
            for byte,letter in zip(encoded,text):
                assert byte not in glyph_map or glyph_map[byte]==letter,(byte,glyph_map.get(byte),letter)
                glyph_map[byte]=letter
            entry['characters'].append(dict(id=f'team-{team}-character-{char}',team=team,character=char,originalLabel=text,suggestedChineseLabel=CHAR_ZH[team][char],translationAuthority='editorial-not-official',nameVerified=True,source=dict(bank=4,pointer=f'0x{ptr:04x}',encodedBytes=encoded)))
            sheet.paste(image_label(encoded,1).resize((len(encoded)*24,48),Image.Resampling.NEAREST),(270+char*96,team*66))
        teams.append(entry)
    for court in range(8):
        observed=next(x for x in menu['courts'] if x['state']['court']==court)
        courts.append(dict(id=court,originalLabel=COURT_JA[court],suggestedChineseLabel=COURT_ZH[court],translationAuthority='editorial-not-official',nameVerified=True,source=observed))
    sheet.save(OUT/'verified-labels.png')
    doc=dict(schemaVersion=1,accepted_for_release=False,authority='label-evidence-not-gameplay-allowlist',romSHA256=digest(rom),sourceRecords={'natural-menu.json':digest(OUT/'natural-menu.json'),'team-pairs.json':digest(OUT/'team-pairs.json'),'member-collisions.json':digest(OUT/'member-collisions.json')},courts=courts,teams=teams,characterGlyphTranscriptions={str(k):v for k,v in sorted(glyph_map.items())},relationships=dict(teamAndCourtAreSeparate=True,teamZeroCourtZeroNamesDiffer=True,versusDistinctTeams='56 ordered distinct pairs selected through original menu; cursor skips chosen opponent',withinTeamDistinctCharacters='all eight original team menus reject moving partner cursor from 1 upward to already selected character 0; other selected-character/multiplayer permutations not exhaustively tested',platformPolicy='server separately controls verified playable combinations; this metadata opens nothing'))
    (ROOT/'games/basketball/selection/names.json').write_text(json.dumps(doc,ensure_ascii=False,indent=2)+'\n')
if __name__=='__main__':main()
