/* Passive suffix and delegated native acknowledgement evidence. */
#include "battle-prefix-observer.c"
static unsigned return_pc,return_sp;static int(*saved_ack)(int);
static int return_ack(int level){if(active)put(7,last_pc,0,0,(unsigned)level);int value=saved_ack(level);if(active)put(8,last_pc,0,0,(unsigned)level);return value;}
int battle_return_ack_attach(void*expected){if(saved_ack||!expected)return 0;saved_ack=(int(*)(int))expected;return 1;}
void*battle_return_ack_pointer(void){return (void*)return_ack;}
int battle_return_ack_detach(void){if(!saved_ack)return 0;saved_ack=0;return 1;}
void battle_return_begin(void){battle_prefix_begin();return_pc=return_sp=0;}
void battle_return_start(unsigned target,unsigned stack){return_pc=target;return_sp=stack;active=1;put(9,get(16),0,0,0);}
void battle_return_cpu(unsigned pc){last_pc=pc;put(1,pc,0,0,0);}
void battle_return_end(unsigned pc){put(10,pc,0,0,0);active=0;done=1;}
void battle_return_hook(int type,int width,unsigned address,unsigned value){unsigned p=address&0xffffff;if(type==1){last_pc=p;if(!active&&!done&&p==0xfff096){unsigned stack=get(15)&65535;return_pc=((word((stack+58)&65535)<<16)|word((stack+60)&65535))&0xffffff;return_sp=get(15)+62;active=1;put(9,p,0,0,0);}if(active){if(p==return_pc&&get(15)==return_sp){battle_return_end(p);}else put(1,p,0,0,0);}}else if(active&&(type==2||type==4)&&p>=0xc00000&&p<0xe00000)put((unsigned)type,last_pc,(unsigned)width,p,value);}

void battle_return_pending(unsigned pc){put(11,pc,0,0,0);active=0;done=1;}

typedef unsigned (*return_read_fn)(unsigned);
typedef void (*return_write_fn)(unsigned,unsigned);
typedef struct{void*base;return_read_fn read8,read16;return_write_fn write8,write16;} return_map;
static return_map*return_maps;static return_write_fn return_saved_write;
static void return_write16(unsigned address,unsigned value){if(active)put(4,last_pc,2,address,value);return_saved_write(address,value);}
int battle_return_io_attach(void*map){if(return_saved_write||!map)return 0;return_maps=map;return_saved_write=return_maps[0xc0].write16;if(!return_saved_write)return 0;return_maps[0xc0].write16=return_write16;return 1;}
int battle_return_io_detach(void){if(!return_saved_write)return 0;int valid=return_maps[0xc0].write16==return_write16;return_maps[0xc0].write16=return_saved_write;return_saved_write=0;return valid;}
