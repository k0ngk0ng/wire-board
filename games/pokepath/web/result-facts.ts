export const resultBoss = [
  "shaymin", "celebi", "lunala", "moltres", "regirock", "groudon", "registeel", "regice", "regigigas",
  "zapdos", "hooh", "articuno", "kyogre", "thundurus", "tapuFini", "nihilego", "tapuKoko", "tapuBulu",
  "tapuLele", "keldeo", "manaphy", "heatran", "landorus", "volcanion", "pheromosa", "cobalion", "tornadus",
  "stakataka", "terrakion", "magearna", "mew", "victini", "marshadow", "sirfetchd", "diancie", "calyrex",
  "zamazenta", "zacian", "latios", "virizion", "meloetta", "cresselia", "solgaleo", "regidrago", "regieleki",
  "reshiram", "spectrier"
] as const;

const victoryText = [
  "You defeated {boss} and showed who's boss, congratulations!",
  "Has derrotado a {boss} y le has demostrado quién manda",
  "Vous avez vaincu {boss} et montré qui est le patron,",
  "Você derrotou {boss} e mostrou quem manda, parabéns!",
  "Hai sconfitto {boss} e hai dimostrato chi comanda",
  "Du hast {boss} besiegt und gezeigt, wer der Boss ist",
  "{boss}を倒してボスの座を見せつけた、祝福します！",
  "{boss}를 물리치고 누가 보스인지 보여줬다, 축하해!",
  "你击败了{boss}，向他展示了谁才是老大，恭喜！",
  "Pokonałeś {boss} i pokazałeś, kto tu rządzi — gratulacje!"
];

const retryText = [
  "You can use retry to return to wave {wave} with {lives} lives",
  "Puedes reintentar y volver a la oleada {wave} con {lives} vidas",
  "Vous pouvez réessayer et revenir à la vague {wave} avec {lives} vies",
  "Você pode reiniciar e voltar à onda {wave} com {lives} vidas",
  "Puoi ritentare e tornare all’ondata {wave} con {lives} vite",
  "Du kannst mit {lives} Leben zu Welle {wave} zurück.",
  "リトライを使うと、{lives}ライフの状態でウェーブ{wave}に戻れます",
  "재시도를 사용하면 목숨 {lives}개로 {wave}웨이브로 돌아갈 수 있습니다",
  "你可以使用重试返回到第{wave}波，并拥有{lives}条生命",
  "Możesz użyć ponownej próby, aby wrócić do fali {wave} z {lives} życiami"
];

export function routeVictoryMessage(language: number, boss: string) {
  return (victoryText[language] ?? victoryText[0]).replace("{boss}", () => boss);
}

export function routeRetryMessage(language: number, wave: number, lives: number) {
  return (retryText[language] ?? retryText[0]).replace("{wave}", String(wave)).replace("{lives}", String(lives));
}
