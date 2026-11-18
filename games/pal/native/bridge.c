// SPDX-License-Identifier: GPL-3.0-only
// Platform bridge for the pinned, unchanged SDLPAL independent rule core.
// The headless authority observes original text/UI and uses pure memory files.
// Rendering hooks are separate from rules; unbound gameplay calls remain fatal.
#include "main.h"
#include "wb_video.h"
#include "wb_scene.h"
#include "wb_rng.h"
#include "wb_ui.h"
#include "wb_raster.h"
#include "wb_palette.h"
#include "wb_font_metrics.h"
#include "wb_screen_layout.h"
#include <errno.h>

extern int wb_host_yield(int kind, int value) __attribute__((import_module("pal"), import_name("yield")));
extern void wb_host_fail(const char *message) __attribute__((import_module("pal"), import_name("fail")));
extern unsigned wb_host_inflate(const unsigned char *source,unsigned size,unsigned char *output,unsigned capacity) __attribute__((import_module("pal"),import_name("inflate")));

#ifndef WB_BUILD_PURPOSE
#define WB_BUILD_PURPOSE 1
#endif
#ifndef WB_TEST_BATTLE_HP
#define WB_TEST_BATTLE_HP 0
#endif
// Read-only compiled provenance. The Wasm stamp and artifact manifest must
// agree with these values before a core can join a retained runtime catalog.
unsigned wb_build_info(void){
 static const unsigned info[4]={1,WB_BUILD_PURPOSE,WB_TEST_BATTLE_HP,0};
 return (unsigned)info;
}

typedef struct {const char *name;const unsigned char *data;size_t size;} WBResource;
typedef struct {uint32_t magic;const WBResource *resource;size_t position;} WBFile;
static WBResource resources[16];
static int resource_count;
static WBFile handles[64];
static unsigned char continuation_stack[524288];
// Host observation/input exports must not reuse suspended C stack frames.
unsigned char wb_service_stack[262144] __attribute__((aligned(16)));
static struct {unsigned char *position,*end;} continuation={continuation_stack,continuation_stack+sizeof continuation_stack};
static uint32_t clock_ms, random_seed, current_pc, instruction_count, message_count,last_frame_clock,budget;
static int trigger_depth;
typedef struct {int kind,role,count,selected;int items[128*5];} WBMenu;
static WBMenu menu_stack[16];
static int menu_depth,menu_info[6+128*5];
static PALDIRECTION held_direction=kDirUnknown;
static DWORD queued_keys;
static int booted, current_message=-1;
static unsigned platform_players=1;
static int battle_info[87];
static int coop_info[8];
static int current_face;
static uint32_t view[32];
static int scene_ops[1+8192*8];
static int field_info[365+517*8];
extern int wb_walk_phase(void);
void wb_input(unsigned direction,unsigned keys);
static int scene_backup[1+8192*8],scene_ready,scene_ready_backup,clear_color,clear_color_backup;
static LPWSTR *words,*messages;
static int word_count,msg_count;
static SDL_Color colors[256];
static int palette_state[12]={0,0,1,0},palette_out_base[12];
static int palette_red_step,palette_out_red_step;
static SDL_Palette palette={256,colors};
static SDL_PixelFormat format={&palette,8,1};
static unsigned char screen_pixels[64000],backup_pixels[64000],present_pixels[64000];
static SDL_Surface screen={320,200,320,screen_pixels,&format};
static SDL_Surface backup={320,200,320,backup_pixels,&format};
static SDL_Surface physical={320,200,320,present_pixels,&format};
SDL_Surface *gpScreen=&screen,*gpScreenBak=&backup;
SDL_Surface *gpScreenReal=&physical;
BOOL bScaleScreen=TRUE;
WORD g_wShakeTime,g_wShakeLevel;
static void present(void){}
WBRenderBackend gRenderBackend={present};
volatile BOOL g_bRenderPaused;
volatile PALINPUTSTATE g_InputState;
TEXTLIB g_TextLib;
LPWSTR g_rcCredits[12];

