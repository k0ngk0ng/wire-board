#ifndef WB_HEADLESS_VIDEO_H
#define WB_HEADLESS_VIDEO_H
typedef struct {void (*RenderCopy)(void);} WBRenderBackend;
extern WBRenderBackend gRenderBackend;
extern SDL_Surface *gpScreenReal;
extern BOOL bScaleScreen;
extern WORD g_wShakeTime,g_wShakeLevel;
#endif
