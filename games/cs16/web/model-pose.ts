export function carriedBoneMap(source: string[], target: string[]): number[] {
  return source.map(name => {
    const index = target.indexOf(name);
    if (index < 0 && name.startsWith('Bip01')) throw Error(`持枪模型缺少对应角色骨骼：${name}`);
    return index;
  });
}

export function playerPose(weapon: string, crouch: boolean): string {
  const groups: Record<string, string> = { ak47: 'ak47', galil: 'ak47', m4a1: 'rifle', awp: 'rifle', scout: 'rifle', sg550: 'rifle', aug: 'carbine', famas: 'carbine', p90: 'carbine', ump45: 'carbine', g3sg1: 'mp5', mp5navy: 'mp5', sg552: 'mp5', elite: 'dualpistols', m3: 'shotgun', xm1014: 'm249', m249: 'm249', knife: 'knife', hegrenade: 'grenade', flashbang: 'grenade', smokegrenade: 'grenade', c4: 'c4' };
  return `${crouch ? 'crouch' : 'ref'}_aim_${groups[weapon] ?? 'onehanded'}-blend-4`;
}
