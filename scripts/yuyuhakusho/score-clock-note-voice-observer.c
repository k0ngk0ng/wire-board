/* Research-only composition of the existing music recorder with pan restore. */
#include "score-clock-music-observer.c"
static write_fn note_native_write;
static void note_write(unsigned address, unsigned value) {
    unsigned before = *main_cycles;
    note_native_write(address, value);
    if ((address & 0xffff) == 0x1ff8) put(18, before, 1, address, value, 1);
}
int score_clock_note_enable(void) {
    if (note_native_write || !score_clock_music_enable()) return 0;
    note_native_write = music_native_write;
    music_native_write = note_write;
    return 1;
}
int score_clock_note_disable(void) {
    if (!note_native_write || music_native_write != note_write) return 0;
    music_native_write = note_native_write;
    note_native_write = 0;
    return score_clock_music_disable();
}
