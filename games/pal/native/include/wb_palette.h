#ifndef WB_PALETTE_OBSERVATION_H
#define WB_PALETTE_OBSERVATION_H
void wb_palette_set(int palette,int night,int factor);
void wb_palette_begin_out(void);
void wb_palette_out(int factor);
void wb_palette_red(int palette,int night,int step);
#endif