static void fail(const char *message){wb_host_fail(message);__builtin_trap();}
void wb_platform_players(unsigned players){if(players<1||players>3)fail("invalid platform player count");platform_players=players;}
int wb_multiplayer(void){return platform_players>1;}
unsigned wb_coop_info(void){return (unsigned)coop_info;}
void WB_OriginalBattleCommitAction(BOOL repeat);
void wb_release(void);
void PAL_BattleCommitAction(BOOL repeat){
 unsigned current=g_Battle.UI.wCurPlayerIndex;
 unsigned action=repeat?g_Battle.rgPlayer[current].prevAction.ActionType:g_Battle.UI.wActionType;
 if(wb_multiplayer()&&action==kBattleActionCoopMagic){
  unsigned object=PAL_GetPlayerCooperativeMagic(gpGlobals->rgParty[current].wPlayerRole);
  if(object>=MAX_OBJECTS)fail("invalid original cooperative magic object");
  unsigned magic=gpGlobals->g.rgObject[object].magic.wMagicNumber;
  if(magic>=gpGlobals->g.nMagic)fail("invalid original cooperative magic number");
  memset(coop_info,0,sizeof coop_info);coop_info[0]=1;coop_info[1]=current;coop_info[2]=object;
  coop_info[3]=repeat?g_Battle.rgPlayer[current].action.sTarget:(SHORT)g_Battle.UI.iSelectedIndex;
  coop_info[4]=(1u<<(gpGlobals->wMaxPartyMemberIndex+1))-1u;
  for(unsigned i=0;i<=gpGlobals->wMaxPartyMemberIndex;i++)if(PAL_IsPlayerHealthy(gpGlobals->rgParty[i].wPlayerRole))coop_info[5]|=1u<<i;
  coop_info[6]=gpGlobals->g.lprgMagic[magic].wCostMP;
  int response;
  do{wb_release();response=wb_host_yield(6,object);}while(response==0);
  if(response!=1&&response!=2)fail("invalid cooperative magic response");
  memset(coop_info,0,sizeof coop_info);wb_release();
  if(response==2){g_Battle.UI.state=kBattleUISelectMove;g_Battle.UI.MenuState=kBattleMenuMain;return;}
 }
 WB_OriginalBattleCommitAction(repeat);
}
unsigned wb_battle_info(void){
 memset(battle_info,0,sizeof battle_info);int *b=battle_info;
 b[0]=gpGlobals->fInBattle;b[1]=g_Battle.BattleResult;b[2]=g_Battle.Phase;
 b[3]=g_Battle.UI.state;b[4]=g_Battle.UI.MenuState;b[5]=g_Battle.UI.wCurPlayerIndex;
 b[6]=g_Battle.UI.wSelectedAction;b[7]=g_Battle.UI.wActionType;b[8]=g_Battle.UI.wObjectID;b[9]=g_Battle.UI.iSelectedIndex;
 b[10]=g_Battle.UI.fAutoAttack;b[11]=g_Battle.fRepeat;b[12]=g_Battle.fForce;b[13]=g_Battle.fFlee;
 b[14]=gpGlobals->wMaxPartyMemberIndex+1;b[15]=gpGlobals->fInBattle?g_Battle.wMaxEnemyIndex+1:0;
 if(b[14]>3||b[15]>5)fail("native battle actor count exceeds limit");
 for(int i=0;i<b[14];i++){
  int *p=b+16+i*12;unsigned r=gpGlobals->rgParty[i].wPlayerRole;if(r>=MAX_PLAYER_ROLES)fail("invalid native battle role");
  p[0]=r;p[1]=gpGlobals->g.PlayerRoles.rgwHP[r];p[2]=gpGlobals->g.PlayerRoles.rgwMaxHP[r];
  p[3]=gpGlobals->g.PlayerRoles.rgwMP[r];p[4]=gpGlobals->g.PlayerRoles.rgwMaxMP[r];p[5]=gpGlobals->g.PlayerRoles.rgwLevel[r];
  p[6]=g_Battle.rgPlayer[i].state;p[7]=g_Battle.rgPlayer[i].action.ActionType;p[8]=g_Battle.rgPlayer[i].action.wActionID;
  p[9]=g_Battle.rgPlayer[i].action.sTarget;p[10]=g_Battle.rgPlayer[i].wCurrentFrame;p[11]=g_Battle.rgPlayer[i].fDefending;
 }
 for(int i=0;i<b[15];i++){
  int *e=b+52+i*7;BATTLEENEMY *enemy=&g_Battle.rgEnemy[i];
  e[0]=enemy->wObjectID;e[1]=enemy->e.wHealth;e[2]=enemy->state;e[3]=enemy->wCurrentFrame;
  e[4]=PAL_X(enemy->pos);e[5]=PAL_Y(enemy->pos);e[6]=enemy->iColorShift;
 }
 return (unsigned)b;
}
void wb_menu_push(int kind,int role){if(menu_depth>=16)fail("native menu nesting exceeds limit");WBMenu *m=&menu_stack[menu_depth++];memset(m,0,sizeof *m);m->kind=kind;m->role=role;}
void wb_menu_pop(void){if(menu_depth<=0)fail("native menu nesting underflow");menu_depth--;}
void wb_menu_role(int role){if(menu_depth)menu_stack[menu_depth-1].role=role;}
void wb_menu_list(LPCMENUITEM items,int count,int selected){
 if(!menu_depth)return;if(count<0||count>128)fail("native menu list exceeds limit");
 WBMenu *m=&menu_stack[menu_depth-1];m->count=count;m->selected=count?items[(selected>=0&&selected<count)?selected:0].wValue:0;
 for(int i=0;i<count;i++){int *out=m->items+i*5;out[0]=items[i].wValue;out[1]=items[i].wNumWord;out[2]=items[i].fEnabled;out[3]=PAL_X(items[i].pos);out[4]=PAL_Y(items[i].pos);}
}
unsigned wb_menu_info(void){
 memset(menu_info,0,sizeof menu_info);menu_info[0]=menu_depth;menu_info[4]=-1;
 if(menu_depth){WBMenu *m=&menu_stack[menu_depth-1];menu_info[1]=m->kind;menu_info[2]=m->count;menu_info[3]=m->selected;menu_info[4]=m->role;memcpy(menu_info+6,m->items,m->count*5*sizeof(int));}
 return (unsigned)menu_info;
}
WORD WB_OriginalReadMenu(LPITEMCHANGED_CALLBACK callback,LPCMENUITEM items,INT count,WORD initial,BYTE color);
WORD PAL_ReadMenu(LPITEMCHANGED_CALLBACK callback,LPCMENUITEM items,INT count,WORD initial,BYTE color){
 int own=menu_depth==0;if(own)wb_menu_push(12,-1);wb_menu_list(items,count,initial);
 WORD result=WB_OriginalReadMenu(callback,items,count,initial,color);if(own)wb_menu_pop();return result;
}
BOOL wb_platform_system(void){wb_menu_push(11,-1);while(wb_host_yield(5,0)==0){}wb_menu_pop();return TRUE;}
static int ui_ops[1+8192*8],ui_backup[1+8192*8];
static int dialog_waiting,dialog_acknowledged,dialog_page_count,dialog_page_ids[32],dialog_info[38];
typedef struct {const unsigned char *data;unsigned length;int source,id;} WBImage;
static WBImage ui_images[32];
static int ui_image_count;
#undef free
extern void free(void *pointer);
void wb_free(void *pointer){
 for(int i=ui_image_count-1;i>=0;i--)if(ui_images[i].data==pointer){memmove(ui_images+i,ui_images+i+1,(ui_image_count-i-1)*sizeof *ui_images);ui_image_count--;}
 free(pointer);
}
static void ui_add(int kind,int source,int id,int frame,int x,int y,int color,int small){
 int op[8]={kind,source,id,frame,x,y,color,small};
 // Repainting the exact same glyph/mask replaces its previous paint operation.
 // This bounds selected-menu animation without changing the painted result.
 for(int i=0;i<ui_ops[0];i++){
  int *p=ui_ops+1+i*8;
  if(p[0]==kind&&p[1]==source&&p[2]==id&&p[3]==frame&&p[4]==x&&p[5]==y&&p[7]==small){
   memmove(p,p+8,(ui_ops[0]-i-1)*8*sizeof(int));ui_ops[0]--;break;
  }
 }
 if(ui_ops[0]>=8192)fail("native UI observation overflow");
 memcpy(ui_ops+1+ui_ops[0]++*8,op,sizeof op);
}
unsigned wb_ui_ops(void){return (unsigned)ui_ops;}
unsigned wb_scene_state(void){return (unsigned)scene_ready|((unsigned)clear_color<<8);}
unsigned wb_palette_state(void){return (unsigned)palette_state;}
unsigned wb_palette_red_step(void){return (unsigned)palette_red_step;}
void wb_palette_set(int palette,int night,int factor){
 if(palette<0||factor<0||factor>64)fail("invalid original palette observation");
 memset(palette_state,0,sizeof palette_state);palette_state[0]=palette;palette_state[1]=!!night;
 palette_red_step=0;
 if(factor!=64){palette_state[2]=1;palette_state[3]=factor;}
}
void wb_palette_red(int palette,int night,int step){
 if(step<0||step>32)fail("invalid original red palette step");
 wb_palette_set(palette,night,64);palette_red_step=step;
}
void wb_palette_begin_out(void){memcpy(palette_out_base,palette_state,sizeof palette_state);palette_out_red_step=palette_red_step;}
void wb_palette_out(int factor){
 memcpy(palette_state,palette_out_base,sizeof palette_state);
 palette_red_step=palette_out_red_step;
 if(factor==0){palette_state[2]=1;memset(palette_state+3,0,9*sizeof(int));return;}
 if(factor==64)return;
 if(factor<0||factor>64||palette_state[2]>=8)fail("palette fade observation exceeds limit");
 palette_state[3+palette_state[2]++]=factor;
}
unsigned wb_dialog_info(void){
 memset(dialog_info,0,sizeof dialog_info);dialog_info[0]=dialog_waiting;dialog_info[1]=g_TextLib.nCurrentDialogLine;
 dialog_info[2]=g_TextLib.iDelayTime;dialog_info[3]=current_face;dialog_info[4]=dialog_page_count;dialog_info[5]=g_TextLib.bCurrentFontColor;
 memcpy(dialog_info+6,dialog_page_ids,dialog_page_count*sizeof(int));return (unsigned)dialog_info;
}
void wb_dialog_page(void){dialog_page_count=0;}
void wb_dialog_start(int face){current_face=face;wb_dialog_page();}
void wb_dialog_message(LPCWSTR text){
 if(current_message<0||current_message>=msg_count||text!=messages[current_message])fail("unbound original message");
 if(dialog_page_count>=32)fail("native dialogue page exceeds limit");
 dialog_page_ids[dialog_page_count++]=current_message;message_count++;
}
void wb_dialog_wait(float maximum){
 if(maximum>0)return;
 if(dialog_page_count==0)fail("native dialogue waits without a visible page");
 dialog_waiting=1;
 while(wb_host_yield(2,dialog_page_ids[dialog_page_count-1])==0){}
 dialog_acknowledged=1;
}
void wb_dialog_wait_end(void){dialog_waiting=0;dialog_acknowledged=0;}
void WB_OriginalClearDialog(BOOL wait);
void PAL_ClearDialog(BOOL wait){WB_OriginalClearDialog(wait);wb_dialog_page();}
#ifdef WB_REFERENCE_FONT
int WB_ReferenceCharWidth(uint16_t code);
void WB_ReferenceDrawCharOnSurface(uint16_t code,SDL_Surface *surface,PAL_POS pos,uint8_t color,BOOL small);
void WB_InitReferenceFont(void);
#endif
int PAL_CharWidth(uint16_t code){
#ifdef WB_REFERENCE_FONT
 return WB_ReferenceCharWidth(code);
#else
 if((code>=0xd800&&code<0xf900)||code>=65534)return 0;if(code>=0xf900)code-=0xf900-0xd800;return wb_font_width[code]>>1;
#endif
}
int PAL_FontHeight(void){return 16;}
void PAL_DrawCharOnSurface(uint16_t code,SDL_Surface *surface,PAL_POS pos,uint8_t color,BOOL small){
 if(PAL_CharWidth(code)>0)wb_raster_text(surface,code,PAL_X(pos),PAL_Y(pos),color,small);
 if(surface==gpScreen&&PAL_CharWidth(code)>0)ui_add(0,0,code,0,PAL_X(pos),PAL_Y(pos),color,small);
#ifdef WB_REFERENCE_FONT
 WB_ReferenceDrawCharOnSurface(code,surface,pos,color,small);
#endif
}
static void bind_image(const unsigned char *data,int length,int source,int id){
 if(length<=0||!data)return;
 uintptr_t start=(uintptr_t)data,end=start+(unsigned)length;
 if(end<start)fail("native image binding exceeds address range");
 // Original stack/scratch buffers are reused across image types. Any new
 // write invalidates every overlapping old identity, including a large FBP
 // enclosing this portrait and unknown archives replacing a former UI image.
 for(int i=ui_image_count-1;i>=0;i--){
  uintptr_t old=(uintptr_t)ui_images[i].data,old_end=old+ui_images[i].length;
  if(start<old_end&&old<end){memmove(ui_images+i,ui_images+i+1,(ui_image_count-i-1)*sizeof *ui_images);ui_image_count--;}
 }
 if(!source)return;
 if(ui_image_count>=32)fail("native UI image registry overflow");
 ui_images[ui_image_count++]=(WBImage){data,(unsigned)length,source,id};
}
void wb_ui_resource(const unsigned char *data,int length,unsigned chunk,FILE *file){
 int source=0,id=chunk;
 if(file==gpGlobals->f.fpRGM)source=1;
 else if(file==gpGlobals->f.fpBALL)source=5;
 else if(file==gpGlobals->f.fpFBP&&length==64000)source=4;
 else if(file==gpGlobals->f.fpDATA&&(chunk==CHUNKNUM_SPRITEUI||chunk==12||chunk==10)){source=chunk==12?3:chunk==10?9:2;id=chunk==10?10:0;}
 bind_image(data,length,source,id);
}
static void image_rle(LPCBITMAPRLE bitmap,SDL_Surface *surface,PAL_POS pos,int mode,int color,int shift){
 if(!bitmap||!PAL_RLEGetWidth(bitmap)||!PAL_RLEGetHeight(bitmap))return;
 for(int i=ui_image_count-1;i>=0;i--){
  WBImage image=ui_images[i];uintptr_t p=(uintptr_t)bitmap,start=(uintptr_t)image.data;
  if(p<start||p>=start+image.length)continue;
  int frame=0;
  if(image.source!=1&&image.source!=5&&image.source!=4){
   unsigned count=PAL_SpriteGetNumFrames(image.data);if(count>4096)fail("invalid native UI atlas");
   for(;frame<=(int)count;frame++)if(PAL_SpriteGetFrame(image.data,frame)==bitmap)break;
   if(frame>(int)count)fail("unbound native UI atlas frame");
  }
  wb_raster_image(surface,image.source,image.id,frame,PAL_X(pos),PAL_Y(pos),mode,color,shift);
  if(surface==gpScreen&&image.source<=5&&mode<=1)ui_add(1,image.source,image.id,frame,PAL_X(pos),PAL_Y(pos),mode,0);
  return;
 }
 if(wb_raster_active())fail("unbound original battle bitmap");
}
void wb_ui_rle(LPCBITMAPRLE bitmap,SDL_Surface *surface,PAL_POS pos,BOOL shadow){image_rle(bitmap,surface,pos,shadow?1:0,0,0);}
INT WB_OriginalRLEBlitToSurfaceWithShadow(LPCBITMAPRLE bitmap,SDL_Surface *surface,PAL_POS pos,BOOL shadow);
INT PAL_RLEBlitToSurfaceWithShadow(LPCBITMAPRLE bitmap,SDL_Surface *surface,PAL_POS pos,BOOL shadow){wb_ui_rle(bitmap,surface,pos,shadow);return WB_OriginalRLEBlitToSurfaceWithShadow(bitmap,surface,pos,shadow);}
INT WB_OriginalPAL_RLEBlitWithColorShift(LPCBITMAPRLE bitmap,SDL_Surface *surface,PAL_POS pos,INT shift);
INT PAL_RLEBlitWithColorShift(LPCBITMAPRLE bitmap,SDL_Surface *surface,PAL_POS pos,INT shift){image_rle(bitmap,surface,pos,2,0,shift);return WB_OriginalPAL_RLEBlitWithColorShift(bitmap,surface,pos,shift);}
INT WB_OriginalPAL_RLEBlitMonoColor(LPCBITMAPRLE bitmap,SDL_Surface *surface,PAL_POS pos,BYTE color,INT shift);
INT PAL_RLEBlitMonoColor(LPCBITMAPRLE bitmap,SDL_Surface *surface,PAL_POS pos,BYTE color,INT shift){image_rle(bitmap,surface,pos,3,color,shift);return WB_OriginalPAL_RLEBlitMonoColor(bitmap,surface,pos,color,shift);}
INT WB_OriginalPAL_MKFDecompressChunk(LPBYTE buffer,UINT size,UINT chunk,FILE *file);
INT PAL_MKFDecompressChunk(LPBYTE buffer,UINT size,UINT chunk,FILE *file){
 INT result=WB_OriginalPAL_MKFDecompressChunk(buffer,size,chunk,file);
 wb_ui_resource(buffer,result,chunk,file);
 if(file==gpGlobals->f.fpF)bind_image(buffer,result,6,chunk);
 else if(file==gpGlobals->f.fpFIRE)bind_image(buffer,result,8,chunk);
 else if(file&&((WBFile*)file)->magic==0x50414c46&&!strcmp(((WBFile*)file)->resource->name,"abc.mkf"))bind_image(buffer,result,7,chunk);
 return result;
}
INT WB_OriginalPAL_FBPBlitToSurface(LPBYTE bitmap,SDL_Surface *surface);
INT PAL_FBPBlitToSurface(LPBYTE bitmap,SDL_Surface *surface){
 if(surface&&surface==g_Battle.lpBackground)wb_raster_enable();
 if(bitmap&&wb_raster_active()){
  int found=0;for(int i=ui_image_count-1;i>=0;i--)if(ui_images[i].data==bitmap&&ui_images[i].source==4){wb_raster_image(surface,4,ui_images[i].id,0,0,0,0,0,0);found=1;break;}
  if(!found)fail("unbound original raster background");
 }
 if(bitmap&&surface==gpScreen&&surface->w==320&&surface->h==200){
  int found=0;for(int i=ui_image_count-1;i>=0;i--)if(ui_images[i].data==bitmap&&ui_images[i].source==4){ui_ops[0]=0;ui_add(1,4,ui_images[i].id,0,0,0,0,0);found=1;break;}
  if(!found)fail("unbound original FBP background");
 }
 return WB_OriginalPAL_FBPBlitToSurface(bitmap,surface);
}
void *wb_allocate(unsigned size){return malloc(size);}
unsigned wb_continuation(void){return (unsigned)&continuation;}
unsigned wb_pixels(void){return (unsigned)screen_pixels;}
unsigned wb_presentation_pixels(void){return (unsigned)present_pixels;}
unsigned wb_colors(void){return (unsigned)colors;}
void wb_scene_reset(void){scene_ready=1;scene_ops[0]=0;ui_ops[0]=0;if(!gpGlobals->fInBattle&&wb_raster_active())wb_raster_reset();}
static void scene_add(int kind,int a,int b,int c,int d,int x,int base_y,int sort_y){
 if(scene_ops[0]>=8192)fail("native scene observation overflow");
 int *out=scene_ops+1+scene_ops[0]++*8;out[0]=kind;out[1]=a;out[2]=b;out[3]=c;out[4]=d;out[5]=x;out[6]=base_y;out[7]=sort_y;
}
void wb_scene_sprite(int asset,int frame,int entity,int kind,int x,int base_y,int sort_y){scene_add(0,asset,frame,entity,kind,x,base_y,sort_y);}
// PAL_MakeScene waves the map before drawing actors/covering tiles. Keep that
// original stage in the scene command stream, including immutable raster seeds.
void wb_scene_wave(int level,int phase,int progression){scene_add(2,level,phase,progression,0,0,0,(-2147483647-1));}
void wb_scene_cover(int x,int y,int half,int layer,int height){
 scene_add(1,x,y,half,layer,x*32+half*16-16-PAL_X(gpGlobals->viewport),y*16+half*8+7-PAL_Y(gpGlobals->viewport),y*16+half*8+7+layer+height*8-PAL_Y(gpGlobals->viewport));
}
unsigned wb_scene_ops(void){return (unsigned)scene_ops;}
static int *raster_seeds[2];
static int backup_map,backup_camera_x,backup_camera_y;
static int rng_waiting;
static double rng_deadline;
void wb_raster_seed_clear(void){for(int i=0;i<2;i++){free(raster_seeds[i]);raster_seeds[i]=NULL;}}
int wb_raster_capture_seed(void){
 for(int slot=0;slot<2;slot++){
  int *draw=slot?scene_backup:scene_ops,*ui=slot?ui_backup:ui_ops;
  unsigned words=8+draw[0]*8+ui[0]*8;
  int *p=malloc(words*sizeof(int));if(!p)fail("RNG scene capture allocation failed");
  p[0]=1;p[1]=slot?scene_ready_backup:scene_ready;p[2]=slot?clear_color_backup:clear_color;
  p[3]=slot?backup_map:gpGlobals->g.rgScene[gpGlobals->wNumScene-1].wMapNum;
  p[4]=slot?backup_camera_x:PAL_X(gpGlobals->viewport);p[5]=slot?backup_camera_y:PAL_Y(gpGlobals->viewport);
  p[6]=draw[0];p[7]=ui[0];memcpy(p+8,draw+1,draw[0]*8*sizeof(int));memcpy(p+8+draw[0]*8,ui+1,ui[0]*8*sizeof(int));raster_seeds[slot]=p;
 }
 return 1;
}
unsigned wb_raster_seed_info(unsigned slot){if(slot>1||!raster_seeds[slot])fail("missing captured RNG screen");return (unsigned)raster_seeds[slot];}
void wb_rng_begin(void){wb_raster_rng_begin();}
void wb_rng_frame(int clip,int first,int frame){wb_raster_rng_frame(clip,first,frame);}
void wb_rng_wait_begin(double deadline){rng_waiting=1;rng_deadline=deadline;}
void wb_rng_wait_end(void){rng_waiting=0;}
// Bounded public movement state, with collision answers from the original
// routine. No scripts, RNG, native pointers or future event data cross it.
unsigned wb_field_info(void){
 memset(field_info,0,sizeof field_info);
 if(!scene_ready||gpGlobals->fEnteringScene||gpGlobals->fInBattle||trigger_depth||menu_depth||!PAL_GetCurrentMap())return (unsigned)field_info;
 int party=gpGlobals->wMaxPartyMemberIndex+1,followers=gpGlobals->nFollower;
 if(party<1||party>3||followers>2||party+followers>5)return (unsigned)field_info;
 int camera_x=PAL_X(gpGlobals->viewport),camera_y=PAL_Y(gpGlobals->viewport);
 int origin_x=camera_x+PAL_X(gpGlobals->partyoffset),origin_y=camera_y+PAL_Y(gpGlobals->partyoffset);
 field_info[0]=1;field_info[1]=gpGlobals->dwFrameNum;
 field_info[2]=origin_x;field_info[3]=origin_y;field_info[4]=camera_x;field_info[5]=camera_y;
 field_info[6]=PAL_X(gpGlobals->partyoffset);field_info[7]=PAL_Y(gpGlobals->partyoffset);
 field_info[8]=gpGlobals->wLayer;field_info[9]=gpGlobals->wPartyDirection;field_info[10]=wb_walk_phase();
 field_info[11]=party;field_info[12]=followers;field_info[14]=8;field_info[15]=334;
 for(int i=0;i<5;i++){field_info[16+i*3]=gpGlobals->rgTrail[i].x;field_info[17+i*3]=gpGlobals->rgTrail[i].y;field_info[18+i*3]=gpGlobals->rgTrail[i].wDirection;}
 int first=gpGlobals->g.rgScene[gpGlobals->wNumScene-1].wEventObjectIndex,last=gpGlobals->g.rgScene[gpGlobals->wNumScene].wEventObjectIndex;
 for(int v=-8;v<=8;v++)for(int u=-8;u<=8;u++){
  int x=origin_x+(u-v)*16,y=origin_y+(u+v)*8;
  int flags=PAL_CheckObstacleWithRange(PAL_XY(x,y),TRUE,0,TRUE)?1:0;
  for(int i=first;i<last;i++){
   LPEVENTOBJECT p=&gpGlobals->g.lprgEventObject[i];
   if(p->sState>0&&p->sVanishTime==0&&p->wTriggerMode>=kTriggerTouchNear&&
      abs(x-p->x)+abs(y-p->y)*2<(p->wTriggerMode-kTriggerTouchNear)*32+16){flags|=2;break;}
  }
  field_info[31+(v+8)*17+u+8]=flags;
 }
 // Scripted trails need not lie on the leader's walking lattice. Preserve
 // exact original collision answers for each old trail's follower offsets.
 for(int i=0;i<5;i++)for(int dy=-1;dy<=1;dy++)for(int dx=-1;dx<=1;dx++)
  field_info[31+289+i*9+(dy+1)*3+dx+1]=PAL_CheckObstacleWithRange(PAL_XY(gpGlobals->rgTrail[i].x+dx*16,gpGlobals->rgTrail[i].y+dy*8),TRUE,0,TRUE)?1:0;
 for(int i=0;i<party+followers;i++){
  int *out=field_info+365+field_info[13]++*8,role=gpGlobals->rgParty[i].wPlayerRole;
  out[0]=i<party?0:2;out[1]=role;out[2]=i<party?gpGlobals->g.PlayerRoles.rgwSpriteNum[role]:role;
  out[3]=camera_x+gpGlobals->rgParty[i].x;out[4]=camera_y+gpGlobals->rgParty[i].y;
  out[5]=gpGlobals->rgParty[i].wFrame;out[6]=i<party?gpGlobals->g.PlayerRoles.rgwWalkFrames[role]:3;out[7]=gpGlobals->wLayer;
 }
 for(int i=first;i<last;i++){
  LPEVENTOBJECT p=&gpGlobals->g.lprgEventObject[i];
  if(p->sState<=0||p->sVanishTime>0)continue;
  LPCSPRITE sprite=PAL_GetEventObjectSprite((WORD)i+1);if(!sprite)continue;
  int frame=p->wCurrentFrameNum;if(p->nSpriteFrames==3){if(frame==2)frame=0;else if(frame==3)frame=2;}
  frame+=p->wDirection*p->nSpriteFrames;
  LPCBITMAPRLE bitmap=PAL_SpriteGetFrame(sprite,frame);if(!bitmap)continue;
  int x=(SHORT)p->x-camera_x-PAL_RLEGetWidth(bitmap)/2;
  int vy=(SHORT)p->y-camera_y+11-PAL_RLEGetHeight(bitmap);
  if(x>=384||x<-(int)PAL_RLEGetWidth(bitmap)-64||vy>=264||vy<-(int)PAL_RLEGetHeight(bitmap)-64)continue;
  if(field_info[13]>=517){field_info[0]=0;return (unsigned)field_info;}
  int *out=field_info+365+field_info[13]++*8;
  out[0]=1;out[1]=i+1;out[2]=p->wSpriteNum;out[3]=(SHORT)p->x;out[4]=(SHORT)p->y;
  out[5]=frame;out[6]=p->nSpriteFrames;out[7]=p->sLayer*8;
 }
 return (unsigned)field_info;
}
// Verification-only motor fixtures, never a platform command or campaign path.
void wb_probe_scene_wave(unsigned level,int progression){
 if(level>65535||progression<-32768||progression>32767)fail("invalid scene wave fixture");
 gpGlobals->wScreenWave=level;gpGlobals->sWaveProgression=progression;PAL_MakeScene();
}
void wb_probe_presentation_fade(void){
 PAL_DrawOpeningMenuBackground();
 PAL_SetPalette(0,FALSE);
 SDL_BlitSurface(gpScreen,NULL,gpScreenBak,NULL);
 wb_raster_enable();
 PAL_DrawText(L"SDLPAL",PAL_XY(40,64),0x4F,TRUE,FALSE,FALSE);
 VIDEO_ShakeScreen(4,3);
 VIDEO_FadeScreen(9);
 for(;;)wb_host_yield(3,0);
}
void wb_probe_field_config(unsigned party,unsigned followers,unsigned walk,unsigned odd){
 if(party<1||party>3||followers>2||party+followers>5||(walk!=0&&walk!=3&&walk!=4))fail("invalid field motor fixture");
 gpGlobals->wMaxPartyMemberIndex=party-1;gpGlobals->nFollower=followers;
 for(unsigned i=0;i<party;i++){gpGlobals->rgParty[i].wPlayerRole=i;gpGlobals->g.PlayerRoles.rgwWalkFrames[i]=walk;if(walk==4)gpGlobals->g.PlayerRoles.rgwSpriteNum[i]=1;gpGlobals->rgParty[i].x=PAL_X(gpGlobals->partyoffset);gpGlobals->rgParty[i].y=PAL_Y(gpGlobals->partyoffset);}
 for(unsigned i=party;i<party+followers;i++){gpGlobals->rgParty[i].wPlayerRole=gpGlobals->g.PlayerRoles.rgwSpriteNum[0];gpGlobals->rgParty[i].x=PAL_X(gpGlobals->partyoffset);gpGlobals->rgParty[i].y=PAL_Y(gpGlobals->partyoffset);}
 if(odd)for(int i=0;i<5;i++){gpGlobals->rgTrail[i].x=PAL_X(gpGlobals->viewport)+PAL_X(gpGlobals->partyoffset)-20-i*16;gpGlobals->rgTrail[i].y=PAL_Y(gpGlobals->viewport)+PAL_Y(gpGlobals->partyoffset)+10-i*8;gpGlobals->rgTrail[i].wDirection=kDirNorth;}
 PAL_SetLoadFlags(kLoadPlayerSprite);PAL_LoadResources();PAL_UpdatePartyGestures(FALSE);PAL_MakeScene();
}
void wb_probe_field_move(unsigned direction){
 if(direction>4)fail("invalid field motor direction");
 wb_input(direction,0);PAL_ProcessEvent();PAL_UpdateParty();PAL_MakeScene();gpGlobals->dwFrameNum++;
}
// Verification export: run the original blitter on a caller-owned surface.
// The two fill values in the raster oracle reveal transparency independently.
unsigned wb_probe_rle(const unsigned char *bitmap,unsigned width,unsigned height,unsigned fill){
 if(width>4096||height>4096||(uint64_t)width*height>4194304)fail("invalid raster reference size");
 unsigned char *pixels=malloc(width*height?width*height:1);if(!pixels)fail("raster reference allocation failed");
 memset(pixels,(unsigned char)fill,width*height);SDL_Surface surface={(int)width,(int)height,(int)width,pixels,&format};
 if(PAL_RLEBlitToSurface(bitmap,&surface,PAL_XY(0,0))!=0)fail("original raster reference failed");return (unsigned)pixels;
}

