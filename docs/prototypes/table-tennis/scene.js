import * as T from '../../../web/node_modules/three/build/three.module.js';
const params=new URLSearchParams(location.search),still=params.get('still')==='1';
if(params.get('view')==='preview')document.body.classList.add('preview');
const themes={
 club:{title:'A · 现代球馆',description:'奶油白、鼠尾草绿与赤陶红。纯球拍、柔和日光、清楚的击球轮廓。',bg:'#dce4d9',floor:'#cba87a',table:'#2c796f',edge:'#174c45',home:'#e9e7cd',away:'#bd5b43',short:'#234f45',wall:'#e6e8dd',accent:'#d5e7a0',colors:['#e9e7d8','#2c796f','#bd5b43','#cba87a']},
 arena:{title:'B · 街机赛场',description:'深蓝竞技空间、珊瑚红球台、青绿色边光。清晰的球拍轮廓与抽球速度感。',bg:'#101f33',floor:'#23384a',table:'#78394f',edge:'#482b45',home:'#58c7c0',away:'#ecb259',short:'#213146',wall:'#16273d',accent:'#b4f7df',colors:['#15273c','#b75868','#65c8c3','#edb762']},
 heritage:{title:'C · 复古体育馆',description:'蜂蜜色木地板、墨绿球台和高窗日光。传统胶皮球拍，带生活气息的运动空间。',bg:'#dfcbb1',floor:'#b88953',table:'#316459',edge:'#23473e',home:'#eee0b8',away:'#b96947',short:'#455345',wall:'#e4d2b5',accent:'#ede6a3',colors:['#e6d6bb','#356859','#b66b4b','#b68a57']}
};
let themeName=params.get('style') in themes?params.get('style'):'club',theme=themes[themeName];
const canvas=document.querySelector('#court'),viewport=canvas.parentElement;
let renderer;
try{renderer=new T.WebGLRenderer({canvas,antialias:true,alpha:false,preserveDrawingBuffer:true});}catch(e){viewport.insertAdjacentHTML('beforeend','<div class="error">需要支持 WebGL 的浏览器才能查看 3D 美术原型。</div>');throw e;}
renderer.setPixelRatio(Math.min(devicePixelRatio,2));renderer.shadowMap.enabled=true;renderer.shadowMap.type=T.PCFSoftShadowMap;renderer.outputColorSpace=T.SRGBColorSpace;renderer.toneMapping=T.ACESFilmicToneMapping;renderer.toneMappingExposure=1.12;
let scene,camera,ball,ballShadow,streak,racketRig,racketRest,rackets=[],swingAt=-10,swingType='抽球',seed=8,frame=0,disposed=0;
const v=(x,y,z)=>new T.Vector3(x,y,z);
function rand(){seed=(seed*1664525+1013904223)>>>0;return seed/4294967296;}
function mat(color,roughness=.72,metalness=0){return new T.MeshStandardMaterial({color,roughness,metalness});}
function box(w,h,d,m,x=0,y=0,z=0,parent=scene){const o=new T.Mesh(new T.BoxGeometry(w,h,d),typeof m==='string'?mat(m):m);o.position.set(x,y,z);o.castShadow=true;o.receiveShadow=true;parent.add(o);return o;}
function sphere(r,m,x=0,y=0,z=0,parent=scene,sx=1,sy=1,sz=1){const o=new T.Mesh(new T.SphereGeometry(r,24,16),typeof m==='string'?mat(m):m);o.position.set(x,y,z);o.scale.set(sx,sy,sz);o.castShadow=true;o.receiveShadow=true;parent.add(o);return o;}
function rod(a,b,r,m,parent=scene,r2=r){const dir=b.clone().sub(a),o=new T.Mesh(new T.CylinderGeometry(r,r2,dir.length(),12),typeof m==='string'?mat(m):m);o.position.copy(a).add(b).multiplyScalar(.5);o.quaternion.setFromUnitVectors(v(0,1,0),dir.normalize());o.castShadow=true;o.receiveShadow=true;parent.add(o);return o;}
function line(points,color,width=.025,parent=scene){for(let i=1;i<points.length;i++)rod(points[i-1],points[i],width,color,parent);}
function woodTexture(){const c=document.createElement('canvas');c.width=c.height=1024;const ctx=c.getContext('2d');ctx.fillStyle=theme.floor;ctx.fillRect(0,0,1024,1024);for(let i=0;i<24;i++){let y=i*43;ctx.fillStyle=i%2?'#ffffff0e':'#00000008';ctx.fillRect(0,y,1024,43);ctx.fillStyle='#38271924';ctx.fillRect(0,y,1024,1);for(let j=0;j<8;j++){let x=j*150+(i%2)*75;ctx.fillRect(x,y,1,43);}for(let j=0;j<22;j++){ctx.strokeStyle=rand()>.5?'#4f34100b':'#fff5dc14';ctx.beginPath();let x=rand()*1024;ctx.moveTo(x,y+rand()*42);ctx.lineTo(x+100+rand()*300,y+rand()*42);ctx.stroke();}}const tex=new T.CanvasTexture(c);tex.colorSpace=T.SRGBColorSpace;tex.anisotropy=4;return tex;}
function roundedCourt(){const floorMat=themeName==='arena'?mat(theme.floor):new T.MeshStandardMaterial({map:woodTexture(),roughness:.82});box(15,.24,18,floorMat,0,-.14,0);box(15.3,.3,18.3,themeName==='arena'?'#102030':'#a4997d',0,-.39,0);line([v(-5.8,.015,-7),v(5.8,.015,-7),v(5.8,.015,7),v(-5.8,.015,7),v(-5.8,.015,-7)],themeName==='arena'?'#66818b':'#efe7c7',.025);}
function table(){const top=mat(theme.table,.4),edge=mat(theme.edge,.48);box(4.2,.13,7.56,top,0,1.72,0);box(4.24,.15,7.6,edge,0,1.59,0);for(const x of [-2.04,2.04])box(.035,.009,7.42,'#f2f2dc',x,1.791,0);for(const z of [-3.7,3.7])box(4.07,.009,.035,'#f2f2dc',0,1.791,z);box(.022,.009,7.4,'#d9e4cf',0,1.791,0);
 const steel=mat(themeName==='arena'?'#223c51':'#4d6054',.5,.25);for(const z of [-2.4,2.4]){for(const x of [-1.55,1.55]){rod(v(x,1.55,z),v(x*1.12,.15,z),.08,steel);sphere(.12,'#343e37',x*1.12,.1,z,scene,1,.7,1);}rod(v(-1.55,.65,z),v(1.55,.65,z),.06,steel);}rod(v(0,.6,-2.4),v(0,.6,2.4),.07,steel);
 const netmat=new T.MeshStandardMaterial({color:'#edf2df',transparent:true,opacity:.38,side:T.DoubleSide});const net=new T.Mesh(new T.PlaneGeometry(4.38,.43),netmat);net.position.set(0,2.005,0);scene.add(net);for(let x=-2.18;x<=2.2;x+=.11)rod(v(x,1.79,0),v(x,2.22,0),.006,'#e3e7d3');for(let y=1.8;y<=2.2;y+=.073)rod(v(-2.2,y,0),v(2.2,y,0),.006,'#e3e7d3');rod(v(-2.25,2.23,0),v(2.25,2.23,0),.022,'#f7f3e4');for(const x of [-2.28,2.28]){rod(v(x,1.6,0),v(x,2.28,0),.04,steel);box(.22,.1,.17,steel,x,1.76,0);}
}
function paddle(parent,position,color){const g=new T.Group();g.name="paddle";rackets.push(g);g.position.copy(position);g.rotation.x=-.55;g.rotation.z=-.55;parent.add(g);const wood=mat('#d2ad72'),rubber=mat(color||(themeName==='arena'?'#f47b60':'#c64c3e'),.9);const disk=new T.Mesh(new T.CylinderGeometry(.23,.23,.06,32),wood);disk.rotation.x=Math.PI/2;disk.scale.y=1;disk.scale.z=1.13;g.add(disk);const face=new T.Mesh(new T.CircleGeometry(.216,32),rubber);face.position.z=.035;face.scale.y=1.13;g.add(face);rod(v(0,-.17,0),v(0,-.47,0),.046,wood,g);return g;}
function bench(x,z,rotation){const g=new T.Group();g.position.set(x,0,z);g.rotation.y=rotation;scene.add(g);const timber=mat(themeName==='heritage'?'#9d603b':'#b79764');for(const zz of [-.23,0,.23])box(2.6,.12,.19,timber,0,.58,zz,g);for(const xx of [-.95,.95])for(const zz of [-.2,.2])box(.085,.58,.085,'#566452',xx,.27,zz,g);box(2.6,.32,.09,timber,0,1.03,.35,g);box(.55,.13,.42,theme.away,.4,.72,0,g);for(const xx of [-.65,-.35]){rod(v(xx,.66,-.05),v(xx,1,-.05),.065,'#c9d8c2',g);sphere(.067,'#f0e8cf',xx,1,-.05,g,1,.25,1);} }
function plant(x,z){const g=new T.Group();g.position.set(x,0,z);scene.add(g);const pot=new T.Mesh(new T.CylinderGeometry(.34,.24,.6,24),mat('#b7795b'));pot.position.y=.3;pot.castShadow=true;g.add(pot);for(let i=0;i<9;i++){let angle=i*2.4;let dx=Math.cos(angle)*.32,dz=Math.sin(angle)*.32;rod(v(0,.5,0),v(dx,1.4+(i%3)*.23,dz),.018,'#52724e',g);const leaf=sphere(.28,i%2?'#557e59':'#78936b',dx,1.45+(i%3)*.2,dz,g,.6,1.7,.22);leaf.rotation.z=Math.sin(angle)*.7;leaf.rotation.y=angle;}}
function arenaLights(){for(const x of [-6.9,6.9]){box(.1,2.4,15,'#172b3b',x,1.2,-.2);box(.05,.055,15,new T.MeshStandardMaterial({color:'#68d5c7',emissive:'#41a798',emissiveIntensity:1.1}),x-(Math.sign(x)*.07),2.42,-.2);for(let z=-5;z<6;z+=2.5){box(.08,.8,1.6,'#274658',x-(Math.sign(x)*.06),1.3,z);}}for(let i=0;i<7;i++){const x=(i-3)*1.6;box(1.3,.38,.7,i%2?'#384758':'#b47464',x,.3,-7.1);box(1.3,.6,.15,'#243747',x,.75,-7.4);}for(let x of [-5,5]){const spot=new T.SpotLight('#e6fff5',18,24,Math.PI/6,.8,1);spot.position.set(x,9,1);spot.target.position.set(0,0,0);scene.add(spot,spot.target);box(.9,.18,.4,'#6d908c',x,7,-6);}}
function environment(){roundedCourt();
 const upper=box(50,16,.2,theme.wall,0,8,-8.7);upper.castShadow=false;const ceiling=box(50,.12,40,themeName==='arena'?'#0f2136':'#eeede2',0,12,1);ceiling.castShadow=false;
 for(const x of [-6,6]){const lamp=box(.16,.04,5,new T.MeshStandardMaterial({color:'#faf4d8',emissive:'#fff1cc',emissiveIntensity:.4}),x,7.55,-1.4);lamp.castShadow=false;}
if(themeName==='arena'){arenaLights();return;}
 const backZ=-8.4;box(15,.4,.3,themeName==='club'?'#698b76':'#967d5b',0,.2,backZ);box(15,4.2,.25,theme.wall,0,2.5,backZ);
 if(themeName==='heritage'){
  for(let x=-5.5;x<6;x+=3.65){box(2.6,2.4,.04,'#f8eace',x,3.2,backZ+.15);for(let xx of [-1.25,0,1.25])box(.06,2.4,.07,'#88775a',x+xx,3.2,backZ+.19);for(let y of [2.03,3.2,4.37])box(2.6,.06,.07,'#88775a',x,y,backZ+.2);}
  for(let i=0;i<16;i++)box(15,.035,.018,'#c5ad87',0,.6+i*.075,backZ+.14);
  const clock=new T.Mesh(new T.CircleGeometry(.34,40),mat('#f2e8cd'));clock.position.set(0,2.6,backZ+.2);scene.add(clock);line([v(-.14,2.6,backZ+.23),v(0,2.6,backZ+.23),v(0,2.82,backZ+.23)],'#4f5946',.015);
  bench(-5.6,-3,Math.PI/2);bench(5.7,1,-Math.PI/2);
  for(let z=-4;z<6;z+=3.1){const patch=box(4,.008,1.4,new T.MeshStandardMaterial({color:'#fff1bb',transparent:true,opacity:.24,depthWrite:false}),-2.2,.012,z);patch.rotation.y=-.5;patch.castShadow=false;}
 }else{
  for(let x=-6.8;x<7;x+=.35)box(.07,2.4,.05,'#b3c1ac',x,1.55,backZ+.15);
  box(5,.8,.06,'#e2dcc7',0,3.4,backZ+.16);for(const x of [-1.4,0,1.4]){const disk=new T.Mesh(new T.CircleGeometry(.21,40),mat(x===0?'#b56d51':'#6b8b71'));disk.position.set(x,3.4,backZ+.21);scene.add(disk);}
  bench(-5.8,-1.8,Math.PI/2);plant(-6.5,-6.7);plant(6.3,-6.4);
 }
 // Side advertising boards are abstract material accents, without logos or tiny text.
 for(const x of [-6.8,6.8]){box(.12,.85,6.1,themeName==='club'?'#dce3d0':'#69846a',x,.45,3.7);box(.14,.055,6.12,'#f1e9ce',x,.9,3.7);}
}
function firstPersonRacket(){
 racketRig=new T.Group();camera.add(racketRig);scene.add(camera);
 const racket=paddle(racketRig,v(.43,-.16,-1.72),'#267c6a');racket.rotation.set(-.1,-.3,-.28);racket.scale.setScalar(.76);
 const front=new T.Mesh(new T.CircleGeometry(.213,48),mat('#263b38',.93));front.position.z=.037;front.scale.y=1.13;racket.add(front);
 racketRig.traverse(o=>{o.castShadow=false;});racketRest=v(0,0,0);
}
function build(){if(scene){scene.traverse(o=>{if(o.geometry)o.geometry.dispose();if(o.material){for(const m of Array.isArray(o.material)?o.material:[o.material]){if(m.map)m.map.dispose();m.dispose();}}});disposed++;}
 seed=8;rackets=[];theme=themes[themeName];scene=new T.Scene();scene.background=new T.Color(theme.bg);scene.fog=new T.Fog(theme.bg,32,60);camera=new T.PerspectiveCamera(35,1,.1,100);
 const ambient=new T.HemisphereLight(themeName==='arena'?'#b7cfef':'#fff5dc',themeName==='arena'?'#18394a':'#879e83',themeName==='arena'?1.5:2.5);scene.add(ambient);
 const sun=new T.DirectionalLight(themeName==='heritage'?'#ffe0aa':'#fff9e9',themeName==='arena'?1.8:3.5);sun.position.set(-7,12,5);sun.castShadow=true;sun.shadow.mapSize.set(2048,2048);Object.assign(sun.shadow.camera,{left:-11,right:11,top:12,bottom:-12,near:1,far:35});sun.shadow.normalBias=.035;sun.shadow.bias=-.0002;sun.shadow.radius=4;scene.add(sun);
 const fill=new T.DirectionalLight(themeName==='arena'?'#71cfdf':'#c4dbd0',themeName==='arena'?1.3:1);fill.position.set(7,7,-6);scene.add(fill);
 environment();table();
 const teammate=paddle(scene,v(-1.4,2.25,.8),'#328b78');teammate.rotation.set(-.15,-.25,.45);teammate.scale.setScalar(1.35);
 const opponent=paddle(scene,v(.9,2.55,-4.05),'#bd5548');opponent.rotation.set(-.1,.12,-.3);opponent.scale.setScalar(1.6);
 const partner=paddle(scene,v(-1.15,2.35,-4.7),'#bd5548');partner.rotation.set(.08,-.16,.3);partner.scale.setScalar(1.6);
 firstPersonRacket();
 ball=sphere(.105,new T.MeshStandardMaterial({color:'#fff9df',emissive:'#fff5bd',emissiveIntensity:.32}),-.55,2.45,2.3);ball.castShadow=false;const outline=new T.Mesh(new T.SphereGeometry(.116,20,14),new T.MeshBasicMaterial({color:'#365248',side:T.BackSide}));ball.add(outline);const shmat=new T.MeshBasicMaterial({color:'#1e423a',transparent:true,opacity:.24,depthWrite:false});ballShadow=new T.Mesh(new T.CircleGeometry(.12,24),shmat);ballShadow.rotation.x=-Math.PI/2;scene.add(ballShadow);
 const curve=new T.CatmullRomCurve3([v(.2,2.32,.5),v(-.05,2.5,1.1),v(-.3,2.54,1.7),v(-.55,2.45,2.3)]);streak=new T.Mesh(new T.TubeGeometry(curve,30,.025,6,false),new T.MeshBasicMaterial({color:themeName==='arena'?'#bafcf0':'#ffefb3',transparent:true,opacity:.8}));scene.add(streak);
 document.body.classList.remove('theme-club','theme-arena','theme-heritage');document.body.classList.add('theme-'+themeName);document.querySelector('#theme-title').textContent=theme.title;document.querySelector('#theme-description').textContent=theme.description;document.querySelector('#swatches').innerHTML=theme.colors.map(c=>`<i style="background:${c}"></i>`).join('');document.querySelectorAll('[data-theme]').forEach(b=>b.setAttribute('aria-pressed',b.dataset.theme===themeName));resize();window.__prototype={ready:true,theme:themeName,players:4,rackets:rackets.length,bodyMovementRequired:false,doubles:true,perspective:'first-person',selfBodyVisible:false,productionRules:false,shots:[],disposedScenes:disposed};}
