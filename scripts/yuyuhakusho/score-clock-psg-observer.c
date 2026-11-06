/* Research-only passive PSG map wrapper. Never linked into production. */
#include <stddef.h>
#include <stdint.h>
typedef unsigned (*read_fn)(unsigned);
typedef void (*write_fn)(unsigned, unsigned);
typedef struct { void *base; read_fn read8, read16; write_fn write8, write16; } map_t;
typedef struct { uint32_t pc, at, after, refresh, address, value; } event_t;
static map_t *maps;
static int32_t *cycles, *refresh;
static write_fn saved_write;
static unsigned pc, count, overflow;
static event_t events[16];
static void observed_write(unsigned address, unsigned value) {
    unsigned at = (unsigned)*cycles;
    saved_write(address, value);
    if ((address & 0xffffff) == 0xc00011) {
        if (count == 16) { overflow = 1; return; }
        events[count++] = (event_t){pc, at, (unsigned)*cycles, (unsigned)*refresh, address, value};
    }
}
int score_clock_psg_attach(void *map, void *cycle, void *deadline) {
    if (saved_write || !map || !cycle || !deadline) return 0;
    maps = (map_t *)map; cycles = (int32_t *)cycle; refresh = (int32_t *)deadline;
    saved_write = maps[0xc0].write8;
    if (!saved_write) return 0;
    maps[0xc0].write8 = observed_write; count = overflow = pc = 0;
    return 1;
}
int score_clock_psg_detach(void) {
    if (!saved_write) return 0;
    int intact = maps[0xc0].write8 == observed_write;
    maps[0xc0].write8 = saved_write; saved_write = 0;
    return intact;
}
void score_clock_psg_pc(unsigned value) { pc = value; }
unsigned score_clock_psg_count(void) { return count; }
unsigned score_clock_psg_overflow(void) { return overflow; }
unsigned score_clock_psg_size(void) { return sizeof(event_t); }
const event_t *score_clock_psg_events(void) { return events; }
