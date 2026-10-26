"""Research-only frontend for the pinned Genesis Plus GX reference core."""
import ctypes as C
import hashlib
import importlib.util
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ROM_SHA256 = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
spec = importlib.util.spec_from_file_location('md_original_probe', ROOT/'docs/research/md-yuyuhakusho/probe.py')
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def buttons_at(events, frame):
    result = [0] * 8
    for event in events:
        if event['start'] <= frame <= event['end']:
            for button in event['buttons']:
                result[event.get('port', 0)] |= 1 << button
    return result


class CPUHead(C.Structure):
    # Pinned core/m68k/m68k.h prefix. Pointer size/alignment supplied by ctypes.
    _fields_ = [('memory_map', C.c_void_p * (256 * 5)), ('poll', C.c_uint * 3),
                ('cycles', C.c_int), ('refresh_cycles', C.c_int), ('cycle_end', C.c_uint),
                ('dar', C.c_uint * 16), ('pc', C.c_uint)]


class ReferenceSession:
    def __init__(self, root=ROOT, output=None, core=None, teamplayer=False):
        self.root = Path(root).resolve()
        self.output = Path(output or self.root/'.local/md-yuyuhakusho/reference').resolve()
        if not self.output.is_relative_to(self.root):
            raise ValueError('output must remain inside worktree')
        self.output.mkdir(parents=True, exist_ok=True)
        self.core_path = Path(core or self.root/'.local/md-yuyuhakusho/tools/genesis_plus_gx_libretro.dylib').resolve()
        self.rom_path = self.root/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
        rom = self.rom_path.read_bytes()
        if sha(rom) != ROM_SHA256:
            raise ValueError('ROM differs from pinned research target')
        self.lib = C.CDLL(str(self.core_path))
        self.image, self.width, self.height, self.pixel_format = b'', 0, 0, 0
        self.frame, self.buttons, self.callbacks = 0, [0] * 8, []
        self.directory = str(self.output).encode()
        def callback(name, result, parameters, function):
            typ = C.CFUNCTYPE(result, *parameters)
            cb = typ(function)
            self.callbacks.append(cb)
            self.api('retro_set_'+name, None, typ)(cb)
        callback('environment', C.c_bool, [C.c_uint, C.c_void_p], self.environment)
        callback('video_refresh', None, [C.c_void_p, C.c_uint, C.c_uint, C.c_size_t], self.video)
        callback('audio_sample', None, [C.c_int16, C.c_int16], lambda a, b: None)
        callback('audio_sample_batch', C.c_size_t, [C.c_void_p, C.c_size_t], lambda p, n: n)
        callback('input_poll', None, [], lambda: None)
        callback('input_state', C.c_int16, [C.c_uint]*4, self.input)
        if self.api('retro_api_version', C.c_uint)() != 1:
            raise ValueError('Unexpected libretro ABI')
        info = probe.Info()
        self.api('retro_get_system_info', None, C.POINTER(probe.Info))(C.byref(info))
        self.api('retro_init', None)()
        self.rom_buffer = C.create_string_buffer(rom)
        game = probe.Game(str(self.rom_path).encode(), C.cast(self.rom_buffer, C.c_void_p), len(rom), None)
        if not self.api('retro_load_game', C.c_bool, C.POINTER(probe.Game))(C.byref(game)):
            raise RuntimeError('ROM load failed')
        controller = self.api('retro_set_controller_port_device', None, C.c_uint, C.c_uint)
        controller(0, 1793 if teamplayer else 513)
        controller(1, 513)
        av = probe.AV()
        self.api('retro_get_system_av_info', None, C.POINTER(probe.AV))(C.byref(av))
        self.ram_size = self.api('retro_get_memory_size', C.c_size_t, C.c_uint)(2)
        self.ram_pointer = self.api('retro_get_memory_data', C.c_void_p, C.c_uint)(2)
        if self.ram_size != 65536 or not self.ram_pointer:
            raise ValueError('Unexpected MD RAM mapping')
        self.cpu = CPUHead.in_dll(self.lib, 'm68k')
        self.get_register = self.api('m68k_get_reg', C.c_uint, C.c_uint)
        self.run_frame = self.api('retro_run', None)
        self.identity = {'rom_sha256': sha(rom), 'core_sha256': sha(self.core_path.read_bytes()),
                         'core_name': info.name.decode(), 'core_version': info.version.decode(),
                         'fps': av.timing.fps, 'ram_bytes': self.ram_size,
                         'frontend_sha256': sha(Path(__file__).read_bytes()), 'teamplayer': teamplayer}
        self.closed = False

    def api(self, name, result, *parameters):
        f = getattr(self.lib, name)
        f.restype, f.argtypes = result, list(parameters)
        return f

    def environment(self, command, data):
        if command in (9, 30, 31):
            C.cast(data, C.POINTER(C.c_char_p))[0] = self.directory
            return True
        if command == 10:
            fmt = C.cast(data, C.POINTER(C.c_int))[0]
            if fmt not in (0, 1, 2):
                return False
            self.pixel_format = fmt
            return True
        if command in (3, 17):
            C.cast(data, C.POINTER(C.c_bool))[0] = command == 3
            return True
        if command == (47 | 0x10000):
            C.cast(data, C.POINTER(C.c_int))[0] = 7
            return True
        if command == 15:
            C.cast(data, C.POINTER(C.c_void_p))[1] = None
        return False

    def video(self, data, width, height, pitch):
        if data:
            row = width * (4 if self.pixel_format == 1 else 2)
            self.image = b''.join(C.string_at(data+y*pitch, row) for y in range(height))
            self.width, self.height = width, height

    def input(self, port, device, index, button):
        if device != 1 or index or port >= len(self.buttons):
            return 0
        return self.buttons[port] if button == 256 else int(bool(self.buttons[port] & (1 << button)))

    def ram_native(self):
        return C.string_at(self.ram_pointer, self.ram_size)

    def ram_cpu(self):
        # This arm64 little-endian build stores each 68000 word byte-swapped.
        raw = self.ram_native()
        data = bytearray(len(raw))
        data[0::2], data[1::2] = raw[1::2], raw[0::2]
        return bytes(data)

    def step(self, frame, buttons):
        self.frame, self.buttons = frame, list(buttons)
        self.run_frame()
        regs = [self.get_register(i) for i in range(21)]
        if self.cpu.pc != regs[16] or list(self.cpu.dar) != regs[:16]:
            raise RuntimeError('Pinned CPU prefix no longer matches core ABI')
        ram_hash, video_hash = sha(self.ram_native()), sha(self.image)
        return {'reference': {'frame': frame, 'ram': ram_hash, 'video': video_hash,
                              'dimensions': [self.width, self.height]},
                'ram': ram_hash, 'video': video_hash, 'registers': regs}

    def screenshot(self, path):
        path = Path(path).resolve()
        if not path.is_relative_to(self.root):
            raise ValueError('screenshot must remain inside worktree')
        probe.png(path, self.image, self.width, self.height, self.pixel_format)

    def save(self):
        size = self.api('retro_serialize_size', C.c_size_t)()
        data = C.create_string_buffer(size)
        if not self.api('retro_serialize', C.c_bool, C.c_void_p, C.c_size_t)(data, size):
            raise RuntimeError('serialize failed')
        return {'data': data.raw, 'refresh_cycles': self.cpu.refresh_cycles,
                'core_sha256': self.identity['core_sha256'], 'frame': self.frame}

    def restore(self, checkpoint, restore_refresh_cycles=False):
        if checkpoint['core_sha256'] != self.identity['core_sha256']:
            raise ValueError('checkpoint core identity mismatch')
        data = C.create_string_buffer(checkpoint['data'], len(checkpoint['data']))
        if not self.api('retro_unserialize', C.c_bool, C.c_void_p, C.c_size_t)(data, len(data)):
            raise RuntimeError('unserialize failed')
        if restore_refresh_cycles:
            # Upstream state.c saves cycles/int_level/stopped, but omits this
            # bus refresh deadline used by m68k_run. No game RAM is rewritten.
            self.cpu.refresh_cycles = checkpoint['refresh_cycles']
        self.frame = checkpoint['frame']

    def close(self):
        if not self.closed:
            self.api('retro_unload_game', None)()
            self.api('retro_deinit', None)()
            self.closed = True

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
