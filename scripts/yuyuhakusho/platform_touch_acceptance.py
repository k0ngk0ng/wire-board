"""Native multi-touch on the actual 320 px Board and public MD controller."""
def check(browser,js,native,report):
 browser('focus','[data-yuyu-board]')
 geometry=js('''(()=>{const d=document.querySelector("[data-controller-dpad]").getBoundingClientRect(),b=document.querySelector('[data-controller-button="x"]').getBoundingClientRect();return {right:{x:d.x+d.width*.82,y:d.y+d.height*.5},diagonal:{x:d.x+d.width*.18,y:d.y+d.height*.82},up:{x:d.x+d.width*.5,y:d.y+d.height*.18},x:{x:b.x+b.width*.5,y:b.y+b.height*.5}}})()''')
 def point(name,id):return dict(geometry[name],id=id,radiusX=3,radiusY=3,force=1)
 def touch(type,points,expected):return {'method':'Input.dispatchTouchEvent','type':type,'touchPoints':points,'expectButtons':expected}
 results=native([touch('touchStart',[point('right',1)],8),touch('touchStart',[point('right',1),point('x',2)],1032),touch('touchMove',[point('diagonal',1),point('x',2)],1030),touch('touchEnd',[point('diagonal',1)],1024),touch('touchCancel',[],0)])
 # The previous controls test deliberately cleared D and installed E as right.
 results+=native([{'type':'keyDown','key':'e','code':'KeyE','windowsVirtualKeyCode':69,'expectButtons':8},touch('touchStart',[point('up',1)],9),touch('touchCancel',[],8),{'type':'keyUp','key':'e','code':'KeyE','windowsVirtualKeyCode':69,'expectButtons':0}])
 report['native_touch']={'pass':True,'viewport_width':320,'multi_touch':True,'sliding_diagonal':True,'high_md_x_ordinary':True,'cancel_preserves_keyboard':True,'observations':results}
