import { BasketballInput } from '../../games/basketball/web/input';
import type { ControllerProfile } from '../../web/src/controllers/input';

const profile: ControllerProfile = { revision: 0, turboHz: 10, keyboard: { up:'KeyW',down:'KeyS',left:'KeyA',right:'KeyD',select:'KeyF',start:'Space',b:'KeyJ',a:'KeyK',turboB:'KeyU',turboA:'KeyI' } };
const root = document.querySelector<HTMLElement>('#game')!;
const cancelled:number[]=[];
const input = new BasketballInput(profile, root,mask=>cancelled.push(mask));
input.bindButton(document.querySelector<HTMLButtonElement>('#right')!, ['right']);
input.bindButton(document.querySelector<HTMLButtonElement>('#a')!, ['a']);
Object.assign(window, {
  inputAcceptance: {
    sample: (at?: number) => input.sample(at),
    configureRight: () => input.configure({...profile, keyboard: {...profile.keyboard,right:'ArrowRight'}}),
    reset: () => input.configure(profile),
    enable: (value: boolean) => input.setEnabled(value),
    dispose: () => input.dispose(),
    cancel: () => input.cancel(),
    cancellations: () => [...cancelled],
    clearCancellations: () => { cancelled.length=0; },
    pointer: (id: number, buttons: ('a'|'right'|'turboA'|'turboB')[], at=0) => input.pressPointer(id,buttons,at),
    releasePointer: (id: number, cancelled = false) => input.releasePointer(id,cancelled),
    // Synthetic edges supplement native browser key presses for time-window
    // assertions; the E2E driver separately uses native press/click actions.
    key: (type: string, code: string, repeat = false) => root.dispatchEvent(new KeyboardEvent(type,{bubbles:true,code,repeat,cancelable:true})),
  },
});
root.focus();
