// SPDX-License-Identifier: GPL-3.0-only
// Retain original offscreen draws as a bounded immutable surface graph. Arena
// collection uses all live surfaces; export contains only screen dependencies.
#include "main.h"
#include "wb_raster.h"
#include "wb_rng.h"
#include "wb_video.h"

#define NODES 16384
#define SURFACES 32
typedef struct { int v[16]; } RasterNode;
typedef struct { SDL_Surface *surface; int root; } RasterSurface;
static RasterNode nodes[NODES+1];
static RasterSurface surfaces[SURFACES];
static int enabled,cursor=1,used,rng_base,display_active;
static unsigned char marked[NODES+1];
static int work[NODES*2+4],mapped[NODES+1];
static int output[3+NODES*16];

static void invalid(const char *message){TerminateOnError("native raster: %s",message);}
int wb_raster_active(void){return enabled;}
void wb_raster_reset(void){enabled=0;used=0;cursor=1;rng_base=0;display_active=0;wb_raster_seed_clear();memset(surfaces,0,sizeof surfaces);memset(nodes,0,sizeof nodes);}

static void collect(int a,int b){
 memset(marked,0,sizeof marked);int top=0;
 if(a)work[top++]=a;if(b)work[top++]=b;
 for(int i=0;i<SURFACES;i++)if(surfaces[i].surface&&surfaces[i].root)work[top++]=surfaces[i].root;
 while(top){
  int id=work[--top];if(id<1||id>NODES)invalid("dependency exceeds arena");
  if(marked[id])continue;marked[id]=1;
  if(nodes[id].v[1])work[top++]=nodes[id].v[1];
  if(nodes[id].v[2])work[top++]=nodes[id].v[2];
  if(top>=NODES*2)invalid("dependency traversal exceeds limit");
 }
 used=0;
 for(int i=1;i<=NODES;i++){if(marked[i])used++;else memset(&nodes[i],0,sizeof nodes[i]);}
 cursor=1;
}
static int add(int kind,int a,int b,int w,int h,const int *params,int count){
 if(w<1||h<1||w>4096||h>4096||count>11)invalid("surface or operation exceeds limit");
 if(used==NODES)collect(a,b);
 if(used==NODES)invalid("live surface graph exceeds limit");
 while(nodes[cursor].v[3]){cursor++;if(cursor>NODES)cursor=1;}
 int id=cursor++;if(cursor>NODES)cursor=1;
 RasterNode *n=&nodes[id];memset(n,0,sizeof *n);
 n->v[0]=kind;n->v[1]=a;n->v[2]=b;n->v[3]=w;n->v[4]=h;
 if(count)memcpy(n->v+5,params,count*sizeof(int));used++;return id;
}
static RasterSurface *surface(SDL_Surface *s){
 if(!s)invalid("missing surface");
 for(int i=0;i<SURFACES;i++)if(surfaces[i].surface==s)return &surfaces[i];
 for(int i=0;i<SURFACES;i++)if(!surfaces[i].surface){
  surfaces[i].surface=s;surfaces[i].root=add(0,0,0,s->w,s->h,NULL,0);return &surfaces[i];
 }
 invalid("live surfaces exceed limit");return NULL;
}
void wb_raster_enable(void){
 if(enabled)return;
 wb_raster_reset();enabled=1;surface(gpScreen);surface(gpScreenBak);
 if(!gpGlobals->fInBattle&&wb_raster_capture_seed()){
  int screen[]={0},backup[]={1};
  surface(gpScreen)->root=add(9,0,0,320,200,screen,1);
  surface(gpScreenBak)->root=add(9,0,0,320,200,backup,1);
 }
}
void wb_raster_begin_display(int partial,int physical){
 if(partial)return;
 if(physical)wb_raster_enable();
 display_active=physical;
 if(enabled)surface(gpScreenReal)->root=add(0,0,0,320,200,NULL,0);
}
void wb_raster_replace_index(SDL_Surface *s,int from,int to){
 if(from<0||from>255||to<0||to>255)invalid("invalid palette index replacement");
 wb_raster_enable();
 RasterSurface *dst=surface(s);int p[]={from,to};
 dst->root=add(11,dst->root,0,s->w,s->h,p,2);
}
void wb_raster_rng_begin(void){wb_raster_enable();rng_base=surface(gpScreen)->root;}
void wb_raster_rng_frame(int clip,int first,int frame){int p[]={clip,first,frame};surface(gpScreen)->root=add(10,rng_base,0,320,200,p,3);}
void wb_raster_surface_free(SDL_Surface *s){
 for(int i=0;i<SURFACES;i++)if(surfaces[i].surface==s)memset(&surfaces[i],0,sizeof surfaces[i]);
}
void wb_raster_image(SDL_Surface *s,int source,int id,int frame,int x,int y,int mode,int color,int shift){
 if(!enabled)return;RasterSurface *dst=surface(s);int p[]={source,id,frame,x,y,mode,color,shift};
 // FBP is an opaque full-screen asset, so it replaces the preceding surface.
 int parent=source==4&&x==0&&y==0&&s->w==320&&s->h==200?0:dst->root;
 dst->root=add(1,parent,0,s->w,s->h,p,8);
}
void wb_raster_text(SDL_Surface *s,int code,int x,int y,int color,int small){
 if(!enabled)return;RasterSurface *dst=surface(s);int p[]={code,x,y,color,small};
 dst->root=add(2,dst->root,0,s->w,s->h,p,5);
}
void wb_raster_copy(SDL_Surface *src,SDL_Surface *dst,int sx,int sy,int dx,int dy,int w,int h){
 if(!enabled||w<=0||h<=0)return;
 RasterSurface *from=surface(src),*to=surface(dst);
 if(!sx&&!sy&&!dx&&!dy&&w==src->w&&h==src->h&&src->w==dst->w&&src->h==dst->h){to->root=from->root;return;}
 int p[]={sx,sy,dx,dy,w,h};to->root=add(3,from->root,to->root,dst->w,dst->h,p,6);
}
void wb_raster_fill(SDL_Surface *s,int x,int y,int w,int h,int color){
 if(!enabled||w<=0||h<=0)return;RasterSurface *dst=surface(s);int p[]={x,y,w,h,color};
 if(x==0&&y==0&&w==s->w&&h==s->h){int solid[]={color};dst->root=add(0,0,0,s->w,s->h,solid,1);}
 else dst->root=add(4,dst->root,0,s->w,s->h,p,5);
}
void wb_raster_stretch(SDL_Surface *src,SDL_Surface *dst,int sx,int sy,int sw,int sh,int dx,int dy,int dw,int dh){
 if(!enabled||sw<=0||sh<=0||dw<=0||dh<=0)return;
 RasterSurface *from=surface(src),*to=surface(dst);int p[]={sx,sy,sw,sh,dx,dy,dw,dh};
 to->root=add(8,from->root,to->root,dst->w,dst->h,p,8);
}
void wb_raster_background(void){
 if(!enabled)return;RasterSurface *from=surface(g_Battle.lpBackground),*to=surface(g_Battle.lpSceneBuf);
 int p[]={g_Battle.sBackgroundColorShift};to->root=add(5,from->root,0,320,200,p,1);
}
void wb_raster_wave(SDL_Surface *s,int level,int phase){
 if(!enabled)return;RasterSurface *dst=surface(s);int p[]={level,phase};
 if(s->w!=320||s->h!=200)invalid("original wave requires 320x200");
 dst->root=add(6,dst->root,0,320,200,p,2);
}
void wb_raster_blend_surfaces(SDL_Surface *src,SDL_Surface *dst,int pass,int component){
 if(!enabled)return;RasterSurface *from=surface(src),*to=surface(dst);
 int p[]={pass,component};to->root=add(7,from->root,to->root,320,200,p,2);
}
void wb_raster_blend(int pass,int component){wb_raster_blend_surfaces(g_Battle.lpSceneBuf,gpScreenBak,pass,component);}
unsigned wb_raster_graph(void){
 output[0]=1;output[1]=output[2]=0;if(!enabled)return (unsigned)output;
 int root=surface(display_active?gpScreenReal:gpScreen)->root,top=0,count=0;memset(marked,0,sizeof marked);memset(mapped,0,sizeof mapped);
 work[top++]=root;
 while(top){
  int id=work[top-1];RasterNode *n=&nodes[id];
  if(!marked[id]){
   marked[id]=1;
   if(n->v[2]&&!marked[n->v[2]])work[top++]=n->v[2];
   if(n->v[1]&&!marked[n->v[1]])work[top++]=n->v[1];
   if(top>=NODES*2)invalid("export traversal exceeds limit");
  }else{
   top--;
   if(marked[id]==2)continue;
   if((n->v[1]&&!mapped[n->v[1]])||(n->v[2]&&!mapped[n->v[2]]))invalid("cyclic surface dependency");
   int *dst=output+3+count*16;memcpy(dst,n->v,16*sizeof(int));
   dst[1]=mapped[n->v[1]];dst[2]=mapped[n->v[2]];mapped[id]=++count;marked[id]=2;
  }
 }
 output[1]=count;output[2]=mapped[root];return (unsigned)output;
}
