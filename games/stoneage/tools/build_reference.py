#!/usr/bin/env python3
"""Freeze only classic C combat primitives for the independent E2E oracle."""
import argparse
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[3]
parser=argparse.ArgumentParser()
parser.add_argument('--source',type=Path,required=True)
args=parser.parse_args()
path=args.source/'server/legacy/source/2.5/gmsv/battle/battle_event.c'
body=path.read_bytes().decode('gb18030',errors='replace')
def extract(signature):
 start=body.index(signature);opening=body.index('{',start);depth=1;end=opening+1
 while depth:
  depth+=(body[end]=='{')-(body[end]=='}');end+=1
 return body[start:end]
functions=[extract(s) for s in ['float BATTLE_adjustRidePet3A(', 'static int\tBATTLE_AttrCalc(', 'int BATTLE_DamageCalc(']]
header=r'''/* Extracted classic 2.5 primitives; see source.json. Test oracle only.
 * Explicitly fixes _BATTLE_NEWPOWER and neutral field; random fractional
 * inputs are controlled by the test instead of depending on platform libc. */
#include <stdio.h>
#include <stdlib.h>
#define _BATTLE_NEWPOWER
#define BOOL int
#define ATTACKSIDE 0
#define DEFFENCESIDE 1
#define CHAR_ARM 0
#define AJ_SAME 1.0
#define AJ_UP 1.5
#define AJ_DOWN .6
#define D_ATTR .0001
#define D_16 (1.0/16)
#define D_8 (1.0/8)
#define DAMAGE_RATE 2.0
enum { CHAR_WORKATTACKPOWER,CHAR_WORKDEFENCEPOWER,CHAR_WORKQUICK,CHAR_WORKSTONE };
static int units[4][4], am, dm, throwing, element;
static double fraction;
#define RAND(x,y) ((x-1)+1+(int)((double)(y-(x-1))*fraction))
static int CHAR_getWorkInt(int i,int f){return units[i][f];}
static int CHAR_getItemIndex(int i,int f){(void)i;(void)f;return throwing;}
static int BATTLE_IsThrowWepon(int i){return i;}
static int BATTLE_getRidePet(int i){return (i==0?am:dm)?i+2:-1;}
'''
attribute=r'''
static int BATTLE_AttrAdjust(int a,int d,int damage){
 (void)a;(void)d;
 /* Attacker is 100 Earth; defender is Earth, Water or Wind. */
 return BATTLE_AttrCalc(0,0,damage*100,0,0,
   0,element==1?100:0,element==0?100:0,element==2?100:0,0);
}
'''
driver=r'''
int main(int argc,char **argv){
 if(argc!=18)return 2;
 for(int i=0;i<12;i++)units[i/3][i%3]=atoi(argv[i+1]);
 am=atoi(argv[13]);dm=atoi(argv[14]);throwing=atoi(argv[15]);
 fraction=atoi(argv[16])/10000.0;element=atoi(argv[17]);
 printf("{\"damage\":%d}\n",BATTLE_DamageCalc(0,1));return 0;
}
'''
out=ROOT/'games/stoneage/tests/reference'
out.mkdir(parents=True,exist_ok=True)
code=header+functions[0]+'\n'+functions[1]+'\n'+attribute+functions[2]+'\n'+driver
(out/'classic.c').write_text('\n'.join(line.rstrip() for line in code.splitlines())+'\n')
(out/'source.json').write_text(json.dumps({'source':'stoneage/server/legacy/source/2.5/gmsv/battle/battle_event.c','sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'functions':[s.split('(')[0].strip() for s in functions],'scope':'original active _BATTLE_NEWPOWER branch, physical base damage, rider coefficients and elemental adjustment; not full-engine parity'},indent=2)+'\n')
print(out.relative_to(ROOT))
