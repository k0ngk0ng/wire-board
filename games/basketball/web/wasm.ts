import { fetchLoadingBytes, type LoadingProgress } from "./loading";
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
const progressStates = new Map<string,LoadingProgress>();
const listeners = new Map<string,Set<(p:LoadingProgress)=>void>>();
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
export function loadBasketballWasm(baseURL: string, onProgress: (p:LoadingProgress)=>void = ()=>{}): Promise<BasketballWasm> {
  const base=new URL(baseURL.replace(/\/$/, '')+'/',location.href).href;
  const subscribers=listeners.get(base) ?? new Set();listeners.set(base,subscribers);subscribers.add(onProgress);
  onProgress(progressStates.get(base) ?? {percent:0,label:"加载游戏程序"});
  const report=(p:LoadingProgress)=>{progressStates.set(base,p);for(const cb of subscribers)cb(p)};
  const subscribe=(promise:Promise<BasketballWasm>)=>promise.finally(()=>subscribers.delete(onProgress));
  const existing=loaded.get(base);if(existing)return subscribe(existing);
  const pending=(async()=>{
    await ensureGo(new URL('wasm_exec.js',base).href);
    const runtime=window as RuntimeWindow;
    const go=new runtime.Go!();
    const descriptor=await fetch(new URL('build.json',base),{signal:AbortSignal.timeout(45000)});
    if(!descriptor.ok)throw Error('无法加载游戏程序版本');
    const {wasm}=await descriptor.json() as {wasm:{url:string;sha256:string;bytes:number}};
    if(!wasm || wasm.url !== `https://cdn.ichenj.com/bga/basketball/runtime-${wasm.sha256}/basketball.wasm` || !/^[a-f0-9]{64}$/.test(wasm.sha256) || !Number.isSafeInteger(wasm.bytes) || wasm.bytes<=0)throw Error('游戏程序版本无效');
    const bytes=await fetchLoadingBytes(wasm.url,p=>report({percent:p.total ? Math.min(90,p.loaded/p.total*90) : 0,label:'下载游戏程序'}),wasm);
    report({percent:95,label:'初始化游戏程序'});
    const module=await WebAssembly.instantiate(bytes,go.importObject);
    let failure: unknown;
    void go.run(module.instance).catch(error=>{failure=error;});
    const call=runtime.basketballWasm;
    if(typeof call!=='function')throw new Error('Basketball engine did not initialize');
    report({percent:100,label:'游戏程序已就绪'});
    return {request<T>(command: Readonly<Record<string,unknown>>): T {
      if(failure)throw new BasketballEngineError('runtime_stopped',String(failure));
      const reply=JSON.parse(call(JSON.stringify(command))) as Envelope<T>;
      if(!reply.ok)throw new BasketballEngineError(reply.error?.code??'engine_error',reply.error?.detail??'Basketball request failed');
      return reply.data as T;
    }};
  })().catch(error=>{loaded.delete(base);progressStates.delete(base);throw error;});
  loaded.set(base,pending);return subscribe(pending);
}
