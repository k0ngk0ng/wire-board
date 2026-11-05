/* Research-only passive delegates around the pinned reference core handlers. */
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "z80.h"

typedef void (*fm_fn)(unsigned,unsigned,unsigned);
typedef unsigned char (*read_fn)(unsigned);
typedef void (*write_fn)(unsigned,unsigned char);
typedef struct {
 uint32_t kind,frame,cycles_before,cycles_after,m68k_before,m68k_after,z80_before,z80_after,address,data,bank,pc;
 uint32_t registers[13];
 uint8_t state[128];
} pcm_event;
static fm_fn saved_fm;static read_fn saved_read;static write_fn saved_write;
static fm_fn *fm_slot;static read_fn *read_slot;static write_fn *write_slot;
static Z80_Regs *cpu;static int32_t *mcycles;static uint32_t *bank;static uint8_t *ram;
static pcm_event *events;static size_t count,capacity;static unsigned overflow,frame;
static uint8_t latch[2];
static void snapshot(pcm_event *e){
 e->frame=frame;e->pc=cpu->pc.w.l;e->bank=*bank;
 PAIR *r[]={&cpu->pc,&cpu->sp,&cpu->af,&cpu->bc,&cpu->de,&cpu->hl,&cpu->ix,&cpu->iy,&cpu->af2,&cpu->bc2,&cpu->de2,&cpu->hl2,&cpu->wz};
 for(int i=0;i<13;i++)e->registers[i]=r[i]->d;
 memcpy(e->state,ram+0x1f80,128);
}
static pcm_event *next(unsigned kind,unsigned address,unsigned data){
 if(count>=capacity){overflow++;return NULL;}pcm_event *e=&events[count++];memset(e,0,sizeof(*e));e->kind=kind;e->address=address;e->data=data;snapshot(e);return e;
}
static void observed_fm(unsigned cycles,unsigned address,unsigned data){
 unsigned a=address&3,p=a>>1;
 if(!(a&1))latch[p]=(uint8_t)data;
 else if((p==0&&(latch[0]==0x2a||latch[0]==0x2b))||(p==1&&latch[1]==0xb6)){
  pcm_event *e=next(p==0&&latch[0]==0x2a?1:2,(p<<8)|latch[p],data);
  if(e){e->cycles_before=cycles;e->cycles_after=cycles;e->m68k_before=e->m68k_after=*mcycles;e->z80_before=e->z80_after=cpu->cycles;}
 }
 saved_fm(cycles,address,data);
}
static unsigned char observed_read(unsigned address){
 unsigned pc=cpu->pc.w.l;unsigned zbefore=cpu->cycles,mbefore=*mcycles;unsigned char data=saved_read(address);
 unsigned kind=address>=0x8000?3:(address>=0xc00&&address<0xe00?4:(address>=0xa00&&address<0xc00?5:(address>=0x800&&address<0xa00?6:0)));
 if(kind){pcm_event *e=next(kind,address,data);if(e){e->pc=pc;e->cycles_before=zbefore;e->cycles_after=cpu->cycles;e->m68k_before=mbefore;e->m68k_after=*mcycles;e->z80_before=zbefore;e->z80_after=cpu->cycles;}}
 return data;
}
static void observed_write(unsigned address,unsigned char data){
 saved_write(address,data);
 if(address==0x1fc1||address==0x1fc5||address==0x1f86||address==0x1fa6||address==0x1ffc||address==0x1ffe){
  pcm_event *e=next(7,address,data);if(e){e->cycles_before=e->cycles_after=cpu->cycles;e->m68k_before=e->m68k_after=*mcycles;e->z80_before=e->z80_after=cpu->cycles;}
 }
}
unsigned pcm_model_event_size(void){return sizeof(pcm_event);}
unsigned pcm_model_z80_size(void){return sizeof(Z80_Regs);}
int pcm_model_attach(void *fm,void *rd,void *wr,void *z,void *mc,void *zb,void *zr,size_t max){
 if(saved_fm||!fm||!rd||!wr||!z||!mc||!zb||!zr||!max)return 0;
 fm_slot=(fm_fn*)fm;read_slot=(read_fn*)rd;write_slot=(write_fn*)wr;
 saved_fm=*fm_slot;saved_read=*read_slot;saved_write=*write_slot;
 if(saved_fm==observed_fm||saved_read==observed_read||saved_write==observed_write)return 0;
 cpu=(Z80_Regs*)z;mcycles=(int32_t*)mc;bank=(uint32_t*)zb;ram=(uint8_t*)zr;
 events=(pcm_event*)calloc(max,sizeof(pcm_event));if(!events)return 0;capacity=max;count=overflow=frame=0;latch[0]=latch[1]=0xff;
 *fm_slot=observed_fm;*read_slot=observed_read;*write_slot=observed_write;return 1;
}
int pcm_model_detach(void){
 if(!saved_fm)return 0;
 int intact=*fm_slot==observed_fm&&*read_slot==observed_read&&*write_slot==observed_write;
 *fm_slot=saved_fm;*read_slot=saved_read;*write_slot=saved_write;saved_fm=NULL;saved_read=NULL;saved_write=NULL;return intact;
}
void pcm_model_frame(unsigned f){frame=f;}
const pcm_event *pcm_model_events(void){return events;}
size_t pcm_model_count(void){return count;}
unsigned pcm_model_overflow(void){return overflow;}
void pcm_model_free(void){free(events);events=NULL;capacity=count=0;}
