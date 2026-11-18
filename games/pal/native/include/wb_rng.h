#ifndef WB_ORIGINAL_RNG_OBSERVER_H
#define WB_ORIGINAL_RNG_OBSERVER_H
void wb_rng_begin(void);
void wb_rng_frame(int clip,int first,int frame);
void wb_rng_wait_begin(double deadline);
void wb_rng_wait_end(void);
int wb_raster_capture_seed(void);
void wb_raster_seed_clear(void);
unsigned wb_raster_seed_info(unsigned slot);
#endif
