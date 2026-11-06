package pvz_test

import (
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "reflect"
 "testing"
 "time"

 "github.com/k0ngk0ng/wire-board/games/pvz/engine"
 pvz "github.com/k0ngk0ng/wire-board/games/pvz/server"
 "github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func fullWorldArtifact(t *testing.T,v any) {t.Helper();out:=filepath.Join("..","..","..",".local/pvz/full-world");if e:=os.MkdirAll(out,0755);e!=nil {t.Fatal(e)};raw,_:=json.MarshalIndent(map[string]any{"test":t.Name(),"passed":!t.Failed(),"scope":"production module, scene/action contracts; controlled mechanism fixtures labelled separately","result":v},"","  ");if e:=os.WriteFile(filepath.Join(out,fmt.Sprintf("%x.json",[]byte(t.Name()))),raw,0600);e!=nil {t.Fatal(e)}}
func fullWorldMap(t *testing.T,v any)map[string]any{t.Helper();raw,_:=json.Marshal(v);var m map[string]any;if e:=json.Unmarshal(raw,&m);e!=nil {t.Fatal(e)};return m}
func TestPvZFullWorldCatalogAndScenesE2E(t *testing.T){
 c:=engine.GetCatalog();if len(c.Plants)!=49||len(c.Zombies)!=33 {t.Fatalf("full original catalog missing: plants=%d zombies=%d",len(c.Plants),len(c.Zombies))}
 m:=pvz.Module();for _,scene:=range []string{"day","night","pool","fog","roof","roofnight"}{for _,n:=range []int{2,4}{t.Run(fmt.Sprintf("%s-%d",scene,n),func(t *testing.T){raw:=json.RawMessage(fmt.Sprintf(`{"scene":%q}`,scene));o,e:=m.Configure(n,raw);if e!=nil {t.Fatal(e)};ctx:=gameapi.Context{Players:n,Actor:-1,Now:time.Unix(1800000000,0)};match,e:=m.Start(ctx,o);if e!=nil {t.Fatal(e)};view,e:=m.View(match,0);if e!=nil {t.Fatal(e)};v:=fullWorldMap(t,view);board:=v["map"].(map[string]any);rows:=float64(5);if scene=="pool"||scene=="fog" {rows=6};if board["id"]!=scene||board["rows"]!=rows||len(v["mowers"].([]any))!=int(rows){t.Fatal("scene geometry disagrees",board)};players:=v["players"].([]any);for _,player:=range players {deck:=player.(map[string]any)["deck"].([]any);if len(deck)==0||len(deck)>10 {t.Fatal("invalid scene default deck")}};if e=match.Validate(n);e!=nil {t.Fatal(e)};fullWorldArtifact(t,v)})}}
 for _,bad:=range []string{`{"scene":"unknown"}`,`{"scene":"night","money":999999}`,`{"scene":"pool","rows":5}`} {if _,e:=m.Configure(2,json.RawMessage(bad));e==nil {t.Fatal("invalid scene/options accepted",bad)}}
}

// Future public actions are encoded at the real module boundary, retaining a
// semantic red before the extended engine Action structure exists.
func TestPvZFullWorldDeckAndLayersE2E(t *testing.T){
 m:=pvz.Module();ctx:=gameapi.Context{Players:2,Actor:-1,Now:time.Unix(1800000000,0)}
 for _,scene:=range []string{"pool","roof","day"}{t.Run(scene,func(t *testing.T){raw:=json.RawMessage(fmt.Sprintf(`{"scene":%q}`,scene));match,e:=m.Start(ctx,raw);if e!=nil {t.Fatal(e)};view,_:=m.View(match,0);v:=fullWorldMap(t,view);epoch:=v["epoch"];seq:=0
  apply:=func(a map[string]any,wantErr bool){t.Helper();seq++;a["seq"]=seq;a["epoch"]=epoch;raw,_:=json.Marshal(a);c:=ctx;c.Actor=0;next,err:=m.Apply(c,match,raw);if (err!=nil)!=wantErr {t.Fatalf("scene=%s a=%v err=%v wantErr=%v",scene,a,err,wantErr)};if err==nil {match=next}}
  apply(map[string]any{"type":"deck","kinds":[]string{"sunflower","peashooter","lilypad","flowerpot","pumpkin","coffeebean","puffshroom","imitater","kernelpult","cobcannon"},"imitateKind":"peashooter"},false)
  apply(map[string]any{"type":"deck","kinds":[]string{"sunflower","sunflower"}},true)
  apply(map[string]any{"type":"deck","kinds":[]string{"imitater"},"imitateKind":"gatlingpea"},true)
  // Independent financial validation prevents an illegal placement spending.
  view,_=m.View(match,0);before:=fullWorldMap(t,view)["players"]
  row:=0;if scene=="pool" {row=2}
  if scene!="day" {apply(map[string]any{"type":"plant","kind":"peashooter","row":row,"col":0},true);view,_=m.View(match,0);if !reflect.DeepEqual(before,fullWorldMap(t,view)["players"]){t.Fatal("illegal terrain placement changed wallet")}}
  if scene=="pool" {apply(map[string]any{"type":"plant","kind":"lilypad","row":row,"col":0},false)}
  if scene=="roof" {apply(map[string]any{"type":"plant","kind":"flowerpot","row":row,"col":0},false)}
  apply(map[string]any{"type":"plant","kind":"peashooter","row":row,"col":0},false)
  view,_=m.View(match,0);v=fullWorldMap(t,view);plants:=v["plants"].([]any);want:=1;if scene!="day" {want=2};if len(plants)!=want {t.Fatal("support/body layers overwritten",plants)}
  fullWorldArtifact(t,v)
 })}
}

func TestPvZFullWorldFogProjectionE2E(t *testing.T){
 m:=pvz.Module();ctx:=gameapi.Context{Players:2,Actor:-1,Now:time.Unix(1800000000,0)};match,e:=m.Start(ctx,json.RawMessage(`{"scene":"fog","prepSeconds":10}`));if e!=nil {t.Fatal(e)}
 for range 200 {ctx.Now=ctx.Now.Add(50*time.Millisecond);match,e=m.OnTimer(ctx,match);if e!=nil {t.Fatal(e)}}
 view,_:=m.View(match,1);v:=fullWorldMap(t,view);a,_:=json.Marshal(map[string]any{"type":"spawn","kind":"zombie","row":0,"epoch":v["epoch"],"seq":1});ctx.Actor=1;match,e=m.Apply(ctx,match,a);if e!=nil {t.Fatal(e)}
 plantView,_:=m.View(match,0);zombieView,_:=m.View(match,1);p,z:=fullWorldMap(t,plantView),fullWorldMap(t,zombieView)
 if len(p["zombies"].([]any))!=0||len(z["zombies"].([]any))!=1 {t.Fatal("fog leaked hidden unit or hid owner's unit")}
 if p["wireMode"]!="projection" {t.Fatal("fog used whole-secret deterministic replay")}
 snapshot:=p["snapshot"].(map[string]any);if len(snapshot["zombies"].([]any))!=0||snapshot["projection"]!=true {t.Fatal("hidden unit leaked through recovery snapshot")}
 stream,e:=m.StreamView(match,0,"");if e!=nil {t.Fatal(e)};s:=fullWorldMap(t,stream.Data);if len(s["zombies"].([]any))!=0 {t.Fatal("hidden unit leaked through stream")}
 fullWorldArtifact(t,map[string]any{"plant":p,"zombie":z})
}
