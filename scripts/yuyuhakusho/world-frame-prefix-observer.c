/* Research-only, passive actual logical frame prelude and its nested IRQs. */
#include "battle-world-dma-observer.c"
static uint8_t frame_snapshots[32][2][65536];
static unsigned frame_reserved_carry[32],frame_start[32],frame_end[32],frame_begin_index[32],frame_end_index[32],frame_cases,selected_frames[32],selected_count;
typedef unsigned char(*frame_read_fn)(unsigned);
static frame_read_fn frame_saved_read,*frame_read_slot;
static unsigned char frame_observed_read(unsigned address){unsigned before=(unsigned)*cycle;unsigned char value=frame_saved_read(address);if(active&&address>=0x8000)dma_put(20,last_pc,0,*batch,(unsigned)*cycle-before);return value;}
int world_frame_prefix_attach_read(void*slot){if(!slot||frame_saved_read)return 0;frame_read_slot=slot;frame_saved_read=*frame_read_slot;*frame_read_slot=frame_observed_read;return 1;}
int world_frame_prefix_detach_read(void){if(!frame_saved_read||*frame_read_slot!=frame_observed_read)return 0;*frame_read_slot=frame_saved_read;frame_saved_read=0;return 1;}
static void frame_snapshot(unsigned which){for(unsigned i=0;i<65536;i++)frame_snapshots[frame_cases][which][i]=ram[i^1];}
void world_frame_prefix_begin(void){battle_prefix_begin();observing=1;frame_cases=selected_count=0;}
void world_frame_prefix_select(unsigned frame){if(selected_count<32)selected_frames[selected_count++]=frame;}
static unsigned frame_selected(void){for(unsigned i=0;i<selected_count;i++)if(selected_frames[i]==current_frame)return 1;return 0;}
void world_frame_prefix_hook(int type,int width,unsigned address,unsigned value){unsigned p=address&0xffffff;if(type==1){last_pc=p;if(!active&&p==0x3a2e&&frame_selected()){if(frame_cases==32){overflow=1;return;}frame_snapshot(0);frame_reserved_carry[frame_cases]=(get(1)>>8)&15;frame_start[frame_cases]=current_frame;frame_begin_index[frame_cases]=count;active=1;dma_put(9,p,0,0,0);}if(active){if(p==0x3a5a||p==0x200){frame_snapshot(1);dma_put(10,p,0,0,0);frame_end[frame_cases]=current_frame;frame_end_index[frame_cases]=count;frame_cases++;active=0;}else dma_put(1,p,0,0,0);}}else if(active&&(type==2||type==4)){if((p>=0xc00000&&p<0xe00000)||(p&0xffff00)==0xa11100||p==0xa01fff)dma_put((unsigned)type,last_pc,(unsigned)width,p,value);}}
const uint8_t*world_frame_prefix_snapshot(unsigned index,unsigned which){return frame_snapshots[index][which&1];}
unsigned world_frame_prefix_carry(unsigned index){return frame_reserved_carry[index];}
unsigned world_frame_prefix_cases(void){return frame_cases;}
unsigned world_frame_prefix_active(void){return active;}
unsigned world_frame_prefix_first(unsigned index){return frame_begin_index[index];}
unsigned world_frame_prefix_last(unsigned index){return frame_end_index[index];}
unsigned world_frame_prefix_frame(unsigned index){return frame_start[index];}
unsigned world_frame_prefix_end_frame(unsigned index){return frame_end[index];}
