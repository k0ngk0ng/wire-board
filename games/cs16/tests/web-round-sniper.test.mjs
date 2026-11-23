import test from 'node:test';
import assert from 'node:assert/strict';
import { motionPhase, zoomAfterShot } from '../web/round-sniper.ts';

test('prediction follows round-end survivors and stops in freeze or final state', () => {
  for(const phase of ['live','round_end']) assert.equal(motionPhase(phase),true);
  for(const phase of ['freeze','finished','']) assert.equal(motionPhase(phase),false);
});

test('bolt sniper local feedback exits scope immediately and expires at exact cycle', () => {
  for(const [weapon,duration] of [['awp',1450],['scout',1250]]) {
    for(const zoom of [0,40,10,15]) {
      const shot={weapon,at:1000};
      assert.equal(zoomAfterShot(weapon,zoom,shot,999),zoom);
      assert.equal(zoomAfterShot(weapon,zoom,shot,1000),0);
      assert.equal(zoomAfterShot(weapon,zoom,shot,1000+duration-1),0);
      assert.equal(zoomAfterShot(weapon,zoom,shot,1000+duration),zoom);
      assert.equal(zoomAfterShot('g3sg1',zoom,shot,1001),zoom);
    }
  }
  assert.equal(zoomAfterShot('awp',40,{weapon:'knife',at:1000},1001),40);
});
