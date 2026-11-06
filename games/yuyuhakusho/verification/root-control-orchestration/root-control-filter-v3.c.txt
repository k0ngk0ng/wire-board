/* Passive original observer: selected instruction boundaries plus every actual
 * M68K byte store to the PSG port. The original memory handler still runs. */
#include "hook_filter.c"
static unsigned root_control_last_pc;
void root_control_cpu_hook(int type,int width,unsigned address,unsigned value){
 if(type==1){root_control_last_pc=address;yyh_cpu_hook(type,width,address,value);}
 else if(type==4&&width==1&&(address&0xffffff)==0xc00011&&observer)observer(type,width,root_control_last_pc,value);
}
