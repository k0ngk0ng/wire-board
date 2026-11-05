/* Passive research recorder. No instruction execution, ROM interpretation or
 * production linkage. The existing pinned reference CPU performs all work. */
#include <stdint.h>
#include <stddef.h>

typedef struct {
  uint32_t kind,pc,address,width,value,frame;
  int32_t cycles,refresh;
  uint32_t callback_id,callback_behavior,irq_depth,arena;
  uint32_t b0,b1,b2,b3,b4,b5,b6;
  uint32_t a0,a1,a2,a3;
} event_t;
static event_t events[65536];static size_t count;static unsigned overflow;
static const unsigned char *ram;static const uint32_t *dar;
static int32_t *cycles;static const int32_t *refresh;
static unsigned stop_pc;
static uint32_t frame,last_pc,callback_id,callback_behavior,irq_depth;
static unsigned arena,arena_depth,arena_sp,arena_return;
static unsigned block_refresh,block_irq,block_bus;
static unsigned active;
static unsigned byte_at(unsigned a){return ram[(a&65535)^1];}
static unsigned word_at(unsigned a){return(byte_at(a)<<8)|byte_at(a+1);}
static unsigned long_at(unsigned a){return(word_at(a)<<16)|word_at(a+2);}
static void put(unsigned k,unsigned p,unsigned a,unsigned w,unsigned v,unsigned x,unsigned y,unsigned z,unsigned t){
 if(count==65536){overflow=1;return;}
 events[count++]=(event_t){k,p,a,w,v,frame,*cycles,*refresh,callback_id,callback_behavior,irq_depth,arena,
  byte_at(0xf800),byte_at(0xf807),byte_at(0xf809),byte_at(0xf80a),byte_at(0xf80b),byte_at(0xf80c),byte_at(0xf80d),x,y,z,t};
}
void phase_memory(const unsigned char*r,const uint32_t*d,int32_t*c,const int32_t*f){ram=r;dar=d;cycles=c;refresh=f;}
void phase_begin(unsigned f){count=overflow=0;frame=f;active=1;}
void phase_stop_at(unsigned pc){stop_pc=pc;}
void phase_reset(void){arena=arena_depth=arena_sp=arena_return=irq_depth=callback_id=callback_behavior=0;}
size_t phase_count(void){return count;}
unsigned phase_overflow(void){return overflow;}
event_t *phase_events(void){return events;}
static int sound_address(unsigned a){a&=65535;return a==0xf800||a==0xf807||(a>=0xf809&&a<=0xf80d);}
static int watch(unsigned address,unsigned width){for(unsigned i=0;i<width;i++)if(sound_address(address+i))return 1;return 0;}
static int marker(unsigned p){switch(p){
 case 0x21960:case 0x21970:case 0x21982:case 0x21994:case 0x219a6:case 0x219aa:
 case 0x219ac:case 0x219cc:case 0x219d6:case 0x219de:case 0x219e4:case 0x219e8:
 case 0x219ea:case 0x219f6:case 0x219fc:case 0x21a00:case 0x21a04:case 0x21a06:
 case 0x375a6:case 0x375ae:case 0x375d0:case 0x375d4:case 0x37c44:case 0x37c8e:
 case 0x37cb2:case 0x37cb4:case 0x37b90:case 0x37bd8:case 0x37c08:case 0x37c0c:
 case 0x38f28:case 0x390f6:case 0x38808:case 0x38a2c:case 0x38a72:case 0x38a7a:
 case 0x38a: return 1;default:return 0;}}
static int block(unsigned p){switch(p){
 case 0x2092c:case 0x2094c:case 0x20950:case 0x20982:case 0x209a4:
 case 0x209da:case 0x209f8:case 0x20a12:case 0x20a30:case 0x20a62:
 case 0x20a94:case 0x20ab4:case 0x20ade:case 0x21556:case 0x215f8:
 case 0x21690:case 0x216ee:return 1;default:return 0;}}
void phase_cpu_hook(int type,int width,unsigned address,unsigned value){
 if(!active)return;
 unsigned p=address&0xffffff;
 if(type==1){
  last_pc=address;
  if(p==0x35a2){callback_id=dar[13]&65535;callback_behavior=long_at(callback_id);put(10,address,0,0,0,dar[15],0,0,0);}
  else if(p==0x35a4){put(11,address,0,0,0,dar[15],0,0,0);callback_id=callback_behavior=0;}
  if(p==0xfff000||p==0x2e5e){++irq_depth;if(arena)++block_irq;put(3,address,0,0,0,dar[15],long_at(dar[15]+2),word_at(dar[15]),0);}
  if(p==0xfff09a||p==0xfff0a8||p==0xfff13a||p==0x2e9e){put(4,address,0,0,0,dar[15],0,0,0);if(irq_depth)--irq_depth;}
  if(arena&&p==arena_return&&dar[15]==arena_sp+4){put(7,address,0,0,0,block_refresh,block_irq,block_bus,0);arena=0;}
  if(p==0x20918){arena=1;arena_depth=irq_depth;arena_sp=dar[15];arena_return=long_at(arena_sp)&0xffffff;block_refresh=block_irq=block_bus=0;
   put(5,address,0,0,0,word_at((dar[13]&65535)+6),word_at(0xa00e),word_at(0xf572),byte_at(0xf550)|(byte_at(0xf55a)<<8));}
  if(arena&&irq_depth==arena_depth&&block(p)){put(6,address,0,0,0,block_refresh,block_irq,block_bus,0);block_refresh=block_irq=block_bus=0;}
  if(arena&&irq_depth==arena_depth&&*cycles>=*refresh)++block_refresh;
  if(stop_pc&&p==stop_pc){put(12,address,0,0,0,dar[15],0,0,0);*cycles=4000000;}
  if(marker(p)){
   if(p!=0x219ea||byte_at(0xf563))put(1,address,0,0,0,dar[15],dar[0],dar[13],dar[12]);
  }
 }else{
  if(arena&&p>=0xa00000&&p<0xe00000)++block_bus;
  if(type==4&&p>=0xe00000&&watch(p,(unsigned)width))put(2,last_pc,address,(unsigned)width,value,0,0,0,0);
  if((type==2||type==4)&&arena&&irq_depth==arena_depth&&p>=0xe00000&&watch(p,(unsigned)width))put(9,last_pc,address,(unsigned)width,value,0,0,0,0);
 }
}
