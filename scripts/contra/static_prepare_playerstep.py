#!/usr/bin/env python3
"""Prepare explicit Japanese player-step entries for a private REA session."""
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-static'
ENTRIES={
 '0xd2d9':'players_frame','0xd356':'scroll_other_player','0xd38a':'actor_frame','0xd45e':'normal_player',
 '0xd495':'normal_motion','0xd4a1':'auto_scroll_player','0xd518':'state_and_x','0xd525':'apply_x',
 '0xd595':'action_priority','0xd5fd':'guarded_jump','0xd626':'ground_dpad','0xd68a':'positive_x',
 '0xd68e':'negative_x','0xd690':'set_x_with_water_gate','0xd6a1':'scene_kind','0xd6ad':'edge_fall',
 '0xd6dd':'jump_motion','0xd735':'air_direction','0xd746':'direction_code_x','0xd752':'air_dpad_priority',
 '0xd79d':'land','0xd7be':'clear_motion','0xd825':'snap_landing_y','0xd848':'aim','0xd8b0':'ledge',
 '0xd8d9':'feet_collision','0xd941':'camera_gate','0xda5e':'side_collision',
 '0xfa12':'fire_bank_bridge','0xfa4a':'sprite_bank_bridge',
}

def main():
    spec=json.loads((BASE/'fixed-spec.json').read_text())
    (BASE/'playerstep-seeds.tsv').write_text(''.join(f'{a}\tfunction\t{n}\n' for a,n in ENTRIES.items()))
    spec.update(seeds=str(BASE/'playerstep-seeds.tsv'),output=str(BASE/'playerstep-rea'),addresses=list(ENTRIES))
    (BASE/'playerstep-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
    lifecycle={'0xd3ed':'spawn','0xd4bd':'death','0xd930':'death_to_respawn',
               '0xda07':'revive_weapon_and_attributes','0xda0b':'reset_player_attributes','0xd30b':'borrow_life'}
    (BASE/'lifecycle-seeds.tsv').write_text(''.join(f'{a}\tfunction\t{n}\n' for a,n in lifecycle.items()))
    life_spec=dict(spec,seeds=str(BASE/'lifecycle-seeds.tsv'),output=str(BASE/'lifecycle-rea'),addresses=list(lifecycle))
    (BASE/'lifecycle-spec.json').write_text(json.dumps(life_spec,indent=2)+'\n')
    print(json.dumps({'functions':len(ENTRIES),'spec':str(BASE/'playerstep-spec.json')}))

if __name__=='__main__':main()
