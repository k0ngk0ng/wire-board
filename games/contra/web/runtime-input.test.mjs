// Failure list before fix: platform fullscreen focuses the board wrapper,
// which is outside the game's inner root; normal keyboard movement is rejected.
// Allowing arbitrary ancestors instead would activate controls from room UI.
import test from 'node:test';import assert from 'node:assert/strict';
import {build} from '../../../web/node_modules/esbuild/lib/main.js';
const built=await build({entryPoints:['games/contra/web/runtime-input.ts'],bundle:true,write:false,format:'esm'});
const {RuntimeInput}=await import('data:text/javascript;base64,'+Buffer.from(built.outputFiles[0].text).toString('base64'));
class Element {constructor(kind){this.kind=kind}closest(){return null}contains(other){return this.kind==='fullscreen'&&other.kind==='root'}matches(selector){return this.kind==='fullscreen'&&selector==='[data-fullscreen-board]'}}
test('fullscreen focus accepts keyboard but unrelated public room controls stay blocked',()=>{
 globalThis.HTMLElement=Element;const listeners=new Map();globalThis.window={addEventListener:(n,f)=>listeners.set(n,f),removeEventListener(){}};globalThis.document={hidden:false,body:new Element('body'),querySelector(){return null},addEventListener(){},removeEventListener(){}};
 const root=new Element('root'),input=new RuntimeInput({keyboard:{right:'KeyD'},turboHz:12},root,()=>{});
 const press=target=>listeners.get('keydown')({code:'KeyD',target,preventDefault(){}});
 press(new Element('fullscreen'));assert.equal(input.sample(performance.now()),1);
 input.cancel();press(new Element('room'));assert.equal(input.sample(performance.now()),0);input.dispose();
});
