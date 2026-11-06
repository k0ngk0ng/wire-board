/* Passive Main-side ordinary command snapshot and metadata recorder. */
#include "score-clock-bridge.c"
static read_fn ordinary_native_read;
static write_fn ordinary_native_write;
static unsigned ordinary_read(unsigned address) {
  unsigned before=*main_cycles;
  unsigned value=ordinary_native_read(address);
  unsigned field=address&0xffff;
  if(field==0x1ffc||field==0x1f87||field==0x1fa7||field==0x1f86||field==0x1fa6)
    put(25,before,1,address,value,1);
  return value;
}
static void ordinary_write(unsigned address,unsigned value) {
  unsigned before=*main_cycles;
  ordinary_native_write(address,value);
  unsigned field=address&0xffff;
  if((field>=0x1f80&&field<=0x1f87)||(field>=0x1fa0&&field<=0x1fa7)||
     (field>=0x1fe6&&field<=0x1fe9)||field==0x1ffd||field==0x1ffe||field==0x1ffb||field==0x1ffa)
    put(26,before,1,address,value,1);
}
int score_clock_ordinary_enable(void) {
  if(!saved_fm_read||ordinary_native_read)return 0;
  ordinary_native_read=zram_read;ordinary_native_write=zram_write;
  zram_read=ordinary_read;zram_write=ordinary_write;return 1;
}
int score_clock_ordinary_disable(void) {
  if(!ordinary_native_read||zram_read!=ordinary_read||zram_write!=ordinary_write)return 0;
  zram_read=ordinary_native_read;zram_write=ordinary_native_write;
  ordinary_native_read=0;ordinary_native_write=0;return 1;
}
