/* Research-only passive sample-IRQ prefix ledger. Original code does all work. */
#include <stddef.h>
#include <stdint.h>
typedef unsigned (*get_fn)(unsigned);
typedef struct{uint32_t kind,pc,width,address,value,cycle,refresh,line,vertical,counter,reg0,reg1,reload,hpending,vpending,batch,phase,flags,control_word,saved_color,ports0,ports1,held0,held1,held2,held3,held4,held5,held6,held7,pin,mask;} event_t;
static const uint8_t *ram,*reg,*hc,*hp,*vp;static const uint16_t*vc;static const uint32_t*line,*batch;static const int32_t*cycle,*refresh;static const uint16_t*pads;static const uint8_t*systems;
static const uint32_t*irq_pin,*irq_mask;static event_t events[8192];static unsigned count,active,done,overflow,last_pc;static get_fn get;
static unsigned word(unsigned off){return *(const uint16_t*)(ram+off);}
static void put(unsigned kind,unsigned pc,unsigned width,unsigned address,unsigned value){if(count==8192){overflow=1;return;}events[count++]=(event_t){kind,pc,width,address,value,(unsigned)*cycle,(unsigned)*refresh,*line,*vc,*hc,reg[0],reg[1],reg[10],*hp,*vp,*batch,word(0xf516),ram[0xf500^1],word(0xf6d2),word(0xf140),systems[0]==13?4:1,systems[1]==13?4:1,pads[0],pads[1],pads[2],pads[3],pads[4],pads[5],pads[6],pads[7],*irq_pin>>8,*irq_mask>>8};}
int battle_prefix_attach(void*r,void*g,void*c,void*f,void*l,void*v,void*h,void*ih,void*iv,void*b,void*p,void*gr,void*ip,void*im){if(!r||!g||!c||!f||!l||!v||!h||!ih||!iv||!b||!p||!gr||!ip||!im)return 0;ram=r;reg=g;cycle=c;refresh=f;line=l;vc=v;hc=h;hp=ih;vp=iv;batch=b;systems=p;pads=(const uint16_t*)((const uint8_t*)p+10);get=(get_fn)gr;irq_pin=ip;irq_mask=im;return 1;}
void battle_prefix_begin(void){count=active=done=overflow=last_pc=0;}
void battle_prefix_hook(int type,int width,unsigned address,unsigned value){unsigned p=address&0xffffff;if(type==1){last_pc=p;if(!active&&!done&&p==0xfff000&&word(0xf516)==0){active=1;put(9,p,0,0,0);}if(active){if(p==0x219ac){put(10,p,0,0,0);active=0;done=1;}else put(1,p,0,0,0);}}else if(active&&(type==2||type==4)){if((p>=0xc00000&&p<0xe00000)||((p&0xfffff0)==0xa10000))put((unsigned)type,last_pc,(unsigned)width,p,value);}}
unsigned battle_prefix_count(void){return count;}
unsigned battle_prefix_complete(void){return done&&!active&&!overflow;}
const event_t*battle_prefix_events(void){return events;}
unsigned battle_prefix_size(void){return sizeof(event_t);}
