export function formatGameNumber(value: number, language = 8): string {
  const separator = language === 2 ? " " : [0, 6, 7].includes(language) ? "," : ".";
  return String(value).replace(/[0-9]{4,}/g, digits => {
    const groups: string[] = [];
    for (let end = digits.length; end > 0; end -= 3) groups.unshift(digits.slice(Math.max(0, end - 3), end));
    return groups.join(separator);
  });
}

export function formatGameGold(value: number, language = 8): string {
  return value >= 99999999999 ? value.toExponential(2) : formatGameNumber(value, language);
}
