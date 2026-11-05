/* Research-only passive wrapper. The fixed reference executes every operation.
 * Reuse the already frozen CPU/semantic recorder without modifying its source. */
#include "audio_phase_budget_bridge.c"
#include "z80.h"

typedef unsigned char (*phase_read_fn)(unsigned);
typedef void (*phase_write_fn)(unsigned,unsigned char);
typedef struct {
 uint32_t ordinal,frame,kind,guest_address,mapped_address,value,z80_pc,bank;
 int32_t m68k_before,m68k_after;
 uint32_t z80_before,z80_after,dma_end,dma_type,dma_length,zstate_before,zstate_after;
} phase_bus_t;
static phase_bus_t bus_events[32768];
static size_t bus_count;
static unsigned bus_overflow;
static phase_read_fn saved_read,*read_slot;
static phase_write_fn saved_write,*write_slot;
static Z80_Regs *zcpu;
static const uint32_t *zbank_ptr,*dma_end_ptr,*dma_length_ptr;
static const uint8_t *dma_type_ptr,*zstate_ptr;

static int external_bus(unsigned address){return address>=0x8000||(address>=0x7f00&&address<0x8000);}
static phase_bus_t before_bus(unsigned address,unsigned kind){
 phase_bus_t e={0};
 e.frame=frame;e.kind=kind;e.guest_address=address;e.bank=*zbank_ptr;
 e.mapped_address=address>=0x8000?e.bank|(address&0x7fff):0xc00000|(address&0xffff);
 e.z80_pc=zcpu->pc.w.l;e.m68k_before=*cycles;e.z80_before=zcpu->cycles;
 e.dma_end=*dma_end_ptr;e.dma_type=*dma_type_ptr;e.dma_length=*dma_length_ptr;e.zstate_before=*zstate_ptr;
 return e;
}
static void after_bus(phase_bus_t e,unsigned data){
 if(bus_count==32768){bus_overflow=1;return;}
 e.ordinal=(uint32_t)count;e.value=data;e.m68k_after=*cycles;e.z80_after=zcpu->cycles;e.zstate_after=*zstate_ptr;
 bus_events[bus_count]=e;
 put(e.kind,e.z80_pc,e.mapped_address,1,data,(uint32_t)bus_count,e.z80_before,e.z80_after,(unsigned)e.m68k_before);
 ++bus_count;
}
static unsigned char observed_read(unsigned address){
 if(!active||!external_bus(address))return saved_read(address);
 phase_bus_t e=before_bus(address,13);unsigned char data=saved_read(address);after_bus(e,data);return data;
}
static void observed_write(unsigned address,unsigned char data){
 if(!active||!external_bus(address)){saved_write(address,data);return;}
 phase_bus_t e=before_bus(address,14);saved_write(address,data);after_bus(e,data);
}
int phase_clock_attach(void *r,void *w,void *z,void *bank,void *de,void *dt,void *dl,void *zs){
 if(saved_read||!r||!w||!z||!bank||!de||!dt||!dl||!zs)return 0;
 read_slot=(phase_read_fn*)r;write_slot=(phase_write_fn*)w;
 saved_read=*read_slot;saved_write=*write_slot;zcpu=(Z80_Regs*)z;
 zbank_ptr=(const uint32_t*)bank;dma_end_ptr=(const uint32_t*)de;dma_type_ptr=(const uint8_t*)dt;
 dma_length_ptr=(const uint32_t*)dl;zstate_ptr=(const uint8_t*)zs;
 *read_slot=observed_read;*write_slot=observed_write;return 1;
}
int phase_clock_detach(void){
 if(!saved_read)return 0;
 int intact=*read_slot==observed_read&&*write_slot==observed_write;
 *read_slot=saved_read;*write_slot=saved_write;saved_read=0;saved_write=0;active=0;return intact;
}
void phase_clock_begin(unsigned f){bus_count=bus_overflow=0;phase_begin(f);}
size_t phase_clock_bus_count(void){return bus_count;}
unsigned phase_clock_bus_overflow(void){return bus_overflow;}
const phase_bus_t *phase_clock_bus_events(void){return bus_events;}
unsigned phase_clock_bus_size(void){return sizeof(phase_bus_t);}
unsigned phase_clock_z80_size(void){return sizeof(Z80_Regs);}
