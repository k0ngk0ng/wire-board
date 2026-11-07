"""Failure inventory before native/runtime build preparation:
same rules with changed bridge/toolchain/loader reuse an immutable URL; an old
or mutated manifest supplies linker flags; missing/noncanonical IDs pass; source
inputs escape the checkout; native compilation sees different rules/toolchain;
Wasm retains native-only pixels or eight full stage payloads. Actual compiles and
offline startup are additionally exercised by accept_production.py.
"""
import copy,hashlib,unittest
from identity import encode_manifest, native_flags
class IdentityTest(unittest.TestCase):
 def manifest(self):return {'version':1,'rulesBuildID':'a'*64,'visualsID':'b'*64,'runtime':{'goVersion':'go1.26.0','sourceSHA256':'c'*64,'builderSHA256':'d'*64},'loader':{'sha256':'e'*64},'wasm':{'sha256':'f'*64}}
 def test_source_toolchain_and_payload_address_separately_from_rules(self):
  original=self.manifest();identity,raw=encode_manifest(original);self.assertEqual(identity,hashlib.sha256(raw).hexdigest());self.assertEqual((identity,raw),encode_manifest(dict(reversed(list(original.items())))))
  for key,value in [('sourceSHA256','d'*64),('goVersion','go1.26.1'),('builderSHA256','e'*64)]:
   changed=copy.deepcopy(original);changed['runtime'][key]=value;self.assertNotEqual(identity,encode_manifest(changed)[0]);self.assertEqual(original['rulesBuildID'],changed['rulesBuildID'])
  for key in ['loader','wasm']:
   changed=copy.deepcopy(original);changed[key]['sha256']='1'*64;self.assertNotEqual(identity,encode_manifest(changed)[0])
 def test_flags_bind_both_identities_without_self_referential_wasm(self):
  identity,_=encode_manifest(self.manifest());flags=native_flags('a'*64,identity);self.assertIn('CompiledRulesBuildID='+'a'*64,flags);self.assertIn('CompiledRuntimeBuildID='+identity,flags)
  for args in [('',identity),('A'*64,identity),('a'*64,''),('a'*63,identity)]:
   with self.assertRaises(ValueError):native_flags(*args)
if __name__=='__main__':unittest.main()
