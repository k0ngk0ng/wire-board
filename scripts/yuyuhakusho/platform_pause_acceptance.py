"""Review: resume must predict locally while authoritative input is delayed."""
def check(browser,js,native,report):
 # Finish the preceding delayed movement pulse before starting an independent
 # pause pulse; its release is still deliberately delayed by the prior case.
 browser('wait','--fn','yuyuBoardProbe.authority.inputs[0].appliedButtons===0&&yuyuBoardProbe.authority.inputs[0].appliedSequence>=Math.max(...yuyuBoardProbe.actions.map(action=>action.sequence??0))')
 browser('focus','[data-yuyu-board]');js('yuyuBoardProbe.delay=500')
 paused=native([{'type':'keyDown','key':' ','code':'Space','windowsVirtualKeyCode':32,'observePaused':True}])[0]
 report['pause_start_delayed']=paused
 assert paused['localPaused'] is True and paused['authorityPhase']=='playing',paused
 native([{'type':'keyUp','key':' ','code':'Space','windowsVirtualKeyCode':32}])
 browser('wait','--fn','yuyuBoardProbe.authority.phase==="paused"')
 browser('wait','--fn','yuyuBoardProbe.authority.inputs[0].appliedButtons===0')
 js('yuyuBoardProbe.delay=500')
 observation=native([{'type':'keyDown','key':' ','code':'Space','windowsVirtualKeyCode':32,'observePaused':False}])[0]
 report['pause_resume_delayed']=observation
 native([{'type':'keyUp','key':' ','code':'Space','windowsVirtualKeyCode':32}])
 assert observation['localPaused'] is False and observation['authorityPhase']=='paused',observation
 browser('wait','--fn','yuyuBoardProbe.authority.phase==="playing"&&yuyuBoardProbe.authority.inputs[0].appliedButtons===0')
 js('yuyuBoardProbe.delay=0')
