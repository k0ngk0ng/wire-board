/* Actual 32d6 wrapper through logical-World entry; passive natural capture.
 * Private recipes explicitly modify only declared inputs at wrapper entry and
 * run identically with observation off/on. No original instruction is changed. */
#include "battle-world-observer.c"
typedef struct { world_event_t base; uint32_t dma_remaining,dma_end,dma_type,vdp_status,transfer_end,transfer_flag,frame_index; } dma_event_t;
static dma_event_t dma_events[8192];
static const uint32_t *dma_remaining_ptr,*dma_end_ptr;
static const uint8_t *dma_type_ptr;
static const uint16_t *status_ptr;
static uint16_t transfers[2048];
static unsigned transfer_words,private_recipe,private_busy,private_pause,private_empty,observing=1;
static unsigned current_frame;
void battle_world_dma_frame(unsigned frame){current_frame=frame;}
int battle_world_dma_attach(void*remaining,void*end,void*type,void*status){if(!remaining||!end||!type||!status)return 0;dma_remaining_ptr=remaining;dma_end_ptr=end;dma_type_ptr=type;status_ptr=status;return 1;}
static void dma_put(unsigned kind,unsigned pc,unsigned width,unsigned address,unsigned value){if(!observing)return;unsigned before=count;world_put(kind,pc,width,address,value);if(count!=before)dma_events[before]=(dma_event_t){world_events[before],*dma_remaining_ptr,*dma_end_ptr,*dma_type_ptr,*status_ptr,word(0xf518),ram[0xf54e^1],current_frame};}
void battle_world_dma_begin(unsigned recipe,unsigned busy,unsigned pause,unsigned empty,unsigned observe){battle_prefix_begin();private_recipe=recipe;private_busy=busy;private_pause=pause;private_empty=empty;observing=observe;transfer_words=0;}
void battle_world_dma_hook(int type,int width,unsigned address,unsigned value){unsigned p=address&0xffffff;if(type==1){last_pc=p;if(!active&&!done&&p==0x32d6){
 if(private_recipe){uint8_t*w=(uint8_t*)ram;w[0xf50e^1]=(w[0xf50e^1]&~0xc0)|(private_busy?0x80:0)|(private_empty?0x40:0);w[0xf807^1]=(uint8_t)private_pause;if(private_empty)*(uint16_t*)(w+0xf518)=0xd400;}
 unsigned end=word(0xf518);if(end<0xd400||end>0xe400||(end-0xd400)%16){overflow=1;return;}transfer_words=(end-0xd400)/2;for(unsigned i=0;i<transfer_words;i++)transfers[i]=(uint16_t)word(0xd400+i*2);active=1;dma_put(9,p,0,0,0);
 }if(active){if(p==0x3a2e||p==0xfff136){dma_put(10,p,0,0,0);active=0;done=1;}else dma_put(1,p,0,0,0);}}
 else if(active&&(type==2||type==4)){if((p>=0xc00000&&p<0xe00000)||(p&0xffff00)==0xa11100||p==0xa01fff)dma_put((unsigned)type,last_pc,(unsigned)width,p,value);}}
const dma_event_t*battle_world_dma_events(void){return dma_events;}
unsigned battle_world_dma_size(void){return sizeof(dma_event_t);}
const uint16_t*battle_world_dma_transfers(void){return transfers;}
unsigned battle_world_dma_transfer_words(void){return transfer_words;}
