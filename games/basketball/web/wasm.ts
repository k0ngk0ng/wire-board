type GoInstance = { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): Promise<void> };
type RuntimeWindow = Window & typeof globalThis & {
  Go?: new () => GoInstance;
  basketballWasm?: (request: string) => string;
};
type Envelope<T> = { ok: boolean; data?: T; error?: { code: string; detail: string } };

export class BasketballEngineError extends Error {
  constructor(public code: string, message: string) { super(message); this.name='BasketballEngineError'; }
}
export interface BasketballWasm {
  request<T>(command: Readonly<Record<string, unknown>>): T;
}
const loaded = new Map<string,Promise<BasketballWasm>>();
let goScript: Promise<void> | undefined;
function ensureGo(url: string): Promise<void> {
  const runtime = window as RuntimeWindow;
  if (runtime.Go) return Promise.resolve();
  if (goScript) return goScript;
  goScript = new Promise<void>((resolve,reject) => {
    const script=document.createElement('script');script.src=url;
    script.onload=()=>runtime.Go ? resolve() : reject(new Error('Go Wasm support is unavailable'));
    script.onerror=()=>reject(new Error('Cannot load Go Wasm support'));
    document.head.append(script);
  }).catch(error=>{goScript=undefined;throw error;});
  return goScript;
}

/** One engine instance supports isolated sessions; no ROM or fixture is loaded. */
export function loadBasketballWasm(baseURL: string): Promise<BasketballWasm> {
  const base=new URL(baseURL.replace(/\/$/, '')+'/',location.href).href;
  const existing=loaded.get(base);if(existing)return existing;
  const pending=(async()=>{
    await ensureGo(new URL('wasm_exec.js',base).href);
    const runtime=window as RuntimeWindow;
    const go=new runtime.Go!();
    const response=await fetch(new URL('basketball.wasm',base));
    if(!response.ok)throw new Error('Cannot load basketball engine');
    const module=await WebAssembly.instantiate(await response.arrayBuffer(),go.importObject);
    let failure: unknown;
    void go.run(module.instance).catch(error=>{failure=error;});
    const call=runtime.basketballWasm;
    if(typeof call!=='function')throw new Error('Basketball engine did not initialize');
    return {request<T>(command: Readonly<Record<string,unknown>>): T {
      if(failure)throw new BasketballEngineError('runtime_stopped',String(failure));
      const reply=JSON.parse(call(JSON.stringify(command))) as Envelope<T>;
      if(!reply.ok)throw new BasketballEngineError(reply.error?.code??'engine_error',reply.error?.detail??'Basketball request failed');
      return reply.data as T;
    }};
  })().catch(error=>{loaded.delete(base);throw error;});
  loaded.set(base,pending);return pending;
}
