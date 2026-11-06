"""Frozen-trace causality control for a read-only barrier investigation."""
from pathlib import Path
import hashlib, importlib.util, json
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification/world-audio-barriers'
OUT.mkdir(parents=True,exist_ok=True)
protected=[ROOT/'scripts/yuyuhakusho/analyze_audio_phase_budget.py',ROOT/'docs/research/md-yuyuhakusho/audio-phase-budget/causality.json']
before={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in protected}
path=ROOT/'scripts/yuyuhakusho/probe_world_audio_barriers.py'
spec=importlib.util.spec_from_file_location('barrier_probe',path);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
result=module.investigate()
assert len(result['scenarios'])==2
for scenario in result['scenarios']:
    assert scenario['original_control']['frame_tail_counterexample']['frame']==375
    assert scenario['original_control']['battle_only_frame_tail_counterexample']['frame']==1813
    assert len(scenario['candidates'])==6
    assert all(c['independent_schedule_proved'] is False for c in scenario['candidates'])
assert before=={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in protected}
result['frozen_sources_unchanged']=before
result['producer_sha256']=hashlib.sha256(path.read_bytes()).hexdigest()
(OUT/'result.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'pass':True,'scenarios':[{'name':s['name'],'results':[(c['policy'],c['start_frame'],c.get('rejected'),c.get('frame'))for c in s['candidates']]}for s in result['scenarios']]}))