function resize(){const w=viewport.clientWidth,h=viewport.clientHeight;renderer.setSize(w,h,false);camera.aspect=w/h;const mobile=w<650;
 camera.position.set(0,3.5,mobile?6.2:6.8);camera.fov=mobile?64:49;
 camera.lookAt(0,1.95,-1.1);camera.updateProjectionMatrix();
 racketRest.set(mobile?-.16:.17,mobile?-.46:-.15,0);racketRig.position.copy(racketRest);renderer.render(scene,camera);
}
function shoot(type){swingType=type;swingAt=performance.now()/1000;document.querySelector('#feedback').textContent=type;document.querySelector('#feedback').classList.add('on');clearTimeout(shoot.timer);shoot.timer=setTimeout(()=>document.querySelector('#feedback').classList.remove('on'),900);window.__prototype.shots.push(type);}
let start=null;const gesture=canvas;
function clearGesture(){start=null;if(racketRig)racketRig.position.copy(racketRest);}
gesture.addEventListener('pointerdown',e=>{if(start||e.button>0)return;start={x:e.clientX,y:e.clientY,id:e.pointerId};gesture.focus({preventScroll:true});gesture.setPointerCapture(e.pointerId);});
gesture.addEventListener('pointermove',e=>{if(!start||start.id!==e.pointerId)return;const dx=(e.clientX-start.x)/viewport.clientWidth,dy=(e.clientY-start.y)/viewport.clientHeight;racketRig.position.set(racketRest.x+Math.max(-.45,Math.min(.45,dx)),racketRest.y-Math.max(-.35,Math.min(.35,dy)),0);});
gesture.addEventListener('pointerup',e=>{if(!start||start.id!==e.pointerId)return;const dy=e.clientY-start.y,dx=e.clientX-start.x;clearGesture();shoot(Math.hypot(dx,dy)<12?'挡球':dy>12?'削球':Math.abs(dx)>Math.abs(dy)*1.8?'变线抽球':'抽球');});
gesture.addEventListener('pointercancel',clearGesture);gesture.addEventListener('lostpointercapture',clearGesture);
gesture.addEventListener('keydown',e=>{if(['ArrowUp','ArrowDown',' ','Enter'].includes(e.key)){e.preventDefault();shoot(e.key==='ArrowDown'?'削球':e.key==='ArrowUp'?'抽球':'挡球');}if(e.key==='Escape')clearGesture();});
document.querySelectorAll('[data-theme]').forEach(b=>b.onclick=()=>{clearGesture();themeName=b.dataset.theme;const u=new URL(location);u.searchParams.set('style',themeName);history.replaceState(null,'',u);build();});window.addEventListener('resize',()=>{clearGesture();resize();});window.addEventListener('blur',clearGesture);document.addEventListener('visibilitychange',()=>{if(document.hidden)clearGesture();});
function animate(ms){const now=ms/1000,age=now-swingAt;const moving=age>=0&&age<1.35;if(moving){let p=Math.min(age/1.1,1);ball.position.set(-.65+2.25*p,swingType==='削球'?2+Math.sin(p*Math.PI)*.65:2+Math.sin(p*Math.PI)*1.4,3.3-6.5*p);streak.visible=false;if(!start){const k=Math.sin(Math.min(age/.45,1)*Math.PI);racketRig.position.set(racketRest.x-k*.42,racketRest.y+k*(swingType==='削球'?-.25:.2),-k*.1);racketRig.rotation.z=k*(swingType==='削球'?.3:-.4);}}else{ball.position.set(-.55,2.45,2.3);streak.visible=true;if(!start){racketRig.position.copy(racketRest);racketRig.rotation.z=0;}}
 ballShadow.position.set(ball.position.x,1.803,ball.position.z);const k=1+(ball.position.y-1.8)*.2;ballShadow.scale.setScalar(k);renderer.render(scene,camera);frame++;window.__prototype.frame=frame;requestAnimationFrame(animate);}
build();requestAnimationFrame(animate);
