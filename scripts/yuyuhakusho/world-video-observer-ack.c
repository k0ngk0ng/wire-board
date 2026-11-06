/* Research-only delegated native IRQ acknowledge. No core source is changed. */
#include <limits.h>
#include "world-video-observer.c"
typedef unsigned int uint;
typedef int sint;
typedef uint8_t uint8;
typedef uint16_t uint16;
typedef uint32_t uint32;
#include "m68k.h"

static m68ki_cpu_core *native_cpu;
static int (*saved_ack)(int);
static uint32_t previous_target=UINT32_MAX,previous_line=UINT32_MAX,previous_frame=UINT32_MAX;

static void batch_record(void){
 if(previous_target!=native_cpu->cycle_end||previous_line!=*line_cycle||previous_frame!=current_frame){
  append(snapshot(),11,last_pc,native_cpu->int_level>>8,native_cpu->int_mask>>8,native_cpu->cycle_end);
  previous_target=native_cpu->cycle_end;previous_line=*line_cycle;previous_frame=current_frame;
 }
}
static int delegated_ack(int level){
 batch_record();
 append(snapshot(),7,last_pc,native_cpu->int_level>>8,native_cpu->int_mask>>8,(unsigned)level);
 int value=saved_ack(level);
 append(snapshot(),10,last_pc,native_cpu->int_level>>8,native_cpu->int_mask>>8,(unsigned)level);
 return value;
}
int world_video_attach_ack(void *cpu,void *expected){
 if(saved_ack||!cpu||!expected)return 0;
 native_cpu=(m68ki_cpu_core*)cpu;
 if((const int32_t*)&native_cpu->cycles!=cpu_cycle||native_cpu->pc!=get_reg(16)||
    native_cpu->int_ack_callback!=(int(*)(int))expected)return 0;
 saved_ack=native_cpu->int_ack_callback;native_cpu->int_ack_callback=delegated_ack;
 previous_target=previous_line=previous_frame=UINT32_MAX;
 return 1;
}
int world_video_detach_ack(void){
 if(!saved_ack||native_cpu->int_ack_callback!=delegated_ack)return 0;
 native_cpu->int_ack_callback=saved_ack;saved_ack=0;native_cpu=0;return 1;
}
void world_video_hook_ack(int type,int width,unsigned address,unsigned value){
 if(!active)return;
 batch_record();world_video_hook(type,width,address,value);
}
