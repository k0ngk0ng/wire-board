import importlib.util
import json
import math
from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / ".local/cs16/research/original/cstrike/models"
REPORTS = ROOT / ".local/cs16/verification"
SPEC = importlib.util.spec_from_file_location("cs16_model_parser", ROOT / "scripts/cs16/models.py")
models = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(models)


NODE_SOURCE = """
import fs from 'node:fs';
import zlib from 'node:zlib';
import { GLTFLoader } from './web/node_modules/three/examples/jsm/loaders/GLTFLoader.js';
import { clone } from './web/node_modules/three/examples/jsm/utils/SkeletonUtils.js';
import * as T from './web/node_modules/three/build/three.module.js';
globalThis.self=globalThis;
globalThis.createImageBitmap=async blob=>{
 const source=new DataView(await blob.arrayBuffer());
 return {width:source.getUint32(16),height:source.getUint32(20),close(){}};
};
const input=JSON.parse(fs.readFileSync(0,'utf8'));
const parsed=new Map();
const reports=[];
for(const test of input){
 let asset=parsed.get(test.file);
 if(!asset){
  const source=zlib.gunzipSync(fs.readFileSync(test.file));
  asset=await new GLTFLoader().parseAsync(source.buffer.slice(source.byteOffset,source.byteOffset+source.byteLength),'');
  parsed.set(test.file,asset);
 }
 const scene=clone(asset.scene);
 const root=new T.Group();
 if(test.cameraAxes){
  root.quaternion.setFromRotationMatrix(new T.Matrix4().makeBasis(new T.Vector3(0,0,-1),new T.Vector3(-1,0,0),new T.Vector3(0,1,0)));
  root.position.set(1.25,-2.5,0.75);
 }
 root.add(scene);
 const clip=asset.animations.find(item=>item.name===test.clip);
 if(!clip)throw Error('Missing original clip '+test.clip);
 const mixer=new T.AnimationMixer(scene);
 const action=mixer.clipAction(clip);
 action.setLoop(T.LoopOnce,1);action.clampWhenFinished=true;action.play();
 mixer.setTime(test.time);root.updateMatrixWorld(true);
 const actual=[];
 scene.traverse(object=>{
  if(!object.isSkinnedMesh)return;
  for(let index=0;index<object.geometry.attributes.position.count;index++){
   const point=object.getVertexPosition(index,new T.Vector3()).applyMatrix4(object.matrixWorld);
   actual.push(point.toArray());
  }
 });
 if(actual.length!==test.positions.length)throw Error('Vertex count mismatch '+test.key+': '+actual.length+' != '+test.positions.length);
 let maximum=0,maximumIndex=-1;
 for(let index=0;index<actual.length;index++){
  const error=Math.max(...actual[index].map((value,axis)=>Math.abs(value-test.positions[index][axis])));
  if(error>maximum){maximum=error;maximumIndex=index;}
 }
 const bounds=values=>({min:[0,1,2].map(axis=>Math.min(...values.map(point=>point[axis]))),max:[0,1,2].map(axis=>Math.max(...values.map(point=>point[axis])))});
 reports.push({key:test.key,clip:test.clip,frame:test.frame,cameraAxes:test.cameraAxes,vertices:actual.length,sourceBounds:bounds(test.positions),threeBounds:bounds(actual),maximumError:maximum,maximumIndex,passed:maximum<=0.001});
}
process.stdout.write(JSON.stringify(reports));
"""


def original_matrix(translation, angles):
    sx, sy, sz = [math.sin(value) for value in angles]
    cx, cy, cz = [math.cos(value) for value in angles]
    return [cz * cy, cz * sy * sx - sz * cx, cz * sy * cx + sz * sx, translation[0], sz * cy, sz * sy * sx + cz * cx, sz * sy * cx - cz * sx, translation[1], -sy, cy * sx, cy * cx, translation[2], 0, 0, 0, 1]


