#include "main.h"

extern int wb_host_yield(int kind, int value) __attribute__((import_module("pal"), import_name("yield")));

static unsigned ending_info[9] = {1, 0, 0, 0, 15, 1, 0, 0, 0};

unsigned wb_ending_info(void) {
 return (unsigned)ending_info;
}

BOOL PAL_PlayAVI(LPCSTR path) {
 unsigned clip = 0, frames = 0;
 if (!strcmp(path, "4.avi")) {clip = 4; frames = 200;}
 if (!strcmp(path, "5.avi")) {clip = 5; frames = 2294;}
 if (!strcmp(path, "6.avi")) {clip = 6; frames = 2196;}
 if (!clip) return FALSE;
 PAL_ClearKeyState();
 ending_info[1] = clip;
 ending_info[3] = frames;
 ending_info[7]++;
 for (unsigned frame = 0; frame < frames; frame++) {
  ending_info[2] = frame;
  ending_info[6] = frame * 1000 / 15;
  UTIL_Delay((frame + 1) * 1000 / 15 - ending_info[6]);
  if (g_InputState.dwKeyPress & (kKeyMenu | kKeySearch)) {
   UTIL_Delay(500);
   break;
  }
 }
 ending_info[1] = 0;
 return TRUE;
}

VOID PAL_Shutdown(int status) {
 if (status != 0) TerminateOnError("native PAL terminated with failure: %d", status);
 ending_info[8] = 1;
 for (;;) wb_host_yield(3, 0);
}

void wb_probe_ending(void) {
 PAL_EndingScreen();
 PAL_Shutdown(0);
}
