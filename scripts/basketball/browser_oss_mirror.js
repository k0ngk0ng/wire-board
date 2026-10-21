// Test-only loopback mirror of verified immutable OSS bytes. This does not enter
// the application bundle; production URLs and integrity verification are retained.
const basketballFixtureFetch = window.fetch;
window.fetch = (input, options) => {
  const url = String(input);
  return basketballFixtureFetch(url.startsWith('https://cdn.ichenj.com/')
    ? '/__oss/' + new URL(url).pathname.slice(1) : input, options);
};
