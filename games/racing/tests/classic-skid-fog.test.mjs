import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "../../../web/node_modules/typescript/lib/typescript.js";
import * as T from "../../../web/node_modules/three/build/three.module.js";
import * as skids from "../web/classic-skids.ts";

// Failure before the fix: a fog-enabled ShaderMaterial lacked Three's fogColor,
// fogDensity, fogNear and fogFar uniforms. The first visible strip then crashed
// WebGLMaterials.refreshFogUniforms. Exercise the real constructor without a GPU.
const source=readFileSync(new URL("../web/classic-skid-renderer.ts",import.meta.url),"utf8");
const compiled=ts.transpileModule(source,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.CommonJS}}).outputText;
const exports={};
new Function("require","exports",compiled)(name=>{
  if(name==="three")return T;
  if(name==="./classic-skids")return skids;
  throw Error(`Unexpected dependency ${name}`);
},exports);

test("visible skid materials own every fog uniform required by Three's renderer",()=>{
  const scene=new T.Scene(),renderer=new exports.ClassicSkidRenderer(scene);
  const material=scene.getObjectByName("classic_tire_marks").material;
  assert.equal(material.fog,true);
  for(const key of ["fogColor","fogDensity","fogNear","fogFar"])
    assert(material.uniforms[key],`fog-enabled skid material is missing ${key}`);
  assert(material.uniforms.fogColor.value instanceof T.Color);
  assert.notEqual(material.uniforms.fogColor,T.UniformsLib.fog.fogColor);
  assert.notEqual(material.uniforms.fogColor.value,T.UniformsLib.fog.fogColor.value);
  const fog=new T.FogExp2("#6793b1",.0002);
  material.uniforms.fogColor.value.copy(fog.color);
  material.uniforms.fogDensity.value=fog.density;
  assert.equal(material.uniforms.fogDensity.value,.0002);
  assert(material.uniforms.skidNow&&material.uniforms.skidLife);
  renderer.dispose();assert.equal(scene.children.length,0);
});
