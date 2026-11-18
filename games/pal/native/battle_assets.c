#include "main.h"

static uint32_t battle_assets[16];

static void add_battle_asset(unsigned group, unsigned id) {
    if (id < 256) {
        battle_assets[group * 8 + id / 32] |= UINT32_C(1) << (id % 32);
    }
}

static void add_magic_assets(unsigned object, BOOL player) {
    if (!object || object >= MAX_OBJECTS) return;
    unsigned number = gpGlobals->g.rgObject[object].magic.wMagicNumber;
    if (number >= gpGlobals->g.nMagic) return;
    const MAGIC *magic = &gpGlobals->g.lprgMagic[number];
    if (player && magic->wType == kMagicTypeSummon) {
        add_battle_asset(0, magic->rgSpecific.wSummonEffect + 10u);
        if (magic->wEffect >= gpGlobals->g.nMagic) return;
        magic = &gpGlobals->g.lprgMagic[magic->wEffect];
    }
    add_battle_asset(1, magic->wEffect);
}

unsigned wb_battle_assets(void) {
    memset(battle_assets, 0, sizeof battle_assets);
    if (!gpGlobals->fInBattle) return (unsigned)battle_assets;
    for (unsigned i = 0; i <= gpGlobals->wMaxPartyMemberIndex && i < MAX_PLAYERS_IN_PARTY; i++) {
        unsigned role = gpGlobals->rgParty[i].wPlayerRole;
        if (role >= MAX_PLAYER_ROLES) continue;
        for (unsigned slot = 0; slot < MAX_PLAYER_MAGICS; slot++) {
            add_magic_assets(gpGlobals->g.PlayerRoles.rgwMagic[slot][role], TRUE);
        }
        if (gpGlobals->wMaxPartyMemberIndex > 0) {
            add_magic_assets(PAL_GetPlayerCooperativeMagic(role), TRUE);
        }
    }
    for (unsigned i = 0; i <= g_Battle.wMaxEnemyIndex && i < MAX_ENEMIES_IN_TEAM; i++) {
        const BATTLEENEMY *enemy = &g_Battle.rgEnemy[i];
        if (enemy->wObjectID && enemy->e.wMagicRate) add_magic_assets(enemy->e.wMagic, FALSE);
    }
    return (unsigned)battle_assets;
}
