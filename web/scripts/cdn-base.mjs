export function assetBase(value) {
  if (!value || value === "/") return "/";
  if (!/^https:\/\/cdn\.ichenj\.com\/bga\/web\/[a-f0-9]{40}\/$/.test(value))
    throw new Error(
      "Expected an immutable CDN base bound to a full commit SHA",
    );
  return value;
}
export function assetURL(base, file) {
  return base + file.replace(/^\//, "");
}
