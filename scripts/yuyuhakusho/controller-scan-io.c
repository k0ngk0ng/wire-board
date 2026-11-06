/* Research-only passive I/O-map wrapper, not linked into production. */
#include <stddef.h>
#include <stdint.h>
typedef unsigned (*read_fn)(unsigned);
typedef void (*write_fn)(unsigned,unsigned);
typedef struct{void*base;read_fn read8,read16;write_fn write8,write16;} map_t;
typedef struct{uint32_t pc,at,after,address,value,write;} event_t;
static map_t*maps;static int32_t*cycles;static read_fn saved_read;static write_fn saved_write;static unsigned pc,count,overflow;static event_t events[512];
static void record(unsigned address,unsigned value,unsigned write,unsigned at){if(count==512){overflow=1;return;}events[count++]=(event_t){pc,at,(unsigned)*cycles,address,value,write};}
static unsigned observed_read(unsigned address){unsigned at=(unsigned)*cycles;unsigned value=saved_read(address);if((address&0xfffff0)==0xa10000)record(address,value,0,at);return value;}
static void observed_write(unsigned address,unsigned value){unsigned at=(unsigned)*cycles;saved_write(address,value);if((address&0xfffff0)==0xa10000)record(address,value,1,at);}
int controller_io_attach(void*map,void*clock){if(saved_read||saved_write||!map||!clock)return 0;maps=map;cycles=clock;saved_read=maps[0xa1].read8;saved_write=maps[0xa1].write8;if(!saved_read||!saved_write)return 0;maps[0xa1].read8=observed_read;maps[0xa1].write8=observed_write;count=overflow=pc=0;return 1;}
int controller_io_detach(void){if(!saved_read||!saved_write)return 0;int intact=maps[0xa1].read8==observed_read&&maps[0xa1].write8==observed_write;maps[0xa1].read8=saved_read;maps[0xa1].write8=saved_write;saved_read=0;saved_write=0;return intact;}
void controller_io_pc(unsigned value){pc=value;}
unsigned controller_io_count(void){return count;}
unsigned controller_io_overflow(void){return overflow;}
const event_t*controller_io_events(void){return events;}
