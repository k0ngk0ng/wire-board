export const mapGuide = [
  { id: "no-talk", name: "无声行动", description: "紧凑的公寓房间与窄走廊相连，家具和转角限制直线射击，近身绕侧的机会较多。", recommendedPlayers: "2 至 4" },
  { id: "full-house", name: "满屋杀机", description: "多个房间围绕中央交叉走廊分布，可以从不同门口切入，注意走廊两端和侧面交火。", recommendedPlayers: "2 至 4" },
  { id: "push-it", name: "霓虹夜店", description: "中央夜店区域与两侧狭长通路相接，开阔区和窄口并存，换线时需要兼顾远枪与近战。", recommendedPlayers: "2 至 4" },
  { id: "overdose", name: "过量", description: "红、蓝地板房间围绕中间通路，门口密集，侧翼房间相互连接，适合从相邻门口绕侧。", recommendedPlayers: "2 至 4" },
  { id: "tension", name: "紧绷", description: "多组房间分成左右区域，黄色走道连接中央与上下侧房，家具提供绕行掩体，转线距离较长。", recommendedPlayers: "2 至 4" },
  { id: "neighbors", name: "邻居", description: "纵向走廊串接几处房间，通路较窄、门较少，拐角交火突出，换位时注意走廊尽头。", recommendedPlayers: "2 至 4" },
  { id: "highball", name: "高球", description: "左侧餐桌与卡座连接中间包房、卫生间，右侧是并排保龄球道，长通道与小隔间形成不同交火距离。", recommendedPlayers: "4" },
  { id: "exposed", name: "暴露", description: "横向展开的宽幅展馆，围栏分隔展位、街机与展台，纵横通道连接各区，远距离转线空间较大。", recommendedPlayers: "4" },
  { id: "hot-heavy", name: "火热行动", description: "浴场中轴长廊连接浴池、桑拿房与喷泉庭院，直线通道和两侧房间交错，留意侧门与狭口。", recommendedPlayers: "4" },
] as const;

export const characterGuide = [
  { id: "jacket", name: "鸡头主角", description: "鸡头面具与夹克，原版主角的标志性造型。" },
  { id: "biker", name: "骑士", description: "原版蓝盔骑士造型，头盔轮廓醒目。" },
  { id: "bodyguard", name: "女保镖", description: "原版持刀保镖造型，保留长发与近身战斗的视觉风格。" },
  { id: "father", name: "黑帮首领", description: "原版黑帮头目的西装与面部造型，轮廓和夹克主角明显不同。" },
  { id: "richter", name: "鼠面刺客", description: "鼠面具与刺客装束，原版关键敌对角色的造型。" },
  { id: "producer", name: "制片人", description: "原版制片人 Boss 的魁梧轮廓，体型外观与瘦身形角色区别明显。" },
  { id: "beard", name: "胡子", description: "保留原版商店人物的胡须与装束，呈现熟悉的店员形象。" },
  { id: "hobo", name: "流浪汉", description: "原版街头流浪汉造型，以胡须和破旧衣着的轮廓辨认。" },
] as const;

export const weaponGuide = [
  { id: "pistol", name: "手枪", group: "gun", description: "单颗弹丸较精准，射击间隔适中，适合先瞄准再开枪。" },
  { id: "uzi", name: "Uzi 冲锋枪", group: "gun", description: "连射最快、散布较大，适合短距离压制，长按很快耗尽弹药。" },
  { id: "m16", name: "M16 步枪", group: "gun", description: "射程较远，散布比 Uzi 小，适合走廊和中远距离交火。" },
  { id: "shotgun", name: "霰弹枪", group: "gun", description: "一发多颗弹丸，近距离覆盖面大；射击间隔长，空档需要借助掩体。" },
  { id: "knife", name: "小刀", group: "melee", description: "攻击快、距离短，需要贴近并朝向对手，挥击不消耗弹药。" },
  { id: "bat", name: "棒球棒", group: "melee", description: "比小刀够得更远，挥击间隔更长，适合门口与转角争夺。" },
  { id: "katana", name: "武士刀", group: "melee", description: "攻击范围较长，节奏比棒球棒快，仍需要正面距离与畅通路线。" },
  { id: "magnum", name: "马格南手枪", group: "gun", description: "单颗弹丸更精准、射程长，但射击节奏慢，初始弹药也较少。" },
  { id: "mp5", name: "MP5 冲锋枪", group: "gun", description: "连射比 Uzi 稍慢，散布更集中、射程更远，适合较稳的持续压制。" },
  { id: "double-barrel", name: "双管霰弹枪", group: "gun", description: "初始仅两发，每发弹丸更多且更分散，射程较短，适合近距离封门。" },
  { id: "axe", name: "斧头", group: "melee", description: "近战中攻击范围最长，挥击间隔也最长，出手后需留意侧面接近的对手。" },
  { id: "machete", name: "砍刀", group: "melee", description: "攻击范围和速度介于小刀与武士刀之间，适合房间内短距离追击。" },
] as const;

export const characterNames: Record<string, string> = Object.fromEntries(characterGuide.map(character => [character.id, character.name]));
export const weaponNames: Record<string, string> = Object.fromEntries([...weaponGuide.map(weapon => [weapon.id, weapon.name]), ["", "徒手"]]);
export const mapNames: Record<string, string> = Object.fromEntries(mapGuide.map(map => [map.id, map.name]));

export function isMeleeWeapon(id: string) {
  return id === "" || weaponGuide.some(weapon => weapon.id === id && weapon.group === "melee");
}
