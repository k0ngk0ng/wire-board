export function mediaAssetsBase(value: string, origin: string): string {
  if (/^https:\/\/[^/]+\/bga\/?$/.test(value)) return origin + "/bga";
  return /^https:\/\/cdn\.ichenj\.com\/bga\/web\/[a-f0-9]{40}\/?$/.test(value)
    ? origin + "/_media"
    : value;
}

export function codeMediaBase(value: string): string {
  if (/^https:\/\/[^/]+\/bga\/?$/.test(value)) return "/bga";
  return /^https:\/\/cdn\.ichenj\.com\/bga\/web\/[a-f0-9]{40}\/?$/.test(value) ? "/_media" : value;
}
