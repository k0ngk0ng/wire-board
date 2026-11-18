#ifndef WB_RASTER_GRAPH_H
#define WB_RASTER_GRAPH_H
// Immutable 16-word nodes: kind, input A/B, width/height, 11 parameters.
// Only asset IDs and original draw operations are exported, never pixel data.
void wb_raster_enable(void);
void wb_raster_reset(void);
void wb_raster_begin_display(int partial,int physical);
void wb_raster_replace_index(SDL_Surface *surface,int from,int to);
int wb_raster_active(void);
void wb_raster_surface_free(SDL_Surface *surface);
void wb_raster_image(SDL_Surface *surface,int source,int id,int frame,int x,int y,int mode,int color,int shift);
void wb_raster_text(SDL_Surface *surface,int code,int x,int y,int color,int small);
void wb_raster_copy(SDL_Surface *src,SDL_Surface *dst,int sx,int sy,int dx,int dy,int w,int h);
void wb_raster_fill(SDL_Surface *surface,int x,int y,int w,int h,int color);
void wb_raster_stretch(SDL_Surface *src,SDL_Surface *dst,int sx,int sy,int sw,int sh,int dx,int dy,int dw,int dh);
void wb_raster_background(void);
void wb_raster_wave(SDL_Surface *surface,int level,int phase);
void wb_raster_blend(int pass,int component);
void wb_raster_blend_surfaces(SDL_Surface *src,SDL_Surface *dst,int pass,int component);
void wb_raster_rng_begin(void);
void wb_raster_rng_frame(int clip,int first,int frame);
unsigned wb_raster_graph(void);
#endif
