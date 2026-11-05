"""Actual fullscreen geometry and trusted native C touch on three viewports.

This contract precedes the game-owned landscape CSS change. Network/RPC are
frozen; no implementation state or viewport/visibility API is overridden.
"""
def check(browser,js,native,report,out):
 before=js('window.yuyuLayoutCanvas=document.querySelector("canvas");({epoch:yuyuBoardProbe.authority.epoch,frame:yuyuBoardProbe.authority.frame})')
 def geometry():return js('''(()=>{const rect=selector=>{const r=document.querySelector(selector).getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom}};const c=rect("canvas"),scale=Math.min(c.width/320,c.height/224);return {viewport:{width:innerWidth,height:innerHeight},board:rect("[data-yuyu-board]"),canvas:c,image:{x:c.x+(c.width-320*scale)/2,y:c.y+(c.height-224*scale)/2,width:320*scale,height:224*scale,right:c.x+(c.width+320*scale)/2,bottom:c.y+(c.height+224*scale)/2},pad:rect("[data-controller-layout=md]"),c:rect('[data-controller-button="c"]'),hud:document.querySelector('[data-layout-public-hud]')?rect('[data-layout-public-hud]'):null,toggle:document.querySelector('[data-fullscreen-toggle]')?rect('[data-fullscreen-toggle]'):null,fullscreen:!!document.querySelector("[data-room-fullscreen]"),nativeFullscreen:!!document.fullscreenElement,scrollWidth:document.documentElement.scrollWidth,source:{width:document.querySelector("canvas").width,height:document.querySelector("canvas").height},fit:getComputedStyle(document.querySelector("canvas")).objectFit,buttons:[...document.querySelectorAll("[data-controller-button]")].map(node=>{const r=node.getBoundingClientRect();return {name:node.dataset.controllerButton,x:r.x,y:r.y,right:r.right,bottom:r.bottom,width:r.width,height:r.height}})}})()''')
 def touch_c(g):
  point={'id':31,'x':g['c']['x']+g['c']['width']/2,'y':g['c']['y']+g['c']['height']/2,'radiusX':3,'radiusY':3,'force':1}
  assert js(f'document.elementFromPoint({point["x"]},{point["y"]})?.closest("[data-controller-button]")?.dataset.controllerButton')=='c'
  result=native([{'method':'Input.dispatchTouchEvent','type':'touchStart','touchPoints':[point],'expectButtons':32},{'method':'Input.dispatchTouchEvent','type':'touchEnd','touchPoints':[],'expectButtons':0}])
  assert 'c' in result[0]['pressedButtons'] and 'c' not in result[1]['pressedButtons'] and result[0]['cBackground']!=result[1]['cBackground'],result
  return result
 cases=[]
 for label,width,height,fullscreen in [('desktop',1100,1000,False),('portrait-320',320,740,False),('landscape-fullscreen',740,360,True)]:
  browser('set','viewport',str(width),str(height))
  if fullscreen:
   browser('click','#fullscreen');browser('wait','--fn','document.querySelector("[data-room-fullscreen]") !== null')
  g=geometry();g['native_touch']=touch_c(g);g['label']=label;cases.append(g);browser('screenshot',str(out/(label+'.png')))
 report['layouts']=cases
 for g in cases:
  assert g['pad']['y']>=g['canvas']['bottom']-.5,{'failure':'pad_must_be_below_canvas','case':g}
  assert g['pad']['bottom']<=g['viewport']['height']+.5 and g['canvas']['y']>=-.5,{'failure':'canvas_and_pad_must_both_fit_viewport','case':g}
  assert g['pad']['x']>=-.5 and g['pad']['right']<=g['viewport']['width']+.5,g
  assert g['scrollWidth']<=g['viewport']['width'],g
  assert g['source']=={'width':320,'height':224} and g['fit']=='contain',g
  assert g['image']['width']>=230 and g['image']['height']>=160 and abs(g['image']['width']/g['image']['height']-320/224)<.001,g
  assert g['pad']['width']>=250,g
  assert abs((g['canvas']['x']+g['canvas']['width']/2)-(g['pad']['x']+g['pad']['width']/2))<=1,g
  for button in g['buttons']:assert button['x']>=-.5 and button['right']<=g['viewport']['width']+.5 and button['y']>=-.5 and button['bottom']<=g['viewport']['height']+.5,g
  if g['fullscreen']:
   assert g['hud'] and g['hud']['y']>=0 and g['hud']['bottom']<=g['canvas']['y']+.5,g
   assert g['toggle'] and g['toggle']['right']<=g['viewport']['width'] and g['toggle']['bottom']<=g['canvas']['y']+.5,g
 browser('press','Escape');browser('wait','--fn','document.querySelector("[data-room-fullscreen]") === null')
 assert js('document.querySelectorAll("canvas").length')==1
 assert js('window.yuyuLayoutCanvas===document.querySelector("canvas")')
 after=js('({epoch:yuyuBoardProbe.authority.epoch,frame:yuyuBoardProbe.authority.frame})');assert before['epoch']==after['epoch'] and int(after['frame'])>int(before['frame'])
 report['game_progress']={'before':before,'after':after}
 report['escape_preserves_canvas']=True
