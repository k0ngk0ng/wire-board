/* Research-only passive PC filter. Does not link into the platform or game. */
#include <stdint.h>
#include <stddef.h>

typedef void (*observer_t)(int, int, unsigned, unsigned);
static observer_t observer;
static unsigned addresses[64];
static size_t address_count;

int yyh_set_observer(observer_t callback, const unsigned *pcs, size_t count) {
    if (count > 64 || (count && !pcs)) return 0;
    observer = callback;
    address_count = count;
    for (size_t i = 0; i < count; i++) addresses[i] = pcs[i];
    return 1;
}

void yyh_cpu_hook(int type, int width, unsigned address, unsigned value) {
    if (type != 1 || !observer) return;
    for (size_t i = 0; i < address_count; i++) {
        if (address == addresses[i]) {
            observer(type, width, address, value);
            return;
        }
    }
}
