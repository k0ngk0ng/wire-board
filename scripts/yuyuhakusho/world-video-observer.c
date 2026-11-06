/* Research-only passive snapshots. The original core executes all operations. */
#include <stddef.h>
#include <stdint.h>
#include <string.h>
#include "z80.h"

typedef unsigned (*get_reg_fn)(unsigned);
typedef int (*instruction_cycles_fn)(void);
typedef struct {
 uint32_t kind,frame,pc,address,width,value;
 int32_t cpu_cycle,instruction_cycles;
 uint32_t line_cycle,vcounter,hcounter,reg0,reg1,reg10,reg12;
 uint32_t hint_pending,vint_pending,status_irq,z80_irq,sr;
 uint32_t phase_word,world_flags,palette_flags;
 uint32_t dma_end,dma_length,dma_mode;
} world_video_event;
static world_video_event events[8192],last;
static size_t count;
static unsigned overflow,active,current_frame,last_pc;
static const uint8_t *regs,*hc,*hp,*vp,*ram,*dm;
static const uint16_t *vc,*status;
static const uint32_t *line_cycle,*dma_end,*dma_length;
static const int32_t *cpu_cycle;
static const Z80_Regs *zcpu;
static get_reg_fn get_reg;
static instruction_cycles_fn instruction_cycles;

static world_video_event snapshot(void){
 world_video_event e={0};
 e.frame=current_frame;e.cpu_cycle=*cpu_cycle;e.instruction_cycles=instruction_cycles();
 e.line_cycle=*line_cycle;e.vcounter=*vc;e.hcounter=*hc;
 e.reg0=regs[0];e.reg1=regs[1];e.reg10=regs[10];e.reg12=regs[12];
 e.hint_pending=*hp;e.vint_pending=*vp;e.status_irq=*status&0x88;
 e.z80_irq=zcpu->irq_state;e.sr=get_reg(17);
 e.phase_word=*(const uint16_t*)(ram+0xf516);
 e.world_flags=ram[0xf50e^1];e.palette_flags=ram[0xf500^1];
 e.dma_end=*dma_end;e.dma_length=*dma_length;e.dma_mode=*dm;
 return e;
}
static int changed(const world_video_event *e){
 return e->line_cycle!=last.line_cycle||e->vcounter!=last.vcounter||e->hcounter!=last.hcounter||
 e->reg0!=last.reg0||e->reg1!=last.reg1||e->reg10!=last.reg10||e->reg12!=last.reg12||
 e->hint_pending!=last.hint_pending||e->vint_pending!=last.vint_pending||e->status_irq!=last.status_irq||
 e->phase_word!=last.phase_word||e->world_flags!=last.world_flags||e->palette_flags!=last.palette_flags;
}
static void append(world_video_event e,unsigned kind,unsigned pc,unsigned address,unsigned width,unsigned value){
 if(count==8192){overflow=1;return;}
 e.kind=kind;e.pc=pc;e.address=address;e.width=width;e.value=value;
 events[count++]=e;last=e;
}
int world_video_attach(void *r,void *h,void *v,void *ih,void *iv,void *st,void *lc,void *cc,void *wr,void *z,void *gr,void *ic,void *de,void *dl,void *dt){
 if(!r||!h||!v||!ih||!iv||!st||!lc||!cc||!wr||!z||!gr||!ic||!de||!dl||!dt)return 0;
 regs=r;hc=h;vc=v;hp=ih;vp=iv;status=st;line_cycle=lc;cpu_cycle=cc;ram=wr;zcpu=z;
 get_reg=(get_reg_fn)gr;instruction_cycles=(instruction_cycles_fn)ic;dma_end=de;dma_length=dl;dm=dt;
 return sizeof(Z80_Regs)==88;
}
void world_video_begin(unsigned frame){
 current_frame=frame;count=overflow=last_pc=0;active=1;
 append(snapshot(),0,get_reg(16),0,0,0);
}
void world_video_end(void){append(snapshot(),9,get_reg(16),0,0,0);active=0;}
void world_video_hook(int type,int width,unsigned address,unsigned value){
 if(!active)return;
 unsigned p=address&0xffffff;
 if(type==1){
  last_pc=p;
  world_video_event e=snapshot();
  if(p==0xfff000||p==0x2e5e){append(e,4,p,0,0,0);return;}
  if(p==0xfff09a||p==0xfff0a8||p==0xfff13a||p==0x2e9e){append(e,5,p,0,0,0);return;}
  if(changed(&e))append(e,1,p,0,0,0);
 }else if(type==4&&p>=0xc00000&&p<0xe00000&&(p&0x1c)==4){
  append(snapshot(),2,last_pc,p,(unsigned)width,value);
 }else if(type==2&&p>=0xc00000&&p<0xe00000&&(p&0x1c)==4){
  append(snapshot(),3,last_pc,p,(unsigned)width,value);
 }else if(type==2&&p>=0xc00000&&p<0xe00000&&(p&0x1c)==8){
  append(snapshot(),6,last_pc,p,(unsigned)width,value);
 }else if(type==1024&&(p&0xfffc)==0x7f04){
  append(snapshot(),8,last_pc,p,(unsigned)width,value);
 }
}
size_t world_video_count(void){return count;}
unsigned world_video_overflow(void){return overflow;}
world_video_event *world_video_events(void){return events;}
unsigned world_video_size(void){return sizeof(world_video_event);}
