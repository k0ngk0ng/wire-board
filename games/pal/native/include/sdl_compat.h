#ifndef PAL_HEADLESS_SDL_H
#define PAL_HEADLESS_SDL_H
#include <stdint.h>
typedef uint8_t Uint8;typedef int8_t Sint8;typedef uint16_t Uint16;typedef int16_t Sint16;
typedef uint32_t Uint32;typedef int32_t Sint32;typedef uint64_t Uint64;typedef int64_t Sint64;
#define SDL_MAJOR_VERSION 2
#define SDL_MINOR_VERSION 0
#define SDL_VERSION_ATLEAST(a,b,c) ((a)<=2)
#define SDL_BYTEORDER 1234
#define SDL_LIL_ENDIAN 1234
#define SDL_BIG_ENDIAN 4321
#define SDL_SwapLE16(x) (x)
#define SDL_SwapLE32(x) (x)
#define SDL_INLINE inline
#define SDLCALL
#define SDL_FORCE_INLINE static inline __attribute__((always_inline))
typedef struct {Uint8 r,g,b,a;} SDL_Color;
typedef struct {int ncolors;SDL_Color *colors;} SDL_Palette;
typedef struct {SDL_Palette *palette;Uint8 BitsPerPixel;Uint8 BytesPerPixel;} SDL_PixelFormat;
typedef struct {int w,h,pitch;void *pixels;SDL_PixelFormat *format;} SDL_Surface;
typedef struct {int x,y,w,h;} SDL_Rect;
typedef union {Uint32 type;char storage[128];} SDL_Event;
typedef struct SDL_CD SDL_CD;
typedef struct SDL_RWops SDL_RWops;
typedef struct SDL_AudioStream SDL_AudioStream;
typedef uint32_t SDL_AudioDeviceID;
typedef struct {int freq;Uint16 format;Uint8 channels;Uint16 samples;void (*callback)(void*,Uint8*,int);void *userdata;} SDL_AudioSpec;
typedef enum {SDL_ScaleModeNearest,SDL_ScaleModeLinear} SDL_ScaleMode;
Uint32 SDL_GetTicks(void);
Uint64 SDL_GetPerformanceCounter(void);
Uint64 SDL_GetPerformanceFrequency(void);
void SDL_Delay(Uint32);
int SDL_BlitSurface(SDL_Surface*,const SDL_Rect*,SDL_Surface*,SDL_Rect*);
int SDL_FillRect(SDL_Surface*,const SDL_Rect*,Uint32);
#define SDL_MUSTLOCK(s) (0)
int SDL_LockSurface(SDL_Surface*);
void SDL_UnlockSurface(SDL_Surface*);
int SDL_SoftStretch(SDL_Surface*,const SDL_Rect*,SDL_Surface*,const SDL_Rect*);
void SDL_FreeSurface(SDL_Surface*);
void SDL_LockAudio(void);void SDL_UnlockAudio(void);void SDL_CloseAudio(void);void SDL_PauseAudio(int);
void SDL_LockAudioDevice(SDL_AudioDeviceID);void SDL_UnlockAudioDevice(SDL_AudioDeviceID);void SDL_CloseAudioDevice(SDL_AudioDeviceID);void SDL_PauseAudioDevice(SDL_AudioDeviceID,int);
#endif
