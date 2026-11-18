#ifndef WB_ORIGINAL_UI_OBSERVATION_H
#define WB_ORIGINAL_UI_OBSERVATION_H
void wb_dialog_start(int face);
void wb_dialog_page(void);
void wb_dialog_message(LPCWSTR text);
void wb_dialog_wait(float maximum);
void wb_dialog_wait_end(void);
void wb_ui_resource(const unsigned char *data,int length,unsigned chunk,FILE *file);
void wb_ui_rle(LPCBITMAPRLE bitmap,SDL_Surface *surface,PAL_POS pos,BOOL shadow);
void wb_menu_push(int kind,int role);
void wb_menu_pop(void);
void wb_menu_role(int role);
void wb_menu_list(LPCMENUITEM items,int count,int selected);
BOOL wb_platform_system(void);
int wb_multiplayer(void);
PALDIRECTION wb_input_direction(void);
#endif
