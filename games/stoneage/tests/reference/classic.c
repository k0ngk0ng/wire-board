/* Extracted classic 2.5 primitives; see source.json. Test oracle only.
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
float BATTLE_adjustRidePet3A( int charaindex, int petindex, int workindex, int action )
{
	float	ret = CHAR_getWorkInt( charaindex, workindex );
	BOOL	throwweapon = BATTLE_IsThrowWepon(CHAR_getItemIndex( charaindex, CHAR_ARM ) );

	switch( workindex )	{
	case CHAR_WORKATTACKPOWER : // 攻击力
		if( throwweapon ) // 投掷
			ret = CHAR_getWorkInt( charaindex, CHAR_WORKATTACKPOWER )
#ifdef _BATTLE_NEWPOWER //andy 1/24 reEdit
			+ CHAR_getWorkInt( petindex, CHAR_WORKATTACKPOWER )*0.4;
#else
			+ CHAR_getWorkInt( petindex, CHAR_WORKATTACKPOWER )*0.2;
#endif
		else // 近战
			ret = CHAR_getWorkInt( charaindex, CHAR_WORKATTACKPOWER )*0.8
				+ CHAR_getWorkInt( petindex, CHAR_WORKATTACKPOWER )*0.8;
		break;
	case CHAR_WORKDEFENCEPOWER : // 防御力
		ret = CHAR_getWorkInt( charaindex, CHAR_WORKDEFENCEPOWER )*0.7
			+ CHAR_getWorkInt( petindex, CHAR_WORKDEFENCEPOWER )*0.7;
		break;
	case CHAR_WORKQUICK : // 敏捷力
		if( action == ATTACKSIDE )	{
			if( throwweapon )	{
				ret = CHAR_getWorkInt( charaindex, CHAR_WORKQUICK )*0.8
					+ CHAR_getWorkInt( petindex, CHAR_WORKQUICK )*0.2;
			}else	{
#ifdef _BATTLE_NEWPOWER //andy 1/24 reEdit
				ret = CHAR_getWorkInt( charaindex, CHAR_WORKQUICK )*0.2
					+ CHAR_getWorkInt( petindex, CHAR_WORKQUICK )*0.8;

#else
				ret = CHAR_getWorkInt( charaindex, CHAR_WORKQUICK )*0.5
					+ CHAR_getWorkInt( petindex, CHAR_WORKQUICK )*0.5;
#endif
			}
		}else
			if( action == DEFFENCESIDE ){
			ret = CHAR_getWorkInt( charaindex, CHAR_WORKQUICK )*0.1
				+ CHAR_getWorkInt( petindex, CHAR_WORKQUICK )*0.9;
		}
		break;
	default :
		ret = CHAR_getWorkInt( charaindex, workindex );
	}

	//print(" adj3A_Now:%d:%d=%f ", charaindex, workindex, ret);

	return	ret;
}
static int	BATTLE_AttrCalc(
		int My_Fire,
		int My_Water,
		int My_Earth,
		int My_Wind,
		int My_None,
		int Vs_Fire,
		int Vs_Water,
		int Vs_Earth,
		int Vs_Wind,
		int Vs_None
)
{
	int iRet = 0;
	My_Fire = My_Fire * Vs_None * AJ_UP
			+ My_Fire * Vs_Fire * AJ_SAME
			+ My_Fire * Vs_Water * AJ_DOWN
			+ My_Fire * Vs_Earth * AJ_SAME
			+ My_Fire * Vs_Wind * AJ_UP;
	My_Water = My_Water * Vs_None * AJ_UP
			+ My_Water * Vs_Fire * AJ_UP
			+ My_Water * Vs_Water * AJ_SAME
			+ My_Water * Vs_Earth * AJ_DOWN
			+ My_Water * Vs_Wind * AJ_SAME;

	My_Earth = My_Earth * Vs_None * AJ_UP
			+ My_Earth  * Vs_Fire * AJ_SAME
			+ My_Earth * Vs_Water * AJ_UP
			+ My_Earth * Vs_Earth * AJ_SAME
			+ My_Earth * Vs_Wind * AJ_DOWN;

	My_Wind = My_Wind * Vs_None * AJ_UP
			+ My_Wind  * Vs_Fire * AJ_DOWN
			+ My_Wind * Vs_Water * AJ_SAME
			+ My_Wind * Vs_Earth * AJ_UP
			+ My_Wind * Vs_Wind * AJ_SAME;

	My_None = My_None * Vs_None * AJ_SAME
			+ My_None * Vs_Fire * AJ_DOWN
			+ My_None * Vs_Water * AJ_DOWN
			+ My_None * Vs_Earth * AJ_DOWN
			+ My_None * Vs_Wind * AJ_DOWN;

	iRet = (My_Fire + My_Water + My_Earth + My_Wind + My_None) ;
	return (iRet * D_ATTR);
}

static int BATTLE_AttrAdjust(int a,int d,int damage){
 (void)a;(void)d;
 /* Attacker is 100 Earth; defender is Earth, Water or Wind. */
 return BATTLE_AttrCalc(0,0,damage*100,0,0,
   0,element==1?100:0,element==0?100:0,element==2?100:0,0);
}
int BATTLE_DamageCalc( int attackindex, int defindex )
{
	float attack, defense;
	int damage = 0;
	int attackpet = BATTLE_getRidePet( attackindex );
	int defpet = BATTLE_getRidePet( defindex );

	// Robin 0727 Ride Pet
	if( attackpet == -1 )	{
		attack = CHAR_getWorkInt( attackindex, CHAR_WORKATTACKPOWER );
	}else	{
		attack = BATTLE_adjustRidePet3A( attackindex, attackpet, CHAR_WORKATTACKPOWER, ATTACKSIDE );
	}
		if( defpet == -1 )	{
#ifdef _BATTLE_NEWPOWER
			defense = CHAR_getWorkInt( defindex, CHAR_WORKDEFENCEPOWER ) * 0.70;
#else
			defense = CHAR_getWorkInt( defindex, CHAR_WORKDEFENCEPOWER ) * 0.45;
			defense += CHAR_getWorkInt( defindex, CHAR_WORKQUICK ) * 0.2;
			defense += CHAR_getWorkInt( defindex, CHAR_WORKFIXVITAL ) * 0.1;
#endif
		}else	{
#ifdef _BATTLE_NEWPOWER
			defense = BATTLE_adjustRidePet3A( defindex, defpet, CHAR_WORKDEFENCEPOWER, DEFFENCESIDE ) * 0.70;
#else
			defense = BATTLE_adjustRidePet3A( defindex, defpet, CHAR_WORKDEFENCEPOWER, DEFFENCESIDE ) * 0.45;
			defense += BATTLE_adjustRidePet3A( defindex, defpet, CHAR_WORKQUICK, DEFFENCESIDE ) * 0.20;
			defense += CHAR_getWorkInt( defindex, CHAR_WORKFIXVITAL ) * 0.05;
#endif
		}

#ifdef _MAGIC_SUPERWALL //铁壁防御
		if( CHAR_getWorkInt( defindex, CHAR_MAGICSUPERWALL ) > 0 ){
			float def = (float)(CHAR_getWorkInt( defindex, CHAR_OTHERSTATUSNUMS ));
			def = (def + rand()%20)/100;
			defense += defense * def;
		}
#endif
#ifdef _NPCENEMY_ADDPOWER //修改怪物的能力值
		if( CHAR_getInt( defindex, CHAR_WHICHTYPE ) == CHAR_TYPEENEMY )	{
			defense += (defense*(rand()%10)+2)/100;
		}
		if( CHAR_getInt( attackindex, CHAR_WHICHTYPE ) == CHAR_TYPEENEMY )	{
			attack += (attack*(rand()%10)+2)/100;
		}
#endif
	if( CHAR_getWorkInt( defindex, CHAR_WORKSTONE ) > 0 ) defense *= 2.0;

#ifdef _PETSKILL_REGRET
	//无装备防御
	if( CHAR_getWorkInt( attackindex, CHAR_WORKBATTLECOM1 ) == BATTLE_COM_S_REGRET
		|| CHAR_getWorkInt( attackindex, CHAR_WORKBATTLECOM1 ) == BATTLE_COM_S_REGRET2 )
		defense = CHAR_getWorkInt(defindex,CHAR_WORKFIXTOUGH);
#endif

#ifdef _EQUIT_NEGLECTGUARD //忽视目标防御力%
	if( CHAR_getWorkInt(  attackindex, CHAR_WORKNEGLECTGUARD) > 1 ) {
		float defp = (float)CHAR_getWorkInt(  attackindex, CHAR_WORKNEGLECTGUARD);
		defp = 1 - (defp/100);
		defense = defense*defp;
	}
#endif

	if( defense <= attack && attack < (defense * 8.0/7.0)   ){
		damage = (int)(RAND( 0, attack * D_16 ));
	}else
	if( defense > attack ){
		damage = (int)(RAND( 0, 1 ));
	}else
	if( attack >= (defense * 8/7)   ){
		float K0;
		K0 = RAND( 0, attack*D_8 ) - attack*D_16;
		damage = (int)(( (attack - defense )*DAMAGE_RATE ) + K0);
	}

	//四属性
	damage = BATTLE_AttrAdjust( attackindex, defindex, damage );
#ifdef _ADD_DEAMGEDEFC //额外伤害 & 附加攻击
	{
		int apower, dpower, otherpower;
		apower = CHAR_getWorkInt( attackindex, CHAR_WORKOTHERDMAGE);
		dpower = CHAR_getWorkInt( defindex, CHAR_WORKOTHERDEFC);

		otherpower = RAND( apower*0.3, apower) - RAND( dpower*0.3, dpower);
		if( otherpower != 0 )	{
			damage += otherpower;
		}
	}
	if( damage < 0 )
		damage = 0;
#endif

	return damage;
}

int main(int argc,char **argv){
 if(argc!=18)return 2;
 for(int i=0;i<12;i++)units[i/3][i%3]=atoi(argv[i+1]);
 am=atoi(argv[13]);dm=atoi(argv[14]);throwing=atoi(argv[15]);
 fraction=atoi(argv[16])/10000.0;element=atoi(argv[17]);
 printf("{\"damage\":%d}\n",BATTLE_DamageCalc(0,1));return 0;
}
