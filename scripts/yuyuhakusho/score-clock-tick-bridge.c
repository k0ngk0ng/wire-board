/* Additional passive fields for the fixed native tick gate. */
#include "score-clock-bridge.c"
void score_clock_tick_hook(int type,int width,unsigned address,unsigned value){
 score_clock_hook(type,width,address,value);
 unsigned low=address&65535;
 if(type==2&&address>=0xe00000&&low==0xf807)put(13,(unsigned)*main_cycles,(unsigned)width,address,value,1);
 if(type==4&&address>=0xe00000){
  if(low==0xf80e)put(14,(unsigned)*main_cycles,(unsigned)width,address,value,1);
  if(low==0xf800)put(9,(unsigned)*main_cycles,(unsigned)width,address,value,1);
  if(low==0xf807)put(10,(unsigned)*main_cycles,(unsigned)width,address,value,1);
 }
}
