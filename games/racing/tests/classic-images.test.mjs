import assert from "node:assert/strict";
import test from "node:test";
import { classicInlineImagePlugin } from "../web/classic-images.ts";

// Failure boundary recorded before implementation: blob/data fetch under CSP,
// decode failure swallowed by GLTFParser, shared source/sampler aliasing,
// wrong dimensions, external image URI and a texture resolving to null.
function fixture({ decodeFails = false, nullTexture = false, uri = false, badSize = false } = {}) {
  let decoded = 0, created = 0, cloned = 0;
  const bytes = new Uint8Array([137,80,78,71,1,2,3,4]).buffer;
  const parser = {
    json: {
      images: [{ bufferView:0,mimeType:"image/png",name:"original",extras:{sourceDimensions:[2,3]},...(uri?{uri:"https://unexpected.example/image.png"}:{}) }],
      textures: [{source:0,sampler:0},{extensions:{EXT_texture_webp:{source:0}},sampler:1}],
    },
    textureLoader: {},
    getDependency: async (kind) => { assert.equal(kind,"bufferView"); return bytes; },
    getDependencies: async () => nullTexture ? [null] : [await parser.loadImageSource(0,{}),await parser.loadImageSource(0,{})],
    loadImageSource: () => { throw Error("original URL loader must never run"); },
    loadTextureImage: async (texture,source,loader) => {
      assert.equal(source,0);assert.equal(loader,parser.textureLoader);
      try { const value=await parser.loadImageSource(source,loader);value.sampler=texture;return value; }
      catch { return null; } // Mirrors Three's permissive catch; plugin must still fail closed.
    },
  };
  const factory = (bitmap) => {
    created++;
    const value = {image:bitmap,userData:{},needsUpdate:false,clone(){cloned++;return {...value,userData:{...value.userData}};}};
    return value;
  };
  const decoder = async (blob,options) => {
    decoded++;
    assert.equal(blob.type,"image/png");
    assert.deepEqual(new Uint8Array(await blob.arrayBuffer()),new Uint8Array(bytes));
    assert.equal(options.premultiplyAlpha,"none");
    assert.equal(options.colorSpaceConversion,"none");
    if(decodeFails)throw Error("decoder failed");
    return {width:badSize?1:2,height:3,close(){}};
  };
  const plugin=classicInlineImagePlugin(parser,factory,{decode:decoder});
  return {parser,plugin,counts:()=>({decoded,created,cloned})};
}

test("verified embedded image decoding does not fetch URLs and preserves clone/sampler ownership",async()=>{
  const value=fixture();
  const originalFetch=globalThis.fetch;
  globalThis.fetch=()=>{throw Error("CSP forbids image fetch")};
  try {
    await value.plugin.beforeRoot();
    assert.equal(value.plugin.name,"EXT_texture_webp");
    assert.equal(value.plugin.loadTexture(0),null);
    const [one,two]=await Promise.all([value.parser.loadTextureImage(0,0,value.parser.textureLoader),value.plugin.loadTexture(1)]);
    assert.notEqual(one,two);assert.equal(one.image,two.image);
    assert.equal(one.sampler,0);assert.equal(two.sampler,1);
    await value.plugin.afterRoot({});
    assert.equal(value.counts().decoded,1);assert.equal(value.counts().created,1);
    assert.ok(value.counts().cloned>=2);
  } finally {globalThis.fetch=originalFetch;}
});

test("an image decode failure rejects initialization instead of producing white models",async()=>{
  await assert.rejects(fixture({decodeFails:true}).plugin.beforeRoot(),/decoder failed/);
});

test("missing texture and mismatched source image dimensions reject readiness",async()=>{
  const value=fixture({nullTexture:true});await value.plugin.beforeRoot();
  await assert.rejects(value.plugin.afterRoot({}),/贴图|texture/i);
  await assert.rejects(fixture({badSize:true}).plugin.beforeRoot(),/尺寸|dimension/i);
});

test("external image references cannot bypass the verified file map",async()=>{
  await assert.rejects(fixture({uri:true}).plugin.beforeRoot(),/内嵌|embedded/i);
});
