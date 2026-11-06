/* Passive world-IRQ entry through its actual world-wrapper call. */
#include "battle-prefix-observer.c"
typedef struct{event_t base;uint32_t world_flags,pause,blocked,bus_state,aux_cycle;} world_event_t;
static world_event_t world_events[8192];static const uint8_t*world_zram,*world_bus;static const uint32_t*world_aux;
int battle_world_attach(void*zram,void*bus,void*aux){if(!zram||!bus||!aux)return 0;world_zram=zram;world_bus=bus;world_aux=aux;return 1;}
static void world_put(unsigned kind,unsigned pc,unsigned width,unsigned address,unsigned value){unsigned before=count;put(kind,pc,width,address,value);if(count!=before)world_events[before]=(world_event_t){events[before],ram[0xf50e^1],ram[0xf807^1],world_zram[0x1fff],*world_bus,*world_aux};}
void battle_world_hook(int type,int width,unsigned address,unsigned value){unsigned p=address&0xffffff;if(type==1){last_pc=p;if(!active&&!done&&p==0xfff000&&word(0xf516)>=4){active=1;world_put(9,p,0,0,0);}if(active){if(p==0x32d6){world_put(10,p,0,0,0);active=0;done=1;}else world_put(1,p,0,0,0);}}else if(active&&(type==2||type==4)){if((p>=0xc00000&&p<0xe00000)||(p&0xffff00)==0xa11100||p==0xa01fff)world_put((unsigned)type,last_pc,(unsigned)width,p,value);}}
const world_event_t*battle_world_events(void){return world_events;}
unsigned battle_world_size(void){return sizeof(world_event_t);}
