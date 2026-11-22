import { Cover } from './Cover';
import { mergeCS16StreamData } from './stream-delta';

export default {
  id: 'cs16', name: '反恐精英', description: '经典第一人称团队对抗。2 至 10 人组成警匪两队，在六张经典地图进行爆破、人质营救或回合歼灭。',
  category: 'realtime', minPlayers: 2, maxPlayers: 10, defaultPlayers: 10, playerCounts: [2, 3, 4, 5, 6, 7, 8, 9, 10], supportsBots: true,
  realtime: true, streamInputOnly: true, mergeStreamData: mergeCS16StreamData, ownsSounds: true, Cover,
} as const;
