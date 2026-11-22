import * as T from 'three';

export class MapFlashlight {
  readonly uniforms = {
    csFlashOrigin: { value: new T.Vector3() },
    csFlashDirection: { value: new T.Vector3(0, 0, -1) },
    csFlashPower: { value: 0 },
    csNightVisionPower: { value: 0 },
  };
  apply(material: T.MeshBasicMaterial) {
    material.onBeforeCompile = shader => {
      Object.assign(shader.uniforms, this.uniforms);
      shader.vertexShader = shader.vertexShader.replace('#include <common>', '#include <common>\nvarying vec3 csFlashPosition;').replace('#include <project_vertex>', '#include <project_vertex>\ncsFlashPosition = (modelMatrix * vec4(transformed, 1.0)).xyz;');
      shader.fragmentShader = shader.fragmentShader.replace('#include <common>', '#include <common>\nvarying vec3 csFlashPosition;\nuniform vec3 csFlashOrigin;\nuniform vec3 csFlashDirection;\nuniform float csFlashPower;\nuniform float csNightVisionPower;').replace('#include <opaque_fragment>', `
        if (csFlashPower > 0.0) {
          vec3 csDelta = csFlashPosition - csFlashOrigin;
          float csDistance = length(csDelta);
          float csCone = smoothstep(0.975, 0.998, dot(normalize(csDelta), csFlashDirection));
          float csFalloff = max(0.0, 1.0 - csDistance / 1024.0);
          outgoingLight += diffuseColor.rgb * vec3(1.0, 0.96, 0.86) * csCone * csFalloff * csFalloff * csFlashPower;
        }
        if (csNightVisionPower > 0.0) {
          float csNightDistance = length(csFlashPosition - csFlashOrigin);
          float csNightFalloff = max(0.0, 1.0 - csNightDistance / 750.0);
          outgoingLight += diffuseColor.rgb * vec3(0.12, 1.0, 0.12) * csNightFalloff * csNightVisionPower;
        }
        #include <opaque_fragment>
      `);
    };
    material.customProgramCacheKey = () => 'cs16-map-flashlight-nightvision-2';
  }
  update(origin: T.Vector3, direction: T.Vector3, enabled: boolean, nightVision = false) {
    this.uniforms.csFlashOrigin.value.copy(origin);
    this.uniforms.csFlashDirection.value.copy(direction);
    this.uniforms.csFlashPower.value = enabled ? 1 : 0;
    this.uniforms.csNightVisionPower.value = nightVision ? 1 : 0;
  }
}
