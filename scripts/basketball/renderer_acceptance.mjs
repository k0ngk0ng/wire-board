import {loadBasketballAssets,BasketballRenderer} from '/renderer.js';
const image=src=>new Promise((resolve,reject)=>{const im=new Image();im.onload=()=>resolve(im);im.onerror=()=>reject(Error('Image failed: '+src));im.src=src});
const pixels=im=>{const c=document.createElement('canvas');c.width=im.width;c.height=im.height;const x=c.getContext('2d');x.drawImage(im,0,0);return x.getImageData(0,0,c.width,c.height).data};
export async function run(){
 const legacyOnly=new URLSearchParams(location.search).has('legacy-only');
 const assets=await loadBasketballAssets('/assets');const canvas=document.querySelector('#game');const renderer=new BasketballRenderer(canvas,assets);window.testRenderer=renderer;
 const fixture=await (await fetch('/cases.json')).json();const reports=[];let scenario='',firstFailure=null;
 for(const entry of fixture.cases){
  if(entry.scenario!==scenario){renderer.resetCourt();scenario=entry.scenario}
  renderer.commitCourtColumns(entry.courtCommits);
  const diagnostics=renderer.render(entry.snapshot);const want=pixels(await image(entry.reference));const actual=canvas.getContext('2d').getImageData(0,0,256,240).data;
  let mismatches=0,first=[];for(let i=0;i<actual.length;i+=4){if(actual[i]!==want[i]||actual[i+1]!==want[i+1]||actual[i+2]!==want[i+2]||actual[i+3]!==want[i+3]){mismatches++;if(first.length<8)first.push([i/4%256,Math.floor(i/4/256)])}}
  if(mismatches&&!firstFailure){firstFailure={scenario:entry.scenario,frame:entry.frame,mismatches,first};window.rendererFailurePNG=canvas.toDataURL();window.rendererFailureSnapshot=entry.snapshot}
  reports.push({scenario:entry.scenario,frame:entry.frame,mismatches,diagnostics});
 }
 const boundary=structuredClone(fixture.cases.at(-1).snapshot);renderer.render(boundary);const beforeResult=canvas.toDataURL();boundary.phase=6;renderer.render(boundary);if(canvas.toDataURL()!==beforeResult)throw Error('Unentered phase6 changes old court submission');
 const rejects=(snapshot,label)=>{let rejected=false;try{renderer.render(snapshot)}catch{rejected=true}if(!rejected)throw Error(label)};
 const entryBlank=structuredClone(boundary);entryBlank.drawOrder=[];entryBlank.objects.forEach(o=>o.pose=127);entryBlank.ballCHR=32;renderer.render(entryBlank);const blankPixels=canvas.toDataURL();entryBlank.ballCHR=13;renderer.render(entryBlank);if(canvas.toDataURL()!==blankPixels)throw Error('Entry CHR switch changes hidden display');
 const badCourtCHR=structuredClone(boundary);badCourtCHR.phase=3;badCourtCHR.ballCHR=13;rejects(badCourtCHR,'Gameplay accepts result CHR');
 const badEntryCHR=structuredClone(boundary);badEntryCHR.ballCHR=13;rejects(badEntryCHR,'Visible uninitialized result accepts result CHR');
 const missingResult=structuredClone(boundary);missingResult.objects[0].kind=17;rejects(missingResult,'Kind17 without initialized result accepted');
 const resultFixture=await (await fetch('/result-cases.json')).json();const resultReports=[],resultPaletteReports=[];const resultPaletteFixture=legacyOnly?{cases:[]}:await(await fetch('/result-palette-cases.json')).json();
 const misplacedResult=structuredClone(resultFixture.cases[0].snapshot);misplacedResult.phase=3;rejects(misplacedResult,'Result metadata outside phase6 accepted');
 const wrongResultCHR=structuredClone(resultFixture.cases[0].snapshot);wrongResultCHR.ballCHR=32;rejects(wrongResultCHR,'Result initialization CHR13 missing');
 for(const [dataset,reports] of [[resultFixture,resultReports],[resultPaletteFixture,resultPaletteReports]])for(const entry of dataset.cases){
  renderer.resetCourt(entry.snapshot.court);renderer.commitCourtColumns(entry.courtHistory);const diagnostics=renderer.render(entry.snapshot);const want=pixels(await image(entry.reference));const got=canvas.getContext('2d').getImageData(0,0,256,240).data;let mismatches=0,first=[];
  for(let i=0;i<got.length;i+=4)if(got[i]!==want[i]||got[i+1]!==want[i+1]||got[i+2]!==want[i+2]||got[i+3]!==want[i+3]){mismatches++;if(first.length<8)first.push([i/4%256,Math.floor(i/4/256)])}
  if(mismatches&&!firstFailure){firstFailure={scenario:'result',frame:entry.frame,mismatches,first};window.rendererFailurePNG=canvas.toDataURL();window.rendererFailureSnapshot=entry.snapshot}
  reports.push({frame:entry.frame,court:entry.snapshot.court,paletteID:entry.snapshot.paletteID,mismatches,diagnostics});
  if(entry.snapshot.court===1)window.resultScenePNG=canvas.toDataURL();else window.resultPaletteScenePNG=canvas.toDataURL();
 }
 if(resultPaletteFixture.cases.length){
  const sample=resultPaletteFixture.cases[0];renderer.resetCourt(sample.snapshot.court);renderer.commitCourtColumns(sample.courtHistory);renderer.render(sample.snapshot);const selected9=canvas.toDataURL();
  const alternate=structuredClone(sample.snapshot);alternate.paletteID=10;renderer.render(alternate);const selected10=canvas.toDataURL();alternate.paletteID=5;renderer.render(alternate);
  if(selected10===selected9||selected10===canvas.toDataURL())throw Error('Result palette10 silently reuses palette9 or base5');
 }
 const courtFixture=legacyOnly?{cases:[]}:await (await fetch('/court-cases.json')).json();const courtReports=[],irqReports=[];const irqFixture=legacyOnly?{cases:[]}:await(await fetch('/irq-cases.json')).json();window.courtScenePNGs={};
 for(const [dataset,reports] of [[courtFixture,courtReports],[irqFixture,irqReports]])for(const entry of dataset.cases){
  if('hudStartLine' in entry.snapshot)throw Error('Frame-specific raster hint substitutes for original split projection');
  renderer.resetCourt(entry.court);renderer.commitCourtEvents(entry.courtEvents);const diagnostics=renderer.render(entry.snapshot);const want=pixels(await image(entry.reference));const got=canvas.getContext('2d').getImageData(0,0,256,240).data;let mismatches=0,first=[];
  for(let i=0;i<got.length;i+=4)if(got[i]!==want[i]||got[i+1]!==want[i+1]||got[i+2]!==want[i+2]||got[i+3]!==want[i+3]){mismatches++;if(first.length<8)first.push([i/4%256,Math.floor(i/4/256)])}
  if(mismatches&&!firstFailure){firstFailure={scenario:'court'+entry.court,frame:entry.frame,mismatches,first};window.rendererFailurePNG=canvas.toDataURL();window.rendererFailureSnapshot=entry.snapshot}
  reports.push({court:entry.court,frame:entry.frame,mismatches,diagnostics});window.courtScenePNGs[entry.court]=canvas.toDataURL();
 }
 if(!legacyOnly){
  for(const entry of courtFixture.cases.filter(e=>e.court===3&&[1600,1680,1750].includes(e.frame)||e.court===4&&e.frame===1750)){
   const state=structuredClone(entry.snapshot);delete state.hudFetchTiming;renderer.resetCourt(entry.court);renderer.commitCourtEvents(entry.courtEvents);renderer.render(state);
   const want=pixels(await image(entry.reference)),got=canvas.getContext('2d').getImageData(0,0,256,240).data;
   if(got.some((v,i)=>v!==want[i]))throw Error('Default split failed the original four regressions');
  }
  const timed=irqFixture.cases.find(e=>e.court===3&&e.frame===1580);renderer.resetCourt(timed.court);renderer.commitCourtEvents(timed.courtEvents);renderer.render(timed.snapshot);const precise=canvas.toDataURL();
  const earlier=structuredClone(timed.snapshot);earlier.hudFetchTiming={addressResetFetch:5,fineScrollResetFetch:9};renderer.render(earlier);if(canvas.toDataURL()===precise)throw Error('Original CPU-derived fetch timing ignored');
  const invalid=structuredClone(timed.snapshot);invalid.hudFetchTiming={addressResetFetch:6,fineScrollResetFetch:4};rejects(invalid,'Reversed fetch timing accepted');
 }
 const fadeFixture=legacyOnly?{cases:[]}:await(await fetch('/fade-cases.json')).json(),fadeReports=[];window.fadeScenePNGs={};
 for(const entry of fadeFixture.cases){
  renderer.resetCourt(entry.snapshot.court);renderer.commitCourtEvents(entry.courtEvents);const diagnostics=renderer.render(entry.snapshot);const want=pixels(await image(entry.reference)),got=canvas.getContext('2d').getImageData(0,0,256,240).data;let mismatches=0,first=[];
  for(let i=0;i<got.length;i+=4)if(got[i]!==want[i]||got[i+1]!==want[i+1]||got[i+2]!==want[i+2]||got[i+3]!==want[i+3]){mismatches++;if(first.length<8)first.push([i/4%256,Math.floor(i/4/256)])}
  if(mismatches&&!firstFailure){firstFailure={scenario:entry.label,frame:entry.frame,level:entry.snapshot.paletteLevel,mismatches,first};window.rendererFailurePNG=canvas.toDataURL();window.rendererFailureSnapshot=entry.snapshot}
  fadeReports.push({label:entry.label,frame:entry.frame,court:entry.snapshot.court,paletteID:entry.snapshot.paletteID,level:entry.snapshot.paletteLevel,direction:entry.direction,result:!!entry.snapshot.result,mismatches,diagnostics});
  const key=[entry.snapshot.court,entry.snapshot.paletteID,entry.direction,entry.snapshot.paletteLevel,entry.snapshot.result?'result':'court'].join('-');window.fadeScenePNGs[key]=canvas.toDataURL();
 }
 renderer.resetCourt(1);const fadeContract=structuredClone(fixture.cases[0].snapshot);renderer.render(fadeContract);const fullColor=canvas.toDataURL();fadeContract.paletteLevel=0;renderer.render(fadeContract);
 if(canvas.getContext('2d').getImageData(0,0,256,240).data.some((v,i)=>i%4===3?v!==255:v!==0))throw Error('Explicit zero palette level is not original black');
 fadeContract.paletteLevel=2;renderer.render(fadeContract);const dimmed=canvas.toDataURL();renderer.render(fadeContract);if(canvas.toDataURL()!==dimmed)throw Error('Repeated frame applies fade twice');delete fadeContract.paletteLevel;renderer.render(fadeContract);if(canvas.toDataURL()!==fullColor)throw Error('Omitted palette level retains old fade');
 for(const level of [-1,5,1.5]){fadeContract.paletteLevel=level;rejects(fadeContract,'Invalid palette level accepted')}
 let invalidCourt=false;try{renderer.resetCourt(8)}catch{invalidCourt=true}if(!invalidCourt)throw Error('Invalid court accepted');
 if(courtFixture.cases.length){const mismatchCourt=structuredClone(courtFixture.cases.find(e=>e.court===0).snapshot);rejects(mismatchCourt,'Court mismatch silently changes ring');}

 renderer.resetCourt();for(const entry of fixture.cases.filter(c=>c.scenario===fixture.cases.at(-1).scenario))renderer.commitCourtColumns(entry.courtCommits);
 const ordered=structuredClone(fixture.cases.at(-1).snapshot);ordered.drawOrder=[];ordered.camera={x:0,y:296};renderer.resetCourt(1);
 const column={column:18,phase:1},patch={patch:{plane:1,row:8,column:18,tiles:[0]}};
 renderer.commitCourtEvents([column]);renderer.render(ordered);const columnPixels=canvas.toDataURL();
 renderer.commitCourtEvents([patch,column]);renderer.render(ordered);if(canvas.toDataURL()!==columnPixels)throw Error('Column refresh retained stale animated tile');
 renderer.commitCourtEvents([column,patch]);renderer.render(ordered);if(canvas.toDataURL()===columnPixels)throw Error('Animated tile after column was lost');const patchPixels=canvas.toDataURL();
 let invalidBatch=false;try{renderer.commitCourtEvents([column,{patch:{plane:1,row:8,column:31,tiles:[1,2]}}])}catch{invalidBatch=true}if(!invalidBatch)throw Error('Out-of-row court patch accepted');renderer.render(ordered);if(canvas.toDataURL()!==patchPixels)throw Error('Invalid patch batch partially mutated court');
 renderer.resetCourt();for(const entry of fixture.cases.filter(c=>c.scenario===fixture.cases.at(-1).scenario))renderer.commitCourtColumns(entry.courtCommits);
 const last=structuredClone(fixture.cases.at(-1).snapshot);last.hud.clock={minutes:1,seconds:23};last.hud.scores=[123,456];renderer.render(last);
 const actual=canvas.getContext('2d').getImageData(0,0,256,240).data;const glyphs=pixels(await image('/assets/court/hud-glyphs.png'));
 for(const [row,col,tile] of [[1,16,2],[1,18,3],[1,19,4],[3,12,2],[3,13,3],[3,14,4],[3,17,5],[3,18,6],[3,19,7]]){
  for(let y=0;y<8;y++)for(let x=0;x<8;x++){const screenY=191+row*8+y;const ai=(screenY*256+col*8+x)*4;const gi=((Math.floor(tile/16)*8+y)*128+(tile%16)*8+x)*4;for(let c=0;c<4;c++)if(actual[ai+c]!==glyphs[gi+c])throw Error('Semantic HUD digits differ at '+[row,col,x,y,c])}
 }
 const named=structuredClone(last);named.hud.clockLabel=[0xf2,0xf3,0,0];named.hud.panels[0].headerGlyphs=[0xfe,0xff,0xe5,0xe5,0xe5,0xe5,0xe5,0xe5,0xe5];renderer.render(named);
 const namePixels=canvas.getContext('2d').getImageData(0,0,256,240).data;
 for(const [row,col,tile] of [[0,2,0xfe],[0,3,0xff],[1,12,0xf2]])for(let y=0;y<8;y++)for(let x=0;x<8;x++){
  const sy=191+row*8+y;if(sy<194)continue;const a=(sy*256+col*8+x)*4,g=((Math.floor(tile/16)*8+y)*128+(tile%16)*8+x)*4;
  for(let c=0;c<4;c++)if(namePixels[a+c]!==glyphs[g+c])throw Error('Original name accent or period label ignored');
 }
 const overloaded=structuredClone(last);overloaded.drawOrder=Array.from({length:12},(_,i)=>i);overloaded.reverseSprites=true;
 overloaded.objects.forEach((o,i)=>{o.kind=0;o.character=0;o.pose=0;o.projected={x:16+i*18,y:70};o.airborne=false;o.shadowSuppressed=false;o.position.height=0});
 const capacity=renderer.render(overloaded);if(capacity.spritePieces!==64||!capacity.capacityReached)throw Error('Original 64-fragment capacity not respected');
 const capped=canvas.toDataURL();overloaded.drawOrder=overloaded.drawOrder.slice(0,8);renderer.render(overloaded);if(canvas.toDataURL()!==capped)throw Error('Sprite-capacity prefix changes pixels');
 const bankCase=structuredClone(last);bankCase.drawOrder=[6];bankCase.objects[6].pose=4;bankCase.objects[6].projected={x:128,y:80};bankCase.objects[6].airborne=false;bankCase.objects[6].shadowSuppressed=true;
 bankCase.ballCHR=32;renderer.render(bankCase);const oldBank=canvas.toDataURL();bankCase.ballCHR=35;renderer.render(bankCase);if(canvas.toDataURL()===oldBank)throw Error('Selected display ball CHR ignored in favor of current pose');
 const overflowFrames=reports.filter(r=>r.diagnostics.overflowingScanlines>0).length;if(overflowFrames===0)throw Error('Original scanline-overflow coverage disappeared');
 let invalidRejected=false;const bad=structuredClone(last);bad.objects[0].pose=999;try{renderer.render(bad)}catch{invalidRejected=true}if(!invalidRejected)throw Error('Invalid semantic pose accepted');renderer.render(last);
 return {status:firstFailure?'failed':'passed',scope:fixture.scope,frames:reports.length,pixelsCompared:reports.length*256*240,firstFailure,semanticHudDigits:true,originalNameAccents:true,invalidPoseRejected:true,displayBallCHRIndependent:true,spriteCapacityVerified:true,originalOverflowFrames:overflowFrames,resultBoundaryVerified:true,orderedCourtEventsVerified:true,courtFrames:courtReports.length,courtPixelsCompared:courtReports.length*256*240,courtFramesDetail:courtReports,irqTimingVerified:!legacyOnly,defaultHudSplitFrames:legacyOnly?0:4,irqHoldoutFrames:irqReports.length,irqHoldoutPixelsCompared:irqReports.length*256*240,irqHoldoutFramesDetail:irqReports,resultFrames:resultReports.length,resultPixelsCompared:resultReports.length*256*240,resultFramesDetail:resultReports,paletteFadeContractVerified:true,fadeFrames:fadeReports.length,fadePixelsCompared:fadeReports.length*256*240,fadeFramesDetail:fadeReports,resultPaletteVariantsVerified:!legacyOnly,resultPaletteFrames:resultPaletteReports.length,resultPalettePixelsCompared:resultPaletteReports.length*256*240,resultPaletteFramesDetail:resultPaletteReports,framesDetail:reports};
}
