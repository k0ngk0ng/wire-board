/* Research-only passive complete-3592 observer. No game/core memory writes. */
#include <stdint.h>
#include <stddef.h>
#include <string.h>

typedef uint32_t (*get_reg_t)(unsigned);
typedef struct {
    uint32_t kind, pass, pc, frame, cycle, refresh, a5, d0, sp;
    uint32_t behavior, size, stage, sr, target, saved;
    uint32_t after_pc, after_frame, after_cycle, after_refresh, after_a5, after_sp;
} scan_event;
static scan_event records[65536];
static uint8_t *ram;
static int32_t *cycles, *refresh;
static uint32_t *batch, last_target, last_pc;
static get_reg_t get;
static uint32_t frame, count, passes, complete, overflow, overlap;
static int allow, active, callback, entering, pending = -1;
typedef unsigned char (*read_fn)(unsigned);
static read_fn saved_read, *read_slot;

static uint32_t read_word(uint32_t address) {
    return ((uint32_t)ram[(address & 65535) ^ 1] << 8) |
           ram[((address + 1) & 65535) ^ 1];
}
static uint32_t read_long(uint32_t address) {
    return (read_word(address) << 16) | read_word(address + 2);
}
static int scanner(uint32_t pc) {
    switch (pc) {
    case 0x3592: case 0x3596: case 0x359a: case 0x359c: case 0x359e:
    case 0x35a2: case 0x35a4: case 0x35a6: case 0x35aa: case 0x35ac:
    case 0x35ae: case 0x35b2: case 0x35b6: case 0x35b8: return 1;
    default: return 0;
    }
}
static int append(uint32_t kind, uint32_t pc) {
    if (count == 65536) { overflow = 1; return -1; }
    scan_event *e = &records[count];
    memset(e, 0, sizeof(*e));
    e->kind = kind; e->pass = passes; e->pc = pc;
    e->frame = frame; e->cycle = *cycles; e->refresh = *refresh;
    e->a5 = get(13); e->d0 = get(0); e->sp = get(15); e->sr = get(17);
    e->target = get(8); e->saved = read_word(e->sp + (kind == 1 ? 4 : 0));
    if ((e->a5 & 65535) >= 0x8000 && (e->a5 & 65535) < 0xa000) {
        e->behavior = read_long(e->a5);
        e->size = read_word(e->a5 + 4);
        e->stage = read_word(e->a5 + 6);
    }
    return (int)count++;
}
int world_object_scan_work_attach(void *memory, void *cycle, void *deadline, void *budget, void *reader) {
    if (!memory || !cycle || !deadline || !budget || !reader) return 0;
    ram = memory; cycles = cycle; refresh = deadline; batch = budget; get = (get_reg_t)reader;
    count = passes = complete = overflow = overlap = frame = 0;
    allow = active = callback = entering = 0; pending = -1; last_target = last_pc = 0;
    return 1;
}
static unsigned char observed_read(unsigned address) {
    uint32_t before = (uint32_t)*cycles;
    unsigned char value = saved_read(address);
    if (active && address >= 0x8000) {
        int index = append(3, last_pc);
        if (index >= 0) {
            scan_event *e = &records[index];
            e->cycle = before; e->after_cycle = (uint32_t)*cycles;
            e->after_frame = frame; e->after_refresh = (uint32_t)*refresh;
            e->target = *batch ? *batch : last_target; e->saved = address;
        }
    }
    return value;
}
int world_object_scan_work_attach_read(void *slot) {
    if (!slot || saved_read) return 0;
    read_slot = slot; saved_read = *read_slot; *read_slot = observed_read; return 1;
}
int world_object_scan_work_detach_read(void) {
    if (!saved_read || *read_slot != observed_read) return 0;
    *read_slot = saved_read; saved_read = 0; return 1;
}
void world_object_scan_work_frame(uint32_t value, int selected) { frame = value; allow = selected; }
void world_object_scan_work_hook(int kind, int width, unsigned pc, unsigned value) {
    (void)width; (void)value;
    if (kind != 1 || !ram) return;
    last_pc = pc; if (*batch) last_target = *batch;
    if (pending >= 0) {
        scan_event *e = &records[pending];
        e->after_pc = pc; e->after_frame = frame;
        e->after_cycle = *cycles; e->after_refresh = *refresh;
        e->after_a5 = get(13); e->after_sp = get(15);
        if (e->pc == 0x35ac || e->pc == 0x35b8) { active = 0; complete++; }
        pending = -1;
    }
    if (pc == 0x3592 && allow) {
        if (active) { overlap++; return; }
        active = 1; passes++;
    }
    if (!active) return;
    if (entering) { append(1, pc); entering = 0; }
    if (pc == 0x35a4 && callback) { append(2, pc); callback = 0; }
    if (!scanner(pc)) return;
    pending = append(0, pc);
    if (pc == 0x35a2) { callback = 1; entering = 1; }
}
scan_event *world_object_scan_work_events(void) { return records; }
uint32_t world_object_scan_work_count(void) { return count; }
uint32_t world_object_scan_work_complete(void) { return complete; }
uint32_t world_object_scan_work_overflow(void) { return overflow; }
uint32_t world_object_scan_work_overlap(void) { return overlap; }
uint32_t world_object_scan_work_pending(void) { return active || pending >= 0; }
uint32_t world_object_scan_work_size(void) { return sizeof(scan_event); }
