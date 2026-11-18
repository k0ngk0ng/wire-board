#include "main.h"

extern unsigned wb_audio_clock(void);

static unsigned audio_serial;
static int audio_commands[128][6];
static int audio_info[3 + 128 * 6];
static int current_cd = -1;

static void audio_command(int kind, int id, BOOL loop, FLOAT fade) {
 if (audio_serial == 0xffffffffu || id < -2 || id > 65535 || fade < 0 || fade > 60) TerminateOnError("invalid native audio command");
 unsigned serial = ++audio_serial;
 int *entry = audio_commands[(serial - 1) % 128];
 entry[0] = (int)serial;
 entry[1] = (int)wb_audio_clock();
 entry[2] = kind;
 entry[3] = id;
 entry[4] = !!loop;
 entry[5] = (int)(fade * 1000 + 0.5f);
}

unsigned wb_audio_info(void) {
 unsigned count = audio_serial < 128 ? audio_serial : 128;
 audio_info[0] = 1;
 audio_info[1] = (int)audio_serial;
 audio_info[2] = (int)count;
 for (unsigned i = 0; i < count; i++) {
  unsigned serial = audio_serial - count + i + 1;
  memcpy(audio_info + 3 + i * 6, audio_commands[(serial - 1) % 128], 6 * sizeof(int));
 }
 return (unsigned)audio_info;
}

void AUDIO_PlayMusic(INT id, BOOL loop, FLOAT fade) {
 if (id > 0) AUDIO_PlayCDTrack(-1);
 audio_command(1, id, loop, fade);
}

void AUDIO_PlaySound(INT id) {
 if (id < -65535 || id > 65535) TerminateOnError("invalid native sound number");
 audio_command(2, id < 0 ? -id : id, FALSE, 0);
}

BOOL AUDIO_CD_Available(void) {
 return TRUE;
}

BOOL AUDIO_PlayCDTrack(INT id) {
 if (id > 0) AUDIO_PlayMusic(-1, FALSE, 0);
 if (id == -2 && current_cd >= 2 && current_cd <= 9) return TRUE;
 BOOL available = id >= 2 && id <= 9;
 current_cd = available ? id : -1;
 audio_command(3, id, id > 0, 0);
 return available || id == -1;
}
