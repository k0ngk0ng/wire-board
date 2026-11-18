#include "main.h"

extern int wb_host_yield(int kind,int value) __attribute__((import_module("pal"),import_name("yield")));
extern VOID WB_OriginalPAL_ReloadInNextTick(INT slot);

static unsigned game_over_info[3]={1,0,0};

unsigned wb_game_over_info(void){return (unsigned)game_over_info;}

void wb_game_over_begin(void){
 if(game_over_info[1]==2)TerminateOnError("native failure already awaits recovery");
 game_over_info[1]=1;
}

VOID PAL_ReloadInNextTick(INT slot){
 if(!game_over_info[1]){WB_OriginalPAL_ReloadInNextTick(slot);return;}
 if(slot<0||slot>5)TerminateOnError("invalid original failure save slot");
 game_over_info[1]=2;game_over_info[2]=(unsigned)slot;
 for(;;)wb_host_yield(7,slot);
}
