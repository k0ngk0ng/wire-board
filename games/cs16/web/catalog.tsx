import { Cover } from './Cover';

export default {
  id: 'cs16', name: '反恐精英 1.6', description: '经典第一人称团队对抗。最多 10 人组成警匪两队，在沙漠、巷道与仓库完成爆破或人质营救。',
  category: 'realtime', minPlayers: 2, maxPlayers: 10, defaultPlayers: 10, playerCounts: [2, 4, 6, 8, 10], requiresFullTable: true, supportsBots: true,
  realtime: true, streamInputOnly: true, ownsSounds: true, Cover,
} as const;
