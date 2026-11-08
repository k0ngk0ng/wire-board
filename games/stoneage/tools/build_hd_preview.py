#!/usr/bin/env python3
"""Build an offline original/HD animation comparator alongside versioned images."""
import argparse
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--source', type=Path, required=True)
    p.add_argument('--output', type=Path, required=True)
    args = p.parse_args()
    output = args.output.resolve()
    assert output.is_relative_to(args.source.resolve() / 'client/web/assets/hd')
    art = json.loads((ROOT / 'games/stoneage/web/art.json').read_text())
    manifest = json.loads((output / 'manifest.json').read_text())
    data = dict(sprites=art['sprites'], available=list(manifest['frames']),
                exported=manifest['exportedFiles'], expected=manifest['expectedFiles'])
    html = '''<!doctype html><html lang="zh-CN"><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>石器高清帧对照</title>
<style>
*{box-sizing:border-box}body{margin:0;padding:20px;background:#172b29;color:#eef5ec;font:15px system-ui}main{max-width:1280px;margin:auto}h1{font-size:22px;margin:0 0 10px}p{color:#c6d3c8;line-height:1.7}nav{display:flex;flex-wrap:wrap;gap:12px;align-items:center;padding:14px;background:#25413a;border-radius:10px}label{display:flex;align-items:center;gap:6px}select,button{background:#f2ead8;color:#23372e;border:0;border-radius:5px;padding:9px;font:inherit}button{cursor:pointer}.panels{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:16px}figure{margin:0;background:#30483f;border:1px solid #688675;border-radius:10px;overflow:hidden}figcaption{padding:10px 14px;background:#25413a}canvas{display:block;width:100%;height:auto;background:repeating-conic-gradient(#33493f 0% 25%,#394f45 0% 50%) 0/24px 24px}#info{min-height:24px;margin:12px 0;color:#e6d99f}@media(max-width:600px){body{padding:10px}.panels{grid-template-columns:1fr}nav{gap:9px;padding:10px}label{font-size:13px}}
</style><main><h1>石器高清帧对照</h1><p id="coverage"></p>
<nav><label>造型 <select id="graphic"></select></label><label>方向 <select id="direction"><option value="3">左上</option><option value="7">右下</option></select></label><label>动作 <select id="action"></select></label><label>缩放 <input id="zoom" type="range" min="0.6" max="2" step="0.1" value="1"></label><button id="play">暂停</button><button id="step">下一帧</button></nav>
<div id="info" role="status"></div><div class="panels"><figure><figcaption>原始帧</figcaption><canvas id="original" width="600" height="460"></canvas></figure><figure><figcaption>GPT 恢复 · 原始轮廓与锚点</figcaption><canvas id="hd" width="600" height="460"></canvas></figure></div>
<p>左右使用相同动作、帧序、tick 和逻辑偏移。空白部分为透明像素。仅覆盖竞技场已用图片；其他方向和未生成帧继续使用原图。本页用于检查动作与画面，不会切换游戏运行素材。</p></main>
<script>const DATA=__DATA__;
const available=new Set(DATA.available),cache=new Map(),errors=[];
const $=s=>document.querySelector(s), names={0:'攻击',1:'受击',2:'倒地',3:'待机',4:'防御',10:'招式'};
let animation,index=0,playing=true,last=0,elapsed=0;
for(const g of Object.keys(DATA.sprites).sort((a,b)=>Number(a)-Number(b))){const o=document.createElement('option');o.value=g;o.textContent=g;$('#graphic').append(o)}
const first=Object.entries(DATA.sprites).find(([g,actions])=>actions.some(a=>a.action===3&&a.frames.every(f=>available.has(f.file))));if(first)$('#graphic').value=first[0];
$('#coverage').textContent=`已导出 ${DATA.exported} / ${DATA.expected} 张；高清像素为原图宽高的 2 倍。`;
function image(path){if(!cache.has(path)){const img=new Image();img.onload=()=>{};img.onerror=()=>errors.push(path);img.src=path;cache.set(path,img)}return cache.get(path)}
function choose(){const old=Number($('#action').value);$('#action').innerHTML='';const actions=DATA.sprites[$('#graphic').value].filter(a=>a.direction===Number($('#direction').value));for(const a of actions){const o=document.createElement('option');o.value=a.action;o.textContent=(names[a.action]||a.action)+(a.frames.every(f=>available.has(f.file))?'':' · 部分未生成');$('#action').append(o)}$('#action').value=actions.some(a=>a.action===old)?old:actions.some(a=>a.action===3)?3:actions[0]?.action;reset()}
function reset(){animation=DATA.sprites[$('#graphic').value].find(a=>a.direction===Number($('#direction').value)&&a.action===Number($('#action').value));index=0;elapsed=0;last=0;if(animation)for(const f of animation.frames){image('../../original/'+f.file);if(available.has(f.file))image(f.file)}}
function paint(id,path,f,scale,smoothing){const canvas=$(id),ctx=canvas.getContext('2d');ctx.clearRect(0,0,canvas.width,canvas.height);ctx.strokeStyle='#a0b59c';ctx.beginPath();ctx.moveTo(290,434);ctx.lineTo(310,434);ctx.moveTo(300,424);ctx.lineTo(300,444);ctx.stroke();const im=image(path);ctx.imageSmoothingEnabled=smoothing;if(im.complete&&im.naturalWidth)ctx.drawImage(im,300+f.x*scale,434+f.y*scale,f.width*scale,f.height*scale)}
function tick(t){if(animation?.frames.length){const duration=Math.max(1,animation.ticks)*1000/60;if(playing&&last)elapsed+=Math.min(100,t-last);if(playing)index=Math.floor(elapsed/duration)%animation.frames.length;const f=animation.frames[index];const extent=Math.max(...animation.frames.map(f=>Math.max(f.height,Math.abs(f.y)+8)));const scale=Math.min(2.3,400/extent)*Number($('#zoom').value);paint('#original','../../original/'+f.file,f,scale,false);paint('#hd',available.has(f.file)?f.file:'../../original/'+f.file,f,scale,true);$('#info').textContent=`第 ${index+1} / ${animation.frames.length} 帧 · ${duration.toFixed(0)} ms/帧${available.has(f.file)?'':' · 此帧尚未生成高清版'}`;window.__hdReview={graphic:Number($('#graphic').value),action:animation.action,direction:animation.direction,index,frames:animation.frames.length,ready:cache.size>0&&[...cache.values()].every(im=>im.complete&&im.naturalWidth),errors:errors.slice(),exported:DATA.exported}}last=t;requestAnimationFrame(tick)}
$('#graphic').onchange=choose;$('#direction').onchange=choose;$('#action').onchange=reset;$('#play').onclick=()=>{playing=!playing;$('#play').textContent=playing?'暂停':'播放';elapsed=index*Math.max(1,animation.ticks)*1000/60};$('#step').onclick=()=>{playing=false;$('#play').textContent='播放';index=(index+1)%animation.frames.length};choose();requestAnimationFrame(tick);
</script></html>'''
    (output / 'review.html').write_text(html.replace('__DATA__', json.dumps(data, separators=(',', ':'))))
    index = dict(pixelScale=2, logicalOffsetsUnchanged=True, sprites=art['sprites'])
    (output / 'arena-animation-index.json').write_text(json.dumps(index, separators=(',', ':')) + '\n')
    print(json.dumps({'preview': str(output / 'review.html'), 'exported': data['exported']}))


if __name__ == '__main__':
    main()
