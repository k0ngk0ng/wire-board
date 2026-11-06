/* Research-only extension of the pinned passive observer; no production link. */
#include "score-clock-bridge.c"
static read_fn pan_native_read;
static write_fn pan_native_write;
static unsigned pan_read(unsigned address) {
  unsigned before = *main_cycles;
  unsigned value = pan_native_read(address);
  if ((address & 0xffff) == 0x1ffd) put(17,before,1,address,value,1);
  return value;
}
static void pan_write(unsigned address,unsigned value) {
  unsigned before = *main_cycles;
  pan_native_write(address,value);
  if ((address & 0xffff) == 0x1ff8) put(18,before,1,address,value,1);
}
int score_clock_pan_enable(void) {
  if (!saved_fm_read || pan_native_read) return 0;
  pan_native_read=zram_read;pan_native_write=zram_write;
  zram_read=pan_read;zram_write=pan_write;
  return 1;
}
int score_clock_pan_disable(void) {
  if (!pan_native_read || zram_read!=pan_read || zram_write!=pan_write) return 0;
  zram_read=pan_native_read;zram_write=pan_native_write;
  pan_native_read=0;pan_native_write=0;
  return 1;
}
