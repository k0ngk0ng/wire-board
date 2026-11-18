export const trajectorySource = `package main

import (
  "encoding/json"
  "fmt"
  "os"
  "github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

type operation struct {
  Type string \`json:"type"\`
  Actor int \`json:"actor"\`
  Action *engine.Action \`json:"action,omitempty"\`
  Milliseconds int64 \`json:"milliseconds,omitempty"\`
}
type checkpoint struct {
  Name string \`json:"name"\`
  After int \`json:"after"\`
  State json.RawMessage \`json:"state"\`
  View engine.View \`json:"view"\`
}
func main() {
  const now int64 = 1730000000000
  const players = 3
  options := engine.Options{Starter:"charmander"}
  var seed uint64
  for index:=uint64(1);index<=256;index++ {
    candidate:=index*2654435761+12345
    state,err:=engine.New(options,players,candidate,now)
    if err!=nil { panic(err) }
    if err=engine.Apply(state,1,engine.Action{Type:"egg",Epoch:state.Epoch});err!=nil { panic(err) }
    if len(state.Team)>1 && state.Team[1].Species=="mudkip" { seed=candidate;break }
  }
  if seed==0 { panic("deterministic first egg seed not found") }
  state,err:=engine.New(options,players,seed,now)
  if err!=nil { panic(err) }
  operations:=[]operation{}
  checkpoints:=[]checkpoint{}
  capture:=func(name string) {
    raw,err:=engine.Marshal(state)
    if err!=nil { panic(err) }
    checkpoints=append(checkpoints,checkpoint{Name:name,After:len(operations),State:raw,View:engine.Project(state)})
  }
  capture("initial")
  apply:=func(actor int,action engine.Action) {
    action.Epoch=state.Epoch
    if err:=engine.Apply(state,actor,action);err!=nil { panic(err) }
    value:=action
    operations=append(operations,operation{Type:"action",Actor:actor,Action:&value})
  }
  apply(1,engine.Action{Type:"egg"})
  capture("natural-second-partner")
  kinds:=map[string]bool{}
  firstFlight:=false
  upgrades:=0
  maximumTowers:=0
  firstComplete:=false
  elapsed:=int64(0)
  for ;elapsed<600000;elapsed+=100 {
    if action:=engine.Propose(state);action!=nil {
      apply(int(elapsed/100)%players,*action)
      if action.Type=="upgrade" { upgrades++;if upgrades==1 { capture("paid-level-up") } }
      if len(state.Towers)>maximumTowers { maximumTowers=len(state.Towers);if maximumTowers<=3 { capture(fmt.Sprintf("deployed-%d-towers",maximumTowers)) } }
    }
    if err:=engine.Advance(state,100);err!=nil { panic(err) }
    operations=append(operations,operation{Type:"tick",Milliseconds:100})
    if !firstFlight && len(state.Projectiles)>0 { firstFlight=true;capture("projectile-in-flight") }
    for _,enemy:=range state.Enemies {
      for _,condition:=range enemy.Conditions {
        if !kinds[condition.Kind] { kinds[condition.Kind]=true;capture("condition-"+condition.Kind) }
      }
    }
    if !firstComplete && state.Records[0]>0 { firstComplete=true;capture("first-wave-completed") }
    if state.Records[0]>=5 && maximumTowers>=3 && upgrades>0 && firstFlight && len(kinds)>=2 { break }
  }
  if state.Records[0]<5 || maximumTowers<3 || upgrades==0 || !firstFlight || len(kinds)<2 { panic(fmt.Sprintf("incomplete coverage: waves=%d towers=%d upgrades=%d flight=%v kinds=%v",state.Records[0],maximumTowers,upgrades,firstFlight,kinds)) }
  capture("final")
  result:=map[string]any{"options":options,"players":players,"seed":seed,"now":now,"operations":operations,"checkpoints":checkpoints,"coverage":map[string]any{"waves":state.Records[0],"maximumTowers":maximumTowers,"upgrades":upgrades,"conditions":kinds,"inFlight":firstFlight,"elapsedMS":elapsed}}
  if err:=json.NewEncoder(os.Stdout).Encode(result);err!=nil { panic(err) }
}
`;
