/* Offline FCEUX 2.6.6 macOS capture adapter. Not linked into the game.
 * Qt's CLI soundrecord setting does not start recording; invoke the existing
 * recorder from the Lua emulation thread, before/after a bounded frame loop.
 * No CPU, APU, game RAM or original driver instructions are changed here.
 */
#include <dlfcn.h>
#include <stdlib.h>
#include <stdio.h>
#include <sys/stat.h>

static FILE *sample_counts;
static unsigned frame_number;

int start(void *lua_state) {
    (void)lua_state;
    int (*begin)(const char *) = dlsym(RTLD_DEFAULT, "_Z21FCEUI_BeginWaveRecordPKc");
    const char *path = getenv("NES_AUDIO_WAV");
    if (!begin || !path || !begin(path)) abort();
    const char *counts = getenv("NES_AUDIO_FRAME_COUNTS");
    if (!counts || !(sample_counts = fopen(counts, "w"))) abort();
    fputs("frame,samples\n", sample_counts);
    frame_number = 0;
    return 0;
}

int checkpoint(void *lua_state) {
    (void)lua_state;
    /* Flush only host recording buffers; no emulated state is touched. */
    fflush(NULL);
    struct stat info;
    const char *path = getenv("NES_AUDIO_WAV");
    if (!sample_counts || !path || stat(path, &info) || info.st_size < 40) abort();
    /* FCEUX initially seeks past the data-size word without extending the
     * file. Until the first PCM write its on-disk header is only 40 bytes. */
    long long samples = info.st_size < 44 ? 0 : (long long)(info.st_size - 44) / 2;
    fprintf(sample_counts, "%u,%lld\n", frame_number++, samples);
    return 0;
}

int stop(void *lua_state) {
    (void)lua_state;
    void (*end)(void) = dlsym(RTLD_DEFAULT, "_Z19FCEUI_EndWaveRecordv");
    if (!end) abort();
    end();
    if (sample_counts) { fclose(sample_counts); sample_counts = NULL; }
    return 0;
}
