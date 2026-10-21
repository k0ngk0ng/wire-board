import { loadBasketballAssetsWithFallback } from "../../games/basketball/web/renderer";

(async()=>{
  const cases=[];
  for(const base of ["/valid","/missing","/corrupt"]){
    const assets=await loadBasketballAssetsWithFallback(base,"/local");
    cases.push({base,courts:assets.courts.length,resultCourts:assets.results.courts.length,poses:assets.results.actorProfiles.length});
  }
  let rejectsUnavailable=false;
  try{await loadBasketballAssetsWithFallback("/missing","/missing-local")}catch{rejectsUnavailable=true}
  (window as any).result={cases,rejectsUnavailable};
  document.body.textContent=JSON.stringify((window as any).result,null,2);
})().catch(error=>{(window as any).result={error:String(error)}});
