import type { GameAudioPort } from '../../../web/src/game-api/module';
import type { ControllerProfile } from '../../../web/src/controllers/input';
import { BasketballInput, type BasketballButton } from './input';
import { BasketballAudio, type BasketballAudioOptions } from './audio';
import { BasketballRenderer, loadBasketballAssets, type BasketballRenderSnapshot, type CourtColumnCommit, type CourtTilePatch } from './renderer';
import { loadBasketballWasm, type BasketballWasm } from './wasm';

export interface BasketballRuntimeConfig {
  match: { Seed: number; [field: string]: unknown };
  humanTactics: [number,number];
}
export interface BasketballRuntimeState {
  videoFrames: number;
  logicSteps: number;
  opening?: unknown;
  playing?: { match: { Phase: number; Period: number }; [field: string]: unknown };
  transition?: unknown;
  /** The single-match session parks after confirmation. The original
   * campaign/menu destination is metadata for the host, not an executed route. */
  outcome?: { winningTeam: number; scores: [number,number]; confirmed: boolean; nextScene: number; nextPhase: number };
  awaitingNextPeriod: boolean;
  display: {
    HasVisible: boolean;
    Visible: BasketballRenderSnapshot;
    CourtColumns: CourtColumnCommit[] | null;
    CourtPatches?: CourtTilePatch[] | null;
    CourtEpoch: number;
    CourtSerial: number;
  };
  [field: string]: unknown;
}
export interface BasketballRuntimeView {
  session: number;
  state: BasketballRuntimeState;
  soundIDs?: number[];
  logicUpdates: number;
}
export interface BasketballCheckpoint { version: number; state: BasketballRuntimeState }
export interface BasketballClientOptions {
  root: HTMLElement;
  canvas: HTMLCanvasElement;
  profile: ControllerProfile;
  runtimeURL: string;
  assetsURL: string;
  audio: GameAudioPort;
  audioOptions?: BasketballAudioOptions;
  config?: BasketballRuntimeConfig;
  /** Omitting a seed creates a nonzero seed once; snapshots retain it. */
  seed?: number;
  controller?: 0|1|2|3;
  onView?: (view: BasketballRuntimeView) => void;
}

/** Reusable local Go/Wasm client. The host supplies video ticks and platform
 * preferences/audio; this class never decides movement, scores, AI or winners.
 */
export class BasketballClient {
  private input: BasketballInput;
  private sound: BasketballAudio;
  private current: BasketballRuntimeView;
  private closed=false;
  private courtKey='';
  private imageKey='';

  static async create(options: BasketballClientOptions): Promise<BasketballClient> {
    const [engine,assets]=await Promise.all([loadBasketballWasm(options.runtimeURL),loadBasketballAssets(options.assetsURL)]);
    const config=options.config ? structuredClone(options.config) : engine.request<BasketballRuntimeConfig>({op:'defaults'});
    if(options.seed !== undefined)config.match.Seed=options.seed;
    else if(!options.config){const seed=crypto.getRandomValues(new Uint32Array(1))[0];config.match.Seed=seed||1;}
    const renderer=new BasketballRenderer(options.canvas,assets);
    return new BasketballClient(engine,renderer,config,options);
  }

  private constructor(private engine:BasketballWasm, private renderer:BasketballRenderer, config:BasketballRuntimeConfig, private options:BasketballClientOptions) {
    this.input=new BasketballInput(options.profile,options.root,mask=>this.cancelRuntimeButtons(mask));
    this.sound=new BasketballAudio(options.audio,options.audioOptions);
    try{this.current=engine.request<BasketballRuntimeView>({op:'create',config});this.present();}
    catch(error){this.input.dispose();this.sound.dispose();throw error;}
  }

  get view(): BasketballRuntimeView { return this.current; }
  readonly controllerSink = {
    pressPointer:(id:number,buttons:readonly string[])=>{
      this.ensureOpen();this.options.root.focus({preventScroll:true});this.input.pressPointer(id,buttons);
    },
    releasePointer:(id:number,cancelled:boolean)=>{
      if(!this.closed)this.input.releasePointer(id,cancelled);
    },
  };
  configure(profile:ControllerProfile){this.ensureOpen();this.input.configure(profile);}
  cancelInput(){this.input.cancel();}
  private cancelRuntimeButtons(mask:number){
    if(this.closed||!this.current)return;
    this.current=this.engine.request<BasketballRuntimeView>({op:'cancelInput',session:this.current.session,controller:this.options.controller??0,buttons:mask});
    this.options.onView?.(this.current);
  }
  bindButton(element:HTMLElement,button:string):()=>void {
    if(!['up','down','left','right','a','b','start','select'].includes(button))throw new Error('Unknown basketball control');
    return this.input.bindButton(element,[button as BasketballButton]);
  }

  advanceVideo(ticks=1): BasketballRuntimeView {
    this.ensureOpen();
    if(!Number.isInteger(ticks)||ticks<1||ticks>120)throw new Error('Advance 1 through 120 video ticks at a time');
    for(let index=0;index<ticks;index++){
      const held=[0,0,0,0];held[this.options.controller??0]=this.input.sample();
      const next=this.engine.request<BasketballRuntimeView>({op:'advance',session:this.current.session,ticks:1,held});
      this.current=next;
      if(next.soundIDs?.length)void this.sound.consume(next.soundIDs,next.state.videoFrames);
      this.present();
    }
    this.options.onView?.(this.current);
    return this.current;
  }

  checkpoint(): BasketballCheckpoint {
    this.ensureOpen();
    return this.engine.request<{checkpoint:BasketballCheckpoint}>({op:'snapshot',session:this.current.session}).checkpoint;
  }

  restore(checkpoint:unknown):BasketballRuntimeView {
    this.ensureOpen();
    const restored=this.engine.request<BasketballRuntimeView>({op:'restore',checkpoint});
    this.engine.request({op:'dispose',session:this.current.session});
    this.input.cancelLocal();this.sound.reset(restored.state.videoFrames);
    this.current=restored;this.courtKey='';this.imageKey='';this.present();
    this.options.onView?.(restored);
    return restored;
  }

  private present(){
    const display=this.current.state.display;
    if(!display.HasVisible){
      this.options.canvas.getContext('2d')?.clearRect(0,0,256,240);this.imageKey='';return;
    }
    const key=`${display.Visible.court}:${display.CourtEpoch}:${display.CourtSerial}`;
    if(key!==this.courtKey){
      // Go compacts columns and drops tile patches overwritten by newer columns.
      // Replay the remaining patches after columns; this is retained display state.
      this.renderer.resetCourt(display.Visible.court);
      this.renderer.commitCourtEvents([...(display.CourtColumns??[]),...(display.CourtPatches??[]).map(patch=>({patch}))]);
      this.courtKey=key;this.imageKey='';
    }
    const image=JSON.stringify(display.Visible);
    if(image!==this.imageKey){this.renderer.render(display.Visible);this.imageKey=image;}
  }

  private ensureOpen(){if(this.closed)throw new Error('Basketball client is closed');}
  dispose(){
    if(this.closed)return;
    this.closed=true;this.input.dispose();this.sound.dispose();
    this.engine.request({op:'dispose',session:this.current.session});
  }
}
