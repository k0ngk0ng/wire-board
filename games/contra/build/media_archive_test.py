"""Failure inventory: nondeterministic packs, missing clean-checkout media,
corrupt/stale package pins, extra or missing members and escaping archive names.
"""
import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from media import inventory
from media_archive import pack_media, unpack_media, prepare_source_media

ROOT=Path(__file__).resolve().parents[3]
class MediaArchiveTest(unittest.TestCase):
    def setUp(self):
        parent=ROOT/'.local/contra-media-archive-tests';parent.mkdir(parents=True,exist_ok=True)
        self.temp=tempfile.TemporaryDirectory(dir=parent);self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name);assets=self.root/'games/contra/assets';assets.mkdir(parents=True)
        self.payload=b'RIFF-original-byte-exact-audio'
        self.digest=hashlib.sha256(self.payload).hexdigest()
        stem={'file':'.local/research/sound.wav','sha256':self.digest}
        (assets/'audio-source.json').write_text(json.dumps({'sounds':[{'id':2,'stems':[stem]}]}))
        (assets/'audio-loops.json').write_text(json.dumps({'songs':[]}))
        self.media=self.root/'media';self.media.mkdir();(self.media/(self.digest+'.wav')).write_bytes(self.payload)
        (self.media/'manifest.json').write_text(json.dumps(inventory(self.root,self.media)))

    def test_deterministic_roundtrip_without_research_directory(self):
        a,b=self.root/'one.tar.xz',self.root/'two.tar.xz'
        pin=pack_media(self.root,self.media,a)
        self.assertEqual(pin,pack_media(self.root,self.media,b))
        self.assertEqual(a.read_bytes(),b.read_bytes())
        out=self.root/'clean-cache'
        unpack_media(self.root,a,pin,out)
        self.assertEqual((out/(self.digest+'.wav')).read_bytes(),self.payload)
        self.assertFalse((self.root/'.local/research').exists())
        unpack_media(self.root,a,pin,out) # Reuse only verified content.
        (out/(self.digest+'.wav')).write_bytes(b'broken')
        with self.assertRaises(ValueError):unpack_media(self.root,a,pin,out)

    def test_corrupt_and_stale_pins_are_rejected(self):
        archive=self.root/'media.tar.xz';pin=pack_media(self.root,self.media,archive)
        archive.write_bytes(archive.read_bytes()+b'corruption')
        with self.assertRaises(ValueError):unpack_media(self.root,archive,pin,self.root/'out')
        self.assertFalse((self.root/'out').exists())

    def test_default_build_media_comes_from_source_package(self):
        assets=self.root/'games/contra/assets'
        archive=assets/'audio-pack-v1.tar.xz';pin=pack_media(self.root,self.media,archive)
        (assets/'audio-pack.json').write_text(json.dumps(pin))
        out=prepare_source_media(self.root)
        self.assertEqual((out/(self.digest+'.wav')).read_bytes(),self.payload)
        self.assertTrue(out.is_relative_to(self.root/'.local'))
        archive.unlink()
        with self.assertRaises(FileNotFoundError):prepare_source_media(self.root)

    def test_unlisted_and_escaping_members_never_extract(self):
        for name in ['../escape.wav','extra.wav']:
            archive=self.root/('bad'+str(len(name))+'.tar.xz')
            with tarfile.open(archive,'w:xz') as tar:
                for member,data in [('manifest.json',(self.media/'manifest.json').read_bytes()),(name,b'bad')]:
                    info=tarfile.TarInfo(member);info.size=len(data);tar.addfile(info,io.BytesIO(data))
            raw=archive.read_bytes();pin={'format':'contra-audio-source-pack-v1','bytes':len(raw),
                 'sha256':hashlib.sha256(raw).hexdigest(),'manifestSHA256':hashlib.sha256((self.media/'manifest.json').read_bytes()).hexdigest()}
            with self.assertRaises(ValueError):unpack_media(self.root,archive,pin,self.root/'out')
            self.assertFalse((self.root/'out').exists())

if __name__=='__main__':unittest.main()
