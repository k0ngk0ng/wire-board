export function mediaAssetsBase(value: string, origin: string): string {
  return /^https:\/\/cdn\.ichenj\.com(?:\/|$)/.test(value)
    ? origin + "/_media"
    : value;
}

export function codeMediaBase(value: string): string {
  return /^https:\/\/cdn\.ichenj\.com\/bga\/web\/[a-f0-9]{40}\/?$/.test(value) ? "/_media" : value;
}