int wb_bind_file(const char *name,const unsigned char *data,unsigned size){
 if(booted||resource_count>=16||!name||!data||size<4)return 0;
 size_t n=strlen(name);if(!n||n>64||strchr(name,'/')||strchr(name,'\\'))return 0;
 for(int i=0;i<resource_count;i++)if(!strcmp(resources[i].name,name))return 0;
 resources[resource_count++]=(WBResource){name,data,size};return 1;
}
void wb_bind_text(int kind,LPWSTR *table,int count){
 if(booted||!table||count<1)fail("invalid native text table");
 if(kind==0){words=table;word_count=count;}else if(kind==1){messages=table;msg_count=count;}else fail("unknown native text table");
}
static FILE *open_resource(const char *name,const char *mode){
 if(strcmp(mode,"rb")&&strcmp(mode,"r"))fail("native rule core cannot write resource files");
 for(int i=0;i<resource_count;i++)if(!strcmp(resources[i].name,name)){
  for(int j=0;j<64;j++)if(handles[j].magic==0){WBFile *file=&handles[j];*file=(WBFile){0x50414c46,&resources[i],0};return (FILE*)file;}
  fail("native resource handle limit");
 }
 return NULL;
}
size_t wb_fread(void *output,size_t size,size_t count,FILE *handle){
 if(!size||!count)return 0;
 WBFile *f=(WBFile*)handle;
 if(!f||f->magic!=0x50414c46||!output||count>SIZE_MAX/size)fail("invalid native resource read");
 size_t available=(f->resource->size-f->position)/size;if(count>available)count=available;
 memcpy(output,f->resource->data+f->position,size*count);f->position+=size*count;return count;
}
int wb_fseek(FILE *handle,long offset,int origin){
 WBFile *f=(WBFile*)handle;if(!f||f->magic!=0x50414c46)fail("invalid native resource seek");
 int64_t base=origin==SEEK_SET?0:origin==SEEK_CUR?(int64_t)f->position:origin==SEEK_END?(int64_t)f->resource->size:-1;
 int64_t next=base+offset;if(base<0||next<0||(uint64_t)next>f->resource->size){errno=EINVAL;return -1;}
 f->position=(size_t)next;return 0;
}
long wb_ftell(FILE *handle){WBFile *f=(WBFile*)handle;if(!f||f->magic!=0x50414c46)fail("invalid native resource tell");return (long)f->position;}
int wb_fclose(FILE *handle){WBFile *f=(WBFile*)handle;if(!f||f->magic!=0x50414c46)fail("invalid native resource close");f->magic=0;return 0;}
size_t wb_fwrite(const void *p,size_t size,size_t count,FILE *f){(void)p;(void)size;(void)count;(void)f;fail("native rule core cannot write files");return 0;}
FILE *UTIL_OpenFile(LPCSTR name){return open_resource(name,"rb");}
FILE *UTIL_OpenFileForMode(LPCSTR name,LPCSTR mode){return open_resource(name,mode);}
FILE *UTIL_OpenRequiredFile(LPCSTR name){FILE *f=open_resource(name,"rb");if(!f)fail("missing native rule resource");return f;}
FILE *UTIL_OpenRequiredFileForMode(LPCSTR name,LPCSTR mode){FILE *f=open_resource(name,mode);if(!f)fail("missing native rule resource");return f;}
FILE *UTIL_OpenFileAtPath(LPCSTR path,LPCSTR name){(void)path;return open_resource(name,"rb");}
FILE *UTIL_OpenFileAtPathForMode(LPCSTR path,LPCSTR name,LPCSTR mode){(void)path;return open_resource(name,mode);}
void UTIL_CloseFile(FILE *f){if(f)wb_fclose(f);}
void *UTIL_malloc(size_t size){void *p=malloc(size);if(!p)fail("native allocation failed");return p;}
void *UTIL_calloc(size_t count,size_t size){void *p=calloc(count,size);if(!p)fail("native allocation failed");return p;}
void TerminateOnError(const char *fmt,...){char message[512];va_list args;va_start(args,fmt);vsnprintf(message,sizeof message,fmt,args);va_end(args);fail(message);}
void UTIL_LogOutput(LOGLEVEL level,const char *fmt,...){
 if(level==LOGLEVEL_DEBUG&&!strncmp(fmt,"[SCRIPT]",8)){
  va_list args;va_start(args,fmt);current_pc=(unsigned)va_arg(args,int);va_end(args);
  instruction_count++;if(!budget--)fail("native script instruction limit");
 }
}
static char global_buffers[PAL_MAX_GLOBAL_BUFFERS][PAL_GLOBAL_BUFFER_SIZE];
char *UTIL_GlobalBuffer(int index){if(index<0||index>=PAL_MAX_GLOBAL_BUFFERS)fail("invalid native buffer index");return global_buffers[index];}
char *UTIL_va(char *buffer,int size,const char *fmt,...){va_list args;va_start(args,fmt);vsnprintf(buffer,size,fmt,args);va_end(args);return buffer;}

