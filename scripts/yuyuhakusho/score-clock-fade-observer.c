/* Research-only boundary sampler plus actual observed-core RAM write hook. */
#include "score-clock-header-observer.c"
typedef struct { uint32_t pc, at, address, value; } fade_effect_t;
static fade_effect_t fade_effects[8];
static unsigned fade_count, fade_overflow, executing_pc;
int score_clock_fade_attach(const void *cycle, const void *deadline, const void *program_counter) {
    fade_count = fade_overflow = executing_pc = 0;
    return score_clock_header_attach(cycle, deadline, program_counter);
}
void score_clock_fade_before(void) {
    score_clock_header_before();
    executing_pc = *pc;
}
void score_clock_fade_hook(int kind, int width, unsigned address, unsigned value) {
    address &= 0xffffff;
    if (kind != 4 || width != 1 || (address != 0xfff804 && address != 0xfff806 && address != 0xfff840)) return;
    if (fade_count == 8) { fade_overflow = 1; return; }
    fade_effects[fade_count++] = (fade_effect_t){executing_pc, (unsigned)*cycles, address, value & 255};
}
unsigned score_clock_fade_count(void) { return fade_count; }
unsigned score_clock_fade_overflow(void) { return fade_overflow; }
unsigned score_clock_fade_size(void) { return sizeof(fade_effect_t); }
const fade_effect_t *score_clock_fade_effects(void) { return fade_effects; }
