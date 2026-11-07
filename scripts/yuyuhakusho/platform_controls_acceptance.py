"""Additional actual-Board controls contract, authored before physical integration.

The Gamepad API device below is virtual. It cannot establish hardware support.
"""
def check(browser,js,native,report):
 browser('focus','[data-yuyu-board]')
 js('window.yuyuBoardProbe.delay=0')
 browser('wait','--fn','yuyuBoardProbe.authority.inputs[0].appliedButtons===0')
 js('''(()=>{const pad={id:"Virtual acceptance standard pad",index:3,connected:true,mapping:"standard",axes:[0,0,0,0],buttons:Array.from({length:17},()=>({pressed:false,touched:false,value:0}))};window.yuyuVirtualPad=pad;Object.defineProperty(navigator,"getGamepads",{configurable:true,value:()=>pad.connected?[null,null,null,pad]:[]});window.dispatchEvent(new Event("gamepadconnected"));})()''')
 browser('wait','150')
 before=js('Number(document.querySelector("[data-yuyu-board]").dataset.localX)')
 marker=js('yuyuBoardProbe.actions.length')
 js('yuyuVirtualPad.buttons[15]={pressed:true,touched:true,value:1}')
 browser('wait','--fn',f'yuyuBoardProbe.actions.slice({marker}).some(action=>action.type==="input"&&(action.buttons&8)!==0)')
 browser('wait','--fn','yuyuBoardProbe.authority.inputs[0].appliedButtons===8')
 after=js('Number(document.querySelector("[data-yuyu-board]").dataset.localX)')
 assert abs(after-before)>=3,{'before':before,'after':after,'virtual':True}
 js('yuyuVirtualPad.buttons[15]={pressed:false,touched:false,value:0}')
 browser('wait','--fn','yuyuBoardProbe.authority.inputs[0].appliedButtons===0')
 # Disconnect releases only the physical source. A still-held keyboard source
 # continues through the same platform action pipeline.
 native([{'type':'keyDown','key':'d','code':'KeyD','windowsVirtualKeyCode':68,'wait':100}])
 js('yuyuVirtualPad.buttons[15]={pressed:true,touched:true,value:1}')
 browser('wait','100')
 js('yuyuVirtualPad.connected=false;window.dispatchEvent(new Event("gamepaddisconnected"))')
 browser('wait','100')
 assert js('yuyuBoardProbe.authority.inputs[0].appliedButtons')==8
 native([{'type':'keyUp','key':'d','code':'KeyD','windowsVirtualKeyCode':68}])
 browser('wait','--fn','yuyuBoardProbe.authority.inputs[0].appliedButtons===0')
 marker=js('yuyuBoardProbe.actions.length')
 js('yuyuVirtualPad.connected=true;window.dispatchEvent(new Event("gamepadconnected"))')
 browser('wait','180')
 assert not js(f'yuyuBoardProbe.actions.slice({marker}).some(action=>action.type==="input"&&(action.buttons&8)!==0)')
 js('yuyuVirtualPad.buttons[15]={pressed:false,touched:false,value:0}')
 browser('wait','100')
 js('yuyuVirtualPad.buttons[15]={pressed:true,touched:true,value:1}')
 browser('wait','--fn',f'yuyuBoardProbe.actions.slice({marker}).some(action=>action.type==="input"&&(action.buttons&8)!==0)')
 js('yuyuVirtualPad.buttons[15]={pressed:false,touched:false,value:0}')
 browser('wait','--fn','yuyuBoardProbe.authority.inputs[0].appliedButtons===0')
 # A legal empty primary key and replacement binding must remove the old key.
 js('''window.dispatchEvent(new CustomEvent("controller-settings-changed",{detail:{userId:"human",model:"game:yuyuhakusho",profile:{revision:2,turboHz:10,keyboard:{up:"KeyW",down:"KeyS",left:"KeyA",right:"",mode:"KeyF",start:"Space",a:"KeyJ",b:"KeyK",c:"KeyL",x:"KeyU",y:"KeyI",z:"KeyO"},keyboardAlt:{right:["KeyE"]}}}}))''')
 browser('focus','[data-yuyu-board]');browser('wait','100')
 marker=js('yuyuBoardProbe.actions.length')
 native([{'type':'keyDown','key':'d','code':'KeyD','windowsVirtualKeyCode':68,'wait':100},{'type':'keyUp','key':'d','code':'KeyD','windowsVirtualKeyCode':68}])
 assert not js(f'yuyuBoardProbe.actions.slice({marker}).some(action=>action.type==="input"&&(action.buttons&8)!==0)')
 native([{'type':'keyDown','key':'e','code':'KeyE','windowsVirtualKeyCode':69,'wait':120}])
 browser('wait','--fn',f'yuyuBoardProbe.actions.slice({marker}).some(action=>action.type==="input"&&(action.buttons&8)!==0)')
 native([{'type':'keyUp','key':'e','code':'KeyE','windowsVirtualKeyCode':69}])
 browser('wait','--fn','yuyuBoardProbe.authority.inputs[0].appliedButtons===0')
 report['controls']={'pass':True,'virtual_gamepad_api':True,'hardware_tested':False,'visible_local_x':{'before':before,'after':after},'applied_release':True,'mixed_source_disconnect':True,'reconnect_requires_neutral':True,'cleared_primary_and_alternative':True}
