#ifndef WB_ORIGINAL_WAIT_INPUT_H
#define WB_ORIGINAL_WAIT_INPUT_H
#include "main.h"

// Keep the original delay loop, then poll arrivals made during the suspended
// wait before its caller executes the frame. Ordinary SDL_Delay menu loops
// retain their own clear-then-poll order and must not consume these arrivals.
#define WB_DelayUntil(time) do { PAL_DelayUntil(time); PAL_ProcessEvent(); } while (0)
#endif