// Preserve the reference LCG and scaling; the seed is explicit and lives in
// saved linear memory instead of falling back to a host wall clock.
static int next_random(void){random_seed=1664525u*random_seed+1013904223u;return ((int32_t)random_seed>>1)+1073741824;}
int RandomLong(int from,int to){if(to<=from)return from;return from+next_random()/(INT_MAX/(to-from+1));}
float RandomFloat(float from,float to){if(to<=from)return from;return from+(float)next_random()/((float)INT_MAX/(to-from));}
Uint32 SDL_GetTicks(void){return clock_ms;}
Uint64 SDL_GetPerformanceCounter(void){return clock_ms;}
Uint64 SDL_GetPerformanceFrequency(void){return 1000;}
void UTIL_Delay(unsigned ms){wb_host_yield(1,(int)ms);clock_ms+=ms;last_frame_clock=clock_ms;PAL_ProcessEvent();if(dialog_waiting&&dialog_acknowledged)g_InputState.dwKeyPress|=kKeySearch;}
void SDL_Delay(Uint32 ms){
 clock_ms+=ms;
 unsigned frame=gpGlobals->fInBattle?BATTLE_FRAME_TIME:FRAME_TIME;
 if(rng_waiting?clock_ms>=rng_deadline:clock_ms-last_frame_clock>=frame){
  unsigned elapsed=clock_ms-last_frame_clock;last_frame_clock=clock_ms;
  wb_host_yield(4,(int)elapsed);
 }
}
void PAL_ClearKeyState(void){g_InputState.dwKeyPress=0;}
void PAL_ProcessEvent(void){g_InputState.prevdir=g_InputState.dir;g_InputState.dir=held_direction;g_InputState.dwKeyPress|=queued_keys;queued_keys=0;}
// A queued arrow edge owns one original UI/frame even after its physical
// release. Continuous direction remains separate, so batching cannot extend a
// short tap into a held movement or reorder it behind a newer held level.
PALDIRECTION wb_input_direction(void){
 DWORD keys=g_InputState.dwKeyPress&(kKeyUp|kKeyDown|kKeyLeft|kKeyRight);
 if(!keys)return g_InputState.dir;
 if((keys&(kKeyUp|kKeyDown))==(kKeyUp|kKeyDown))keys&=~(kKeyUp|kKeyDown);
 if((keys&(kKeyLeft|kKeyRight))==(kKeyLeft|kKeyRight))keys&=~(kKeyLeft|kKeyRight);
 if(keys&kKeyUp)return kDirNorth;
 if(keys&kKeyDown)return kDirSouth;
 if(keys&kKeyLeft)return kDirWest;
 if(keys&kKeyRight)return kDirEast;
 return kDirUnknown;
}
void wb_input(unsigned direction,unsigned keys){if(direction>kDirUnknown||keys&~0x3ffffu)fail("invalid native input");held_direction=(PALDIRECTION)direction;queued_keys|=keys;}
void wb_release(void){held_direction=kDirUnknown;queued_keys=0;g_InputState.dir=g_InputState.prevdir=kDirUnknown;g_InputState.dwKeyPress=0;}
void wb_budget(unsigned limit){budget=limit;}
WORD WB_OriginalRunTriggerScript(WORD entry,WORD object);
WORD PAL_RunTriggerScript(WORD entry,WORD object){trigger_depth++;WORD next=WB_OriginalRunTriggerScript(entry,object);trigger_depth--;return next;}

