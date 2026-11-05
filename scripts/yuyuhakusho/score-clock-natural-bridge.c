/* Add only natural driver/priority/pause markers to the passive port ledger. */
#include "score-clock-bridge.c"
static const unsigned char *natural_ram;
void score_clock_natural_memory(const unsigned char *ram){natural_ram=ram;}
void score_clock_natural_hook(int type,int width,unsigned address,unsigned value){
 score_clock_hook(type,width,address,value);
 if(type==1&&address==0x375ae)put(8,(unsigned)*main_cycles,1,0xf807,natural_ram[0xf807^1],1);
 if(type==1&&address==0x37656)put(11,(unsigned)*main_cycles,0,0,0,1);
 if(type==4&&address>=0xe00000){
  unsigned low=address&65535;
  if(low==0xf800)put(9,(unsigned)*main_cycles,(unsigned)width,address,value,1);
  if(low==0xf807)put(10,(unsigned)*main_cycles,(unsigned)width,address,value,1);
 }
}