def source_pose(model, sequence, frame):
    raw = model.sequence_bytes(sequence)
    transforms = []
    for index, bone in enumerate(model.bones):
        offset = sequence["animation"] + index * 12
        offsets = models.read_values(raw, "<6H", offset)
        values = [bone["value"][axis] + (models.decode_channel(raw, offset + relative, frame + 1)[frame] if relative else 0) * bone["scale"][axis] for axis, relative in enumerate(offsets)]
        if index == sequence["motionBone"]:
            for axis in range(3):
                if sequence["motionType"] & (1 << axis):
                    values[axis] = 0
        local = original_matrix(values[:3], values[3:])
        if bone["parent"] < 0:
            transforms.append(local)
        else:
            parent = transforms[bone["parent"]]
            transforms.append([sum(parent[row * 4 + step] * local[step * 4 + column] for step in range(4)) for row in range(4) for column in range(4)])
    return transforms


def source_vertices(model, sequence, frame, camera_axes):
    matrices = source_pose(model, sequence, frame)
    result = []
    for body in models.read_array(model.raw, "<64s3i", model.header[17], model.header[16]):
        geometry = models.read_values(model.raw, "<64sif10i", body[3])
        mesh_count, mesh_index, vertex_count, vertex_bones, vertex_index = geometry[3:8]
        vertices = models.read_array(model.raw, "<3f", vertex_index, vertex_count)
        joints = [value[0] for value in models.read_array(model.raw, "<B", vertex_bones, vertex_count)]
        for mesh in models.read_array(model.raw, "<5i", mesh_index, mesh_count):
            position = mesh[1]
            visited = set()
            while True:
                count = models.read_values(model.raw, "<h", position)[0]
                position += 2
                if not count:
                    break
                for entry in models.read_array(model.raw, "<4h", position, abs(count)):
                    if entry in visited:
                        continue
                    visited.add(entry)
                    source = vertices[entry[0]]
                    transform = matrices[joints[entry[0]]]
                    point = [sum(transform[row * 4 + column] * source[column] for column in range(3)) + transform[row * 4 + 3] for row in range(3)]
                    if camera_axes:
                        point = [-point[1] + 1.25, point[2] - 2.5, -point[0] + 0.75]
                    result.append(point)
                position += abs(count) * 8
    return result


class OriginalPoseTests(unittest.TestCase):
    def test_original_source_vertices_match_actual_three_skinning(self):
        cases = []
        for key, clips in [("glock18", ["idle1", "draw", "reload"]), ("ak47", ["idle1", "draw"]), ("m4a1", ["idle", "draw", "reload"])]:
            model = models.StudioModel(SOURCE / ("v_" + key + ".mdl"))
            for clip in clips:
                sequence = next(item for item in model.sequences if item["name"] == clip)
                for frame in sorted({0, sequence["frames"] // 2, sequence["frames"] - 1}):
                    for camera_axes in [False, True]:
                        cases.append({"key": key, "file": str(ROOT / ".local/cs16/assets/models" / ("v-" + key + ".glb.gz")), "clip": clip, "frame": frame, "time": frame / sequence["fps"], "cameraAxes": camera_axes, "positions": source_vertices(model, sequence, frame, camera_axes)})
        execution = subprocess.run(["node", "--input-type=module", "-e", NODE_SOURCE], input=json.dumps(cases), text=True, capture_output=True, cwd=ROOT, check=True)
        results = json.loads(execution.stdout)
        REPORTS.mkdir(parents=True, exist_ok=True)
        report = {"passed": all(item["passed"] for item in results), "scope": "Original MDL source vertices against Three GLTFLoader, SkeletonUtils clone and AnimationMixer CPU skinning; PNG dimensions stubbed, no GPU visual or texture assertion", "cases": results}
        (REPORTS / "model-pose-glb.json").write_text(json.dumps(report, indent=2) + "\n")
        for result in results:
            with self.subTest(**result):
                self.assertTrue(result["passed"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
