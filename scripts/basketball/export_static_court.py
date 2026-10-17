"""Export independent original court layers, background tiles and HUD glyphs.
No complete-screen screenshots, captured nametables or ROM enter runtime assets.
"""
from pathlib import Path
from hashlib import sha256
import argparse,json
from static_sprites import SpriteROM,DIGEST
from static_court import build_court,build_hud_template,build_hud_glyphs,build_court_tiles,court_tile_layers,initial_court_ring,stream_court_column,hud_dynamic_cell
p=argparse.ArgumentParser();p.add_argument('--output',type=Path,default=Path('.local/basketball/court'));p.add_argument('--verification-log',type=Path,default=Path('.local/basketball/rea/static-background/tests-after.log'));args=p.parse_args()
rom=SpriteROM(Path('.local/nes-basket/basketball.nes').read_bytes());rgb_path=Path('.local/nes-basket/runs/graphics-right/nes-colors.rgb');colors=rgb_path.read_bytes();args.output.mkdir(parents=True,exist_ok=True)
report=None
for line in args.verification_log.read_text().splitlines():
 if line.startswith('{'):
  candidate=json.loads(line)
  if candidate.get('status')=='passed' and candidate.get('frames')==83:report=candidate
if report is None:raise ValueError('Run test_static_court.py successfully and retain its JSON evidence first')
report['verification_log_sha256']=sha256(args.verification_log.read_bytes()).hexdigest()
report['source_sha256']={str(path):sha256(path.read_bytes()).hexdigest() for path in [Path('scripts/basketball/static_court.py'),Path('scripts/basketball/test_static_court.py')]}
assets={}
def save_image(name,image):
 path=args.output/name;image.save(path);assets[name]={'sha256':sha256(path.read_bytes()).hexdigest(),'size':list(image.size)}
def save_json(name,data):
 path=args.output/name;path.write_text(json.dumps(data,indent=2)+'\n');assets[name]={'sha256':sha256(path.read_bytes()).hexdigest()}
court=build_court(rom,colors)
save_image('court.png',court);save_image('court-upper.png',court.crop((0,0,512,192)));save_image('court-lower.png',court.crop((0,192,512,432)))
save_image('court-tiles.png',build_court_tiles(rom,colors));save_image('hud-template.png',build_hud_template(rom,colors));save_image('hud-glyphs.png',build_hud_glyphs(rom,colors))
layers=court_tile_layers(rom)
# Upper rows 0..5 belong to the original HUD; exclude them from court maps.
for row in range(6):layers[0]['tiles'][row]=[0]*64;layers[0]['palettes'][row]=[0]*64
save_json('court-map.json',{'court':1,'width_tiles':64,'rows_per_page':30,'reserved_upper_rows':6,'layers':layers})
ring=initial_court_ring(rom);ring['tiles'][0][:192]=[0]*192;save_json('initial-ring.json',ring)
packets={}
for phase,label in [(0,'initialization'),(1,'gameplay')]:
 packets[label]=[]
 for column in range(64):
  state=initial_court_ring(rom);stream_court_column(rom,state,column,phase=phase);slot=column&31;indices=[r*8+slot//4 for r in range(8)]
  data=[[state['tiles'][layer][row*32+slot] for row in range(30)] for layer in range(2)];data[0][:6]=[0]*6
  packets[label].append({'column':column,'slot':slot,'tiles':data,'attribute_indices':indices,'attributes':[[state['attributes'][layer][i] for i in indices] for layer in range(2)]})
save_json('stream-columns.json',packets)
save_json('pixel-verification.json',report)
manifest={'accepted_for_release':False,'source':{'rom_sha256':DIGEST,'rgb_sha256':sha256(colors).hexdigest(),'court':'bank1:$800E -> $8034; attributes $801E -> $8074','chr':'court bank1:$BEC5[1] = pages68..71; HUD fixed $F32F=96 and $F35C[phase8]=98 selects pages96..99','palette':'bank4:$962F preset2 -> $968B','stream':'bank1:$BC74/$BCFF'},'assets':assets,'camera':{'encoded_page_height':256,'physical_page_height':240,'invalid_low_bytes':[240,255],'physical_y_formula':'(encodedY >> 8) * 240 + (encodedY & 255)','court_source_y':48,'court_image_virtual_bounds':[0,48,512,480],'upper_layer_virtual_bounds':[0,48,512,240],'lower_layer_virtual_bounds':[0,240,512,480],'draw_court':'drawImage(court, -scrollX, 48 - physicalY); clip to viewport [0,0,256,194]','original_left_mask':{'rect':[0,0,8,192],'color':'#000000'},'ring_texture_atlas':{'tile_size':8,'columns':16,'palette_row_blocks':16},'ring_update':'For each committed StreamColumn, copy packet.tiles[plane][row] to row*32+packet.slot; assign packet.attributes to packet.attribute_indices. NMI display commit timing is separate.'},'hud':{'viewport_rect':[0,194,256,46],'template_source_rect':[0,3,256,46],'template_size':[256,49],'supported_phase_indices':[0,1,2,3,4,5,7,8],'glyph_atlas':{'tile_size':8,'columns':16,'tiles_per_palette':256,'palettes':4,'rect_formula':'[(tile%16)*8,(palette*16+floor(tile/16))*8,8,8]'},'digits':{str(i):i+1 for i in range(10)},'timer_colon_tile':249,'dynamic_regions':[{'name':'left_turn_marker_strip','rect':[16,0,72,8]},{'name':'right_turn_marker_strip','rect':[168,0,72,8]},{'name':'left_player_panel','rect':[8,8,80,24]},{'name':'clock_score_panel','rect':[96,8,64,24]},{'name':'right_player_panel','rect':[168,8,80,24]}],'dynamic_cells':[[x,y] for y in range(7) for x in range(32) if hud_dynamic_cell(x,y)],'clock_tiles':{'row':1,'columns':[16,17,18,19]},'score_tiles':{'row':3,'left_columns':[12,13,14],'right_columns':[17,18,19]},'dynamic_text_semantics':'Glyph pixels are recovered. Gameplay-to-Japanese-name, marker and complete HUD value formatting is not yet reconstructed.'},'verification':{'frames':83,'background_pixels':report['background_pixels'],'hud_pixels':report['hud_pixels'],'original_ring_and_glyph_mismatches':0,'continuous_crop_edge_differences':report['continuous_crop_edge_differences'],'observed_camera_y':[296,297]},'limitations':['Full continuous court cropping differs from original stale edge columns in 9044 observed pixels. Exact rendering requires the supplied ring/attribute packets and original display commit timing.','Pixel proof uses original recorded tile/attribute state as observation; it does not prove a complete Go camera-to-ring scheduling loop.','83 frames observe only lower camera page Y=296/297; upper scenery comes from static ROM and has no natural frame pixel validation here.','Scene phase6 selects HUD second bank14 instead of98; its special presentation has no exported glyph variant or observed validation yet.','Dynamic HUD values are excluded from the transparent base template and verified through independent glyph reconstruction; full state-driven HUD is unfinished.','Assets are local research/build outputs; no OSS upload or full-game visual acceptance has occurred.']}
(args.output/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps({'assets':len(assets),'accepted_for_release':False,'output':str(args.output)}))