LPCWSTR PAL_GetWord(int i){if(i<0||i>=word_count)fail("invalid original word index");return words[i];}
LPCWSTR PAL_GetMsg(int i){if(i<0||i>=msg_count)fail("invalid original message index");current_message=i;return messages[i];}
CODEPAGE PAL_GetCodePage(void){return CP_GBK;}
// Original draw calls are projected for the client without bitmap payloads.
// The verification-only font build additionally paints the original C raster.
VOID WB_OriginalVIDEO_UpdateScreen(const SDL_Rect *rect);
void VIDEO_UpdateScreen(const SDL_Rect *rect){wb_raster_begin_display(rect!=NULL,g_wShakeTime!=0);WB_OriginalVIDEO_UpdateScreen(rect);}
int SDL_BlitSurface(SDL_Surface *src,const SDL_Rect *sr,SDL_Surface *dst,SDL_Rect *dr){
 if(!src||!dst||!src->pixels||!dst->pixels)return -1;
 if(!sr&&!dr&&src==gpScreen&&dst==gpScreenBak){memcpy(ui_backup,ui_ops,(1+ui_ops[0]*8)*sizeof(int));memcpy(scene_backup,scene_ops,(1+scene_ops[0]*8)*sizeof(int));scene_ready_backup=scene_ready;clear_color_backup=clear_color;backup_map=gpGlobals->g.rgScene[gpGlobals->wNumScene-1].wMapNum;backup_camera_x=PAL_X(gpGlobals->viewport);backup_camera_y=PAL_Y(gpGlobals->viewport);}
 if(!sr&&!dr&&src==gpScreenBak&&dst==gpScreen){memcpy(ui_ops,ui_backup,(1+ui_backup[0]*8)*sizeof(int));memcpy(scene_ops,scene_backup,(1+scene_backup[0]*8)*sizeof(int));scene_ready=scene_ready_backup;clear_color=clear_color_backup;}
 int sx=sr?sr->x:0,sy=sr?sr->y:0,dx=dr?dr->x:0,dy=dr?dr->y:0,w=sr?sr->w:src->w,h=sr?sr->h:src->h;
 if(sx<0){w+=sx;dx-=sx;sx=0;}if(sy<0){h+=sy;dy-=sy;sy=0;}
 if(dx<0){w+=dx;sx-=dx;dx=0;}if(dy<0){h+=dy;sy-=dy;dy=0;}
 if(w>src->w-sx)w=src->w-sx;if(w>dst->w-dx)w=dst->w-dx;if(h>src->h-sy)h=src->h-sy;if(h>dst->h-dy)h=dst->h-dy;
 if(w<0)w=0;if(h<0)h=0;
 wb_raster_copy(src,dst,sx,sy,dx,dy,w,h);
 if(dr){dr->w=w;dr->h=h;}
 for(int i=0;i<h;i++){int row=src==dst&&dy>sy?h-1-i:i;memmove((unsigned char*)dst->pixels+(dy+row)*dst->pitch+dx,(unsigned char*)src->pixels+(sy+row)*src->pitch+sx,w);}
 return 0;
}
int SDL_FillRect(SDL_Surface *dst,const SDL_Rect *rect,Uint32 color){
 if(!dst||!dst->pixels)return -1;int x=rect?rect->x:0,y=rect?rect->y:0,right=rect?rect->x+rect->w:dst->w,bottom=rect?rect->y+rect->h:dst->h;
 if(dst==gpScreen&&!rect&&!wb_raster_active()){scene_ready=0;clear_color=(unsigned char)color;scene_ops[0]=0;ui_ops[0]=0;}
 if(x<0)x=0;if(y<0)y=0;if(right>dst->w)right=dst->w;if(bottom>dst->h)bottom=dst->h;
 if(right<=x||bottom<=y)return 0;wb_raster_fill(dst,x,y,right-x,bottom-y,(unsigned char)color);for(int row=y;row<bottom;row++)memset((unsigned char*)dst->pixels+row*dst->pitch+x,(unsigned char)color,right-x);return 0;
}
int SDL_SoftStretch(SDL_Surface *src,const SDL_Rect *sr,SDL_Surface *dst,const SDL_Rect *dr){
 if(!src||!dst||!src->pixels||!dst->pixels)return -1;SDL_Rect from=sr?*sr:(SDL_Rect){0,0,src->w,src->h},to=dr?*dr:(SDL_Rect){0,0,dst->w,dst->h};
 if(from.w<=0||from.h<=0||to.w<=0||to.h<=0)return 0;
 wb_raster_stretch(src,dst,from.x,from.y,from.w,from.h,to.x,to.y,to.w,to.h);
 for(int y=0;y<to.h;y++)for(int x=0;x<to.w;x++){int sx=from.x+x*from.w/to.w,sy=from.y+y*from.h/to.h,dx=to.x+x,dy=to.y+y;if(sx>=0&&sy>=0&&sx<src->w&&sy<src->h&&dx>=0&&dy>=0&&dx<dst->w&&dy<dst->h)((unsigned char*)dst->pixels)[dy*dst->pitch+dx]=((unsigned char*)src->pixels)[sy*src->pitch+sx];}return 0;
}
int SDL_LockSurface(SDL_Surface *surface){return surface?0:-1;}
void SDL_UnlockSurface(SDL_Surface *surface){(void)surface;}
SDL_Surface *VIDEO_DuplicateSurface(SDL_Surface *source,const SDL_Rect *rect){
 if(!source)return NULL;int w=rect?rect->w:source->w,h=rect?rect->h:source->h;
 if(w<=0||h<=0||w>4096||h>4096)fail("invalid native UI surface");
 SDL_Surface *surface=UTIL_malloc(sizeof *surface);*surface=(SDL_Surface){w,h,w,UTIL_calloc(w,h),&format};
 if(SDL_BlitSurface(source,rect,surface,NULL)!=0)fail("native UI surface copy failed");return surface;
}
SDL_Surface *VIDEO_CreateCompatibleSizedSurface(SDL_Surface *source,const SDL_Rect *rect){
 if(!source)return NULL;int w=rect?rect->w:source->w,h=rect?rect->h:source->h;
 if(w<=0||h<=0||w>4096||h>4096)fail("invalid compatible native surface");
 SDL_Surface *surface=UTIL_malloc(sizeof *surface);*surface=(SDL_Surface){w,h,w,UTIL_calloc(w,h),&format};return surface;
}
SDL_Surface *VIDEO_CreateCompatibleSurface(SDL_Surface *source){return VIDEO_CreateCompatibleSizedSurface(source,NULL);}
void SDL_FreeSurface(SDL_Surface *surface){if(surface){wb_raster_surface_free(surface);free(surface->pixels);free(surface);}}
void VIDEO_SetPalette(SDL_Color colorsIn[256]){memcpy(colors,colorsIn,sizeof colors);}
SDL_Color *VIDEO_GetPalette(void){return colors;}
unsigned wb_audio_clock(void){return clock_ms;}
INT PAL_OpeningMenu(void){return 0;}
static INT metadata_decompress(LPCVOID source,LPVOID output,INT capacity){
 const unsigned char *data=source;
 if(data[4]=='W'&&data[5]=='B'&&data[6]=='Z'&&data[7]=='1'){
  uint32_t length=(uint32_t)data[0]|(uint32_t)data[1]<<8|(uint32_t)data[2]<<16|(uint32_t)data[3]<<24;
  uint32_t size=(uint32_t)data[8]|(uint32_t)data[9]<<8|(uint32_t)data[10]<<16|(uint32_t)data[11]<<24;
  if(length>(uint32_t)capacity)fail("native rule metadata exceeds destination");
  if(wb_host_inflate(data+12,size,output,capacity)!=length)fail("native rule metadata did not decode");
  return (INT)length;
 }
 return YJ2_Decompress(source,output,capacity);
}

