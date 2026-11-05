/* Passive local native port recorder, never linked into production. */
#include <stdint.h>
#include <stddef.h>
typedef unsigned (*read_fn)(unsigned);
typedef void (*write_fn)(unsigned,unsigned);
typedef unsigned (*fm_read_fn)(unsigned,unsigned);
typedef void (*fm_write_fn)(unsigned,unsigned,unsigned);
typedef struct {void *base;read_fn read8,read16;write_fn write8,write16;} map_t;
typedef struct {uint32_t kind,pc,at,after,refresh,width,address,value,zstate,z80,owner;} event_t;
static event_t events[65536];static size_t count;static unsigned overflow,pc,depth;
static int32_t *main_cycles,*refresh;
static uint32_t *aux_cycles;static uint8_t *bus_state;
static map_t *maps;static read_fn old_r8,old_r16,zram_read;static write_fn old_w8,old_w16,zram_write;
static fm_read_fn saved_fm_read,*fm_read_slot;static fm_write_fn saved_fm_write,*fm_write_slot;
static void put(unsigned kind,unsigned before,unsigned width,unsigned address,unsigned value,unsigned owner){
 if(count==65536){overflow=1;return;}
 events[count++]=(event_t){kind,pc,before,(unsigned)*main_cycles,(unsigned)*refresh,width,address,value,*bus_state,*aux_cycles,owner};
}
static unsigned observed_fm_read(unsigned cycle,unsigned address){unsigned result=saved_fm_read(cycle,address);put(1,cycle,1,address,result,depth!=0);return result;}
static void observed_fm_write(unsigned cycle,unsigned address,unsigned value){saved_fm_write(cycle,address,value);put(2,cycle,1,address,value,depth!=0);}
static unsigned observed_zram_read(unsigned address){unsigned before=*main_cycles;++depth;unsigned value=zram_read(address);--depth;if((address&0xffff)==0x1f2a)put(3,before,1,address,value,1);return value;}
static void observed_zram_write(unsigned address,unsigned value){++depth;zram_write(address,value);--depth;}
static void after_control(void){if(*bus_state==3){maps[0xa0].read8=observed_zram_read;maps[0xa0].write8=observed_zram_write;}}
static unsigned observed_r8(unsigned address){unsigned before=*main_cycles;unsigned value=old_r8(address);if((address&0xffff00)==0xa11100)put(6,before,1,address,value,1);return value;}
static unsigned observed_r16(unsigned address){unsigned before=*main_cycles;unsigned value=old_r16(address);if((address&0xffff00)==0xa11100)put(6,before,2,address,value,1);return value;}
static void observed_w8(unsigned address,unsigned value){unsigned before=*main_cycles;if((address&0xffff00)==0xa11100)put(4,before,1,address,value,1);old_w8(address,value);after_control();if((address&0xffff00)==0xa11100)put(5,before,1,address,value,1);}
static void observed_w16(unsigned address,unsigned value){unsigned before=*main_cycles;if((address&0xffff00)==0xa11100)put(4,before,2,address,value,1);old_w16(address,value);after_control();if((address&0xffff00)==0xa11100)put(5,before,2,address,value,1);}
void score_clock_pc(unsigned value){pc=value;}
void score_clock_reset(void){count=overflow=depth=pc=0;}
int score_clock_attach(void *mm,void *mc,void *rf,void *zc,void *zs,void *fr,void *fw,void *zr,void *zw){
 if(saved_fm_read||!mm||!mc||!rf||!zc||!zs||!fr||!fw||!zr||!zw)return 0;
 maps=(map_t*)mm;main_cycles=(int32_t*)mc;refresh=(int32_t*)rf;aux_cycles=(uint32_t*)zc;bus_state=(uint8_t*)zs;
 old_r8=maps[0xa1].read8;old_r16=maps[0xa1].read16;old_w8=maps[0xa1].write8;old_w16=maps[0xa1].write16;
 zram_read=(read_fn)zr;zram_write=(write_fn)zw;
 fm_read_slot=(fm_read_fn*)fr;fm_write_slot=(fm_write_fn*)fw;saved_fm_read=*fm_read_slot;saved_fm_write=*fm_write_slot;
 maps[0xa1].read8=observed_r8;maps[0xa1].read16=observed_r16;maps[0xa1].write8=observed_w8;maps[0xa1].write16=observed_w16;
 *fm_read_slot=observed_fm_read;*fm_write_slot=observed_fm_write;after_control();score_clock_reset();return 1;
}
int score_clock_detach(void){
 if(!saved_fm_read)return 0;
 int intact=maps[0xa1].read8==observed_r8&&maps[0xa1].write16==observed_w16&&*fm_read_slot==observed_fm_read&&*fm_write_slot==observed_fm_write;
 maps[0xa1].read8=old_r8;maps[0xa1].read16=old_r16;maps[0xa1].write8=old_w8;maps[0xa1].write16=old_w16;
 if(maps[0xa0].read8==observed_zram_read)maps[0xa0].read8=zram_read;
 if(maps[0xa0].write8==observed_zram_write)maps[0xa0].write8=zram_write;
 *fm_read_slot=saved_fm_read;*fm_write_slot=saved_fm_write;saved_fm_read=0;saved_fm_write=0;return intact;
}
size_t score_clock_count(void){return count;}
unsigned score_clock_overflow(void){return overflow;}
unsigned score_clock_size(void){return sizeof(event_t);}
const event_t *score_clock_events(void){return events;}
void score_clock_hook(int type,int width,unsigned address,unsigned value){
 if(type==1)pc=address;
 else if(type==4&&address>=0xfffffbe0&&address<=0xfffffbff)put(7,(unsigned)*main_cycles,(unsigned)width,address,value,1);
}
