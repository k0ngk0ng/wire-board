#ifndef WB_SCENE_OBSERVATION_H
#define WB_SCENE_OBSERVATION_H
void wb_scene_reset(void);
void wb_scene_wave(int level,int phase,int progression);
void wb_scene_sprite(int asset,int frame,int entity,int kind,int x,int base_y,int sort_y);
void wb_scene_cover(int x,int y,int half,int layer,int height);
#endif