int wb_boot(unsigned seed){
 if(booted||!words||!messages)return 0;
 gpGlobals->f.fpSSS=UTIL_OpenRequiredFile("sss.mkf");gpGlobals->f.fpDATA=UTIL_OpenRequiredFile("data.mkf");
 gpGlobals->f.fpMGO=UTIL_OpenRequiredFile("mgo.mkf");gpGlobals->f.fpFBP=UTIL_OpenRequiredFile("fbp.mkf");
 gpGlobals->f.fpBALL=UTIL_OpenRequiredFile("ball.mkf");gpGlobals->f.fpF=UTIL_OpenRequiredFile("f.mkf");
 gpGlobals->f.fpFIRE=UTIL_OpenRequiredFile("fire.mkf");gpGlobals->f.fpRGM=UTIL_OpenRequiredFile("rgm.mkf");
 gConfig.fIsWIN95=TRUE;gConfig.pszMsgFile=NULL;
 wb_init_screen_layout();
 Decompress=metadata_decompress;random_seed=1664525u*seed+1013904223u;budget=1000000;
 PAL_InitResources();g_InputState.dir=g_InputState.prevdir=kDirUnknown;
 PAL_InitGameData(0);
#ifdef WB_REFERENCE_FONT
 WB_InitReferenceFont();
#endif
 if(PAL_InitUI()!=0)fail("original UI initialization failed");
 g_TextLib.bCurrentFontColor=0x4f;g_TextLib.iDelayTime=3;g_TextLib.posDialogTitle=PAL_XY(12,8);g_TextLib.posDialogText=PAL_XY(44,26);g_TextLib.bDialogPosition=kDialogUpper;
 if(PAL_MKFReadChunk(g_TextLib.bufDialogIcons,282,12,gpGlobals->f.fpDATA)<=0)fail("original dialogue icons missing");
 gpGlobals->partyoffset=PAL_XY(160,112);gpGlobals->fInMainGame=TRUE;
 booted=1;return 1;
}
void wb_run_opening(void){
 if(!booted)fail("native rules not initialized");
 if(gpGlobals->wNumScene!=1)fail("opening harness entered the wrong original scene");
 gpGlobals->fEnteringScene=FALSE;
 gpGlobals->g.rgScene[0].wScriptOnEnter=PAL_RunTriggerScript(gpGlobals->g.rgScene[0].wScriptOnEnter,0xFFFF);
 wb_host_yield(3,gpGlobals->wNumScene);
}
void wb_run_campaign(void){
 if(!booted)fail("native rules not initialized");
#ifdef WB_ENDING_FIXTURE
 PAL_EndingScreen();
#ifdef WB_ENDING_CREDITS_FIXTURE
 extern VOID PAL_AdditionalCredits(VOID);
 PAL_AdditionalCredits();
#endif
 PAL_Shutdown(0);
#elif defined(WB_GAME_OVER_FIXTURE)
 PAL_DrawOpeningMenuBackground();PAL_SetPalette(0,FALSE);
 PAL_DrawText(L"SDLPAL",PAL_XY(64,32),0x4F,TRUE,FALSE,FALSE);
 PAL_RunTriggerScript(41075,0xFFFF);
 fail("original failure fixture returned without recovery boundary");
#elif defined(WB_RNG_FIXTURE_CLIP)
 PAL_SetPalette(gpGlobals->wNumPalette,FALSE);gpGlobals->fNeedToFadeIn=FALSE;
 SDL_FillRect(gpScreen,NULL,255);
 const int stops[]={112,162,186,195,266,-1};int first=0;
 for(int i=0;i<6;i++){PAL_RNGPlay(WB_RNG_FIXTURE_CLIP,first,stops[i],16);first=stops[i]+1;}
 wb_host_yield(3,WB_RNG_FIXTURE_CLIP);
#elif defined(WB_BATTLE_FIXTURE_TEAM)
 // Verification build only: original roles, unmodified resource stats and
 // original PAL_StartBattle. No story progress is claimed by this fixture.
 if(WB_BATTLE_FIXTURE_TEAM>=gpGlobals->g.nEnemyTeam||WB_BATTLE_FIXTURE_FIELD>=gpGlobals->g.nBattleField)fail("battle fixture exceeds original tables");
 gpGlobals->wNumBattleField=WB_BATTLE_FIXTURE_FIELD;
 gpGlobals->wMaxPartyMemberIndex=WB_BATTLE_FIXTURE_PARTY-1;
 const unsigned roles[]={WB_BATTLE_FIXTURE_ROLES};
 for(int i=0;i<WB_BATTLE_FIXTURE_PARTY;i++)gpGlobals->rgParty[i].wPlayerRole=roles[i];
 // A campaign has already selected its palette before entering combat. The
 // isolated verification entry must initialize that same presentation state.
 PAL_SetPalette(gpGlobals->wNumPalette,gpGlobals->fNightPalette);
 int result=PAL_StartBattle(WB_BATTLE_FIXTURE_TEAM,FALSE);wb_host_yield(3,result);
#else
 PAL_GameMain();fail("original campaign returned unexpectedly");
#endif
}
// Explicit verification entry points use the actual original UI functions and
// original resource messages. They are never a campaign or a platform action.
void wb_probe_portrait_reuse(unsigned mode){
 if(mode>2)fail("invalid portrait reuse probe");
 unsigned char *buffer=malloc(70000);if(!buffer)fail("portrait probe allocation failed");
 unsigned char *portrait=buffer+512;
 FILE *file=mode==2?gpGlobals->f.fpBALL:gpGlobals->f.fpRGM;unsigned chunk=mode==2?20:4;
 if(PAL_MKFReadChunk(portrait,65536,chunk,file)<=0)fail("portrait probe first read failed");
 if(PAL_MKFDecompressChunk(buffer,64000,24,gpGlobals->f.fpFBP)!=64000)fail("portrait probe FBP read failed");
 if(PAL_MKFReadChunk(portrait,65536,chunk,file)<=0)fail("portrait probe reload failed");
 LPCBITMAPRLE bitmap=portrait;
 if(mode==1){if(PAL_MKFDecompressChunk(portrait,65536,2,gpGlobals->f.fpMGO)<=0)fail("portrait probe actor read failed");bitmap=PAL_SpriteGetFrame(portrait,0);}
 PAL_RLEBlitToSurface(bitmap,gpScreen,PAL_XY(9,10));wb_host_yield(3,0);
}
void wb_probe_dialogue(unsigned mode){
 if(mode>2)fail("invalid original dialogue probe");
 if(mode==0){PAL_StartDialog(kDialogUpper,0,0,FALSE);g_TextLib.iDelayTime=0;int ids[]={1318,1319,1321,1323,1318};for(int i=0;i<5;i++)PAL_ShowDialogText(PAL_GetMsg(ids[i]));}
 else if(mode==1){PAL_StartDialog(kDialogLower,0,8,FALSE);g_TextLib.iDelayTime=0;PAL_ShowDialogText(PAL_GetMsg(1320));PAL_ShowDialogText(PAL_GetMsg(1318));}
 else {PAL_StartDialog(kDialogCenter,0,0,FALSE);PAL_ShowDialogText(PAL_GetMsg(0));}
 PAL_EndDialog();wb_host_yield(3,mode);
}
void wb_probe_menu(void){MENUITEM items[]={{11,3,TRUE,PAL_XY(44,26)},{22,4,TRUE,PAL_XY(44,44)}};WORD selected=PAL_ReadMenu(NULL,items,2,0,0x4f);wb_host_yield(3,selected);}
unsigned wb_view(void){
 memset(view,0,sizeof view);view[0]=1;view[1]=gpGlobals->wNumScene;
 view[2]=PAL_X(gpGlobals->viewport)+gpGlobals->rgParty[0].x;view[3]=PAL_Y(gpGlobals->viewport)+gpGlobals->rgParty[0].y;
 view[4]=gpGlobals->wMaxPartyMemberIndex+1;
 for(int i=0;i<3;i++)view[5+i]=gpGlobals->rgParty[i].wPlayerRole;
 unsigned role=gpGlobals->rgParty[0].wPlayerRole;
 view[8]=gpGlobals->g.PlayerRoles.rgwSpriteNum[role];view[9]=gpGlobals->g.PlayerRoles.rgwHP[role];view[10]=gpGlobals->g.PlayerRoles.rgwMP[role];
 view[11]=gpGlobals->dwCash;view[12]=random_seed;view[13]=clock_ms;view[14]=current_pc;view[15]=message_count;
 view[16]=(unsigned)(current_message+1);view[17]=gpGlobals->fInBattle?2:menu_depth>0?3:trigger_depth>0?1:0;view[18]=gpGlobals->fEnteringScene;
 view[19]=gpGlobals->fInBattle?g_Battle.UI.wCurPlayerIndex:0;
 view[20]=held_direction;view[21]=queued_keys;view[22]=g_InputState.dwKeyPress;
 view[23]=(uint32_t)(int32_t)PAL_X(gpGlobals->viewport);view[24]=(uint32_t)(int32_t)PAL_Y(gpGlobals->viewport);
 view[25]=gpGlobals->g.rgScene[gpGlobals->wNumScene-1].wMapNum;view[26]=gpGlobals->wNumPalette;view[27]=gpGlobals->fNightPalette;
 view[28]=gpGlobals->dwFrameNum;view[29]=gpGlobals->wLayer;view[30]=current_face;view[31]=g_TextLib.bDialogPosition;
 return (unsigned)view;
}
