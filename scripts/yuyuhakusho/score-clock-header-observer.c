/* Research-only read-only boundary sampler; never linked into production. */
#include <stdint.h>
typedef struct { uint32_t pc, before, deadline, after, next_deadline; } step_t;
static const int32_t *cycles, *refresh;
static const uint32_t *pc;
static step_t steps[128];
static unsigned count, overflow, pending;
int score_clock_header_attach(const void *cycle, const void *deadline, const void *program_counter) {
    if (cycles || !cycle || !deadline || !program_counter) return 0;
    cycles = cycle; refresh = deadline; pc = program_counter;
    count = overflow = pending = 0;
    return 1;
}
void score_clock_header_before(void) {
    if (!cycles || pending || count == 128) { overflow = 1; return; }
    steps[count] = (step_t){*pc, (unsigned)*cycles, (unsigned)*refresh, 0, 0};
    pending = 1;
}
void score_clock_header_after(void) {
    if (!cycles || !pending) { overflow = 1; return; }
    steps[count].after = (unsigned)*cycles;
    steps[count].next_deadline = (unsigned)*refresh;
    count++; pending = 0;
}
int score_clock_header_detach(void) {
    int valid = cycles && !pending && !overflow;
    cycles = refresh = 0; pc = 0;
    return valid;
}
unsigned score_clock_header_count(void) { return count; }
unsigned score_clock_header_overflow(void) { return overflow; }
unsigned score_clock_header_size(void) { return sizeof(step_t); }
const step_t *score_clock_header_steps(void) { return steps; }
