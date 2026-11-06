/* Passive composition of existing note and ordinary request recorders. */
#include "score-clock-note-voice-observer.c"
static read_fn irq_native_read;
static write_fn irq_native_write;
static unsigned irq_read(unsigned address){
 unsigned before=*main_cycles,value=irq_native_read(address),field=address&0xffff;
 if(pc>=0x37d66&&pc<=0x380a6&&(field==0x1ffc||field==0x1f87||field==0x1fa7||field==0x1f86||field==0x1fa6))put(25,before,1,address,value,1);
 return value;
}
static void irq_write(unsigned address,unsigned value){
 unsigned before=*main_cycles;irq_native_write(address,value);unsigned field=address&0xffff;
 if(pc>=0x37d66&&pc<=0x380a6&&((field>=0x1f80&&field<=0x1f87)||(field>=0x1fa0&&field<=0x1fa7)||(field>=0x1fe6&&field<=0x1fe9)||field==0x1ffd||field==0x1ffe||field==0x1ffb||field==0x1ffa))put(26,before,1,address,value,1);
}
int score_clock_irq_enable(void){
 if(irq_native_read||!score_clock_note_enable())return 0;
 irq_native_read=zram_read;irq_native_write=zram_write;zram_read=irq_read;zram_write=irq_write;return 1;
}
int score_clock_irq_disable(void){
 if(!irq_native_read||zram_read!=irq_read||zram_write!=irq_write)return 0;
 zram_read=irq_native_read;zram_write=irq_native_write;irq_native_read=0;irq_native_write=0;return score_clock_note_disable();
}
