import * as T from "three";

type Config = { steeringRatio: number; maxRPM: number; maxSpeedKmh: number };
type Binding = {
  part: string; kind: "steering" | "rpm" | "speed"; node: T.Object3D; joint: T.Group;
  axis: T.Vector3; base: T.Quaternion; excursion: number; angle: number;
  parent: T.Object3D; position: T.Vector3; quaternion: T.Quaternion; scale: T.Vector3;
};
const finite = (value: number, fallback = 0) => Number.isFinite(value) ? value : fallback;
const clamp = (value: number, low: number, high: number) => Math.max(low,Math.min(high,value));

/** The thin steering plane is observed in native vertices, not assumed +Z. */
function thinAxis(points: readonly T.Vector3[], centre: T.Vector3): T.Vector3 {
  const matrix = [[0,0,0],[0,0,0],[0,0,0]], vectors = [[1,0,0],[0,1,0],[0,0,1]];
  for (const point of points) {
    const v=[point.x-centre.x,point.y-centre.y,point.z-centre.z];
    for(let i=0;i<3;i++)for(let j=0;j<3;j++)matrix[i][j]+=v[i]*v[j];
  }
  for(let step=0;step<24;step++){
    let p=0,q=1;
    for(const [i,j] of [[0,2],[1,2]])if(Math.abs(matrix[i][j])>Math.abs(matrix[p][q])){p=i;q=j;}
    if(Math.abs(matrix[p][q])<1e-12)break;
    const angle=.5*Math.atan2(2*matrix[p][q],matrix[q][q]-matrix[p][p]),c=Math.cos(angle),s=Math.sin(angle);
    const a=matrix[p][p],b=matrix[q][q],d=matrix[p][q];
    matrix[p][p]=c*c*a-2*s*c*d+s*s*b;matrix[q][q]=s*s*a+2*s*c*d+c*c*b;matrix[p][q]=matrix[q][p]=0;
    for(let k=0;k<3;k++)if(k!==p&&k!==q){const x=matrix[k][p],y=matrix[k][q];matrix[k][p]=matrix[p][k]=c*x-s*y;matrix[k][q]=matrix[q][k]=s*x+c*y;}
    for(let k=0;k<3;k++){const x=vectors[k][p],y=vectors[k][q];vectors[k][p]=c*x-s*y;vectors[k][q]=s*x+c*y;}
  }
  let index=0;for(let i=1;i<3;i++)if(matrix[i][i]<matrix[index][index])index=i;
  const axis=new T.Vector3(vectors[0][index],vectors[1][index],vectors[2][index]).normalize();
  return axis.z<0?axis.negate():axis;
}

/** Reparents original meshes around measured local hubs; no source buffer edit. */
export class ClassicCockpitDynamics {
  private readonly config: Config;
  private readonly bindings: Binding[] = [];
  private readonly rotation = new T.Quaternion();
  constructor(cockpit: T.Object3D, config: Config) {
    this.config=config;cockpit.updateWorldMatrix(true,true);
    const nodes:T.Object3D[]=[];
    cockpit.traverse(node=>{if(node.userData.originalPart==="steering"||/^:F_(RPM|MPH)\s*\(/.test(String(node.userData.originalPart)))nodes.push(node);});
    for(const node of nodes){
      const part=String(node.userData.originalPart),match=/^:F_(RPM|MPH)\s*\(([\d.]+)\s+to\s+([\d.]+)\)$/.exec(part);
      const kind=part==="steering"?"steering":match?.[1]==="RPM"?"rpm":"speed";
      if(!node.parent||(!match&&kind!=="steering"))throw Error("原驾驶舱动作标记无效");
      const points:T.Vector3[]=[],inverse=node.matrixWorld.clone().invert();
      node.traverse(mesh=>{if(!(mesh instanceof T.Mesh))return;
        const attribute=mesh.geometry.getAttribute("position"),matrix=new T.Matrix4().multiplyMatrices(inverse,mesh.matrixWorld);
        for(let i=0;i<attribute.count;i++)points.push(new T.Vector3().fromBufferAttribute(attribute,i).applyMatrix4(matrix));
      });
      if(!points.length)throw Error("原驾驶舱动作缺少几何");
      let pivot:T.Vector3,axis:T.Vector3;
      if(kind==="steering"){
        pivot=new T.Box3().setFromPoints(points).getCenter(new T.Vector3());axis=thinAxis(points,pivot);
      }else{
        if(points.length!==3)throw Error("原仪表针不是已核对的三角部件");
        const pairs=[[0,1],[0,2],[1,2]].sort((a,b)=>points[a[0]].distanceToSquared(points[a[1]])-points[b[0]].distanceToSquared(points[b[1]]));
        pivot=points[pairs[0][0]].clone().add(points[pairs[0][1]]).multiplyScalar(.5);
        axis=new T.Vector3().crossVectors(points[1].clone().sub(points[0]),points[2].clone().sub(points[0])).normalize();
        if(axis.z<0)axis.negate();
        if(axis.lengthSq()<.99)throw Error("原仪表针盘面无效");
      }
      const parent=node.parent,position=node.position.clone(),quaternion=node.quaternion.clone(),scale=node.scale.clone();
      const joint=new T.Group();joint.name=`classic_cockpit_joint_${kind}`;
      joint.position.copy(pivot).multiply(scale).applyQuaternion(quaternion).add(position);
      joint.quaternion.copy(quaternion);joint.scale.copy(scale);parent.add(joint);joint.add(node);
      node.position.copy(pivot).negate();node.quaternion.identity();node.scale.setScalar(1);
      this.bindings.push({part,kind,node,joint,axis,base:quaternion.clone(),excursion:match?(Number(match[3])-Number(match[2]))*Math.PI*2:0,angle:0,parent,position,quaternion,scale});
    }
  }
  update(reading: { steeringAngle: number; speedKmh: number; rpm: number }) {
    for(const item of this.bindings){
      const angle=item.kind==="steering"
        ? clamp(finite(reading.steeringAngle)*this.config.steeringRatio,-Math.PI,Math.PI)
        : item.excursion*clamp(finite(item.kind==="rpm"?reading.rpm:Math.abs(reading.speedKmh))/Math.max(1,item.kind==="rpm"?this.config.maxRPM:this.config.maxSpeedKmh),0,1);
      item.joint.quaternion.copy(item.base).multiply(this.rotation.setFromAxisAngle(item.axis,angle));item.angle=angle;
    }
  }
  inspect() {
    return this.bindings.map(item=>{
      item.joint.updateWorldMatrix(true,false);
      return {part:item.part,node:item.node.name,kind:item.kind,angle:item.angle,excursion:item.excursion,
        pivot:new T.Vector3().setFromMatrixPosition(item.joint.matrixWorld).toArray(),
        axis:item.axis.clone().transformDirection(item.joint.matrixWorld).toArray()};
    });
  }
  dispose() {
    for(const item of this.bindings){item.node.removeFromParent();item.parent.add(item.node);item.node.position.copy(item.position);item.node.quaternion.copy(item.quaternion);item.node.scale.copy(item.scale);item.joint.removeFromParent();}
    this.bindings.length=0;
  }
}
