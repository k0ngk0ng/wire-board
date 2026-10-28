/* Offline FCEUX recording adapter. It only calls FCEUX's existing WAV writer
 * on the Lua emulation thread; CPU/APU state and game instructions are unchanged.
 * Kept local to Contra so asset reproduction does not depend on another game's
 * compiled research outputs. No symbol from this file is linked into the game.
 */
#include <dlfcn.h>
#include <stdlib.h>
#include <stdio.h>
#include <sys/stat.h>
static FILE *counts;
static unsigned frame;
int start(void *state) {
    (void)state;
    int (*begin)(const char *) = dlsym(RTLD_DEFAULT, "_Z21FCEUI_BeginWaveRecordPKc");
    const char *path = getenv("NES_AUDIO_WAV"), *ledger = getenv("NES_AUDIO_FRAME_COUNTS");
    if (!begin || !path || !begin(path) || !ledger || !(counts=fopen(ledger,"w"))) abort();
    fputs("frame,samples\n",counts); frame=0; return 0;
}
int checkpoint(void *state) {
    (void)state; fflush(NULL);
    struct stat info; const char *path=getenv("NES_AUDIO_WAV");
    if (!counts || !path || stat(path,&info) || info.st_size<40) abort();
    long long samples=info.st_size<44 ? 0 : (long long)(info.st_size-44)/2;
    fprintf(counts,"%u,%lld\n",frame++,samples); return 0;
}
int stop(void *state) {
    (void)state;
    void (*end)(void)=dlsym(RTLD_DEFAULT,"_Z19FCEUI_EndWaveRecordv");
    if (!end) abort(); end();
    if (counts) {fclose(counts);counts=NULL;} return 0;
}
