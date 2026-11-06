/* Research-only passive request/flags/publication recorder. */
#include "score-clock-bridge.c"
static read_fn music_native_read;
static write_fn music_native_write;
static unsigned music_read(unsigned address) {
  unsigned before = *main_cycles;
  unsigned value = music_native_read(address);
  unsigned field = address & 0xffff;
  if (field == 0x1ffd) put(17,before,1,address,value,1);
  if (field == 0x1ffc) put(19,before,1,address,value,1);
  return value;
}
static void music_write(unsigned address,unsigned value) {
  unsigned before = *main_cycles;
  music_native_write(address,value);
  unsigned field = address & 0xffff;
  if (field == 0x1ffd || field == 0x1fe8 || field == 0x1fe9 ||
      field == 0x1fe6 || field == 0x1fe7 || field == 0x1ffe ||
      field == 0x1ffb || field == 0x1ff9) put(20,before,1,address,value,1);
}
int score_clock_music_enable(void) {
  if (!saved_fm_read || music_native_read) return 0;
  music_native_read=zram_read;music_native_write=zram_write;
  zram_read=music_read;zram_write=music_write;
  return 1;
}
int score_clock_music_disable(void) {
  if (!music_native_read || zram_read!=music_read || zram_write!=music_write) return 0;
  zram_read=music_native_read;zram_write=music_native_write;
  music_native_read=0;music_native_write=0;
  return 1;
}
