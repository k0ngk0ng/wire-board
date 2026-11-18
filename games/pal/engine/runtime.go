package engine

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	Delay             = 1
	Dialogue          = 2
	SceneComplete     = 3
	Frame             = 4
	SystemMenu        = 5
	CooperativeAction = 6
	GameOver          = 7
	FieldMode         = 0
	ScriptMode        = 1
	BattleMode        = 2
	MenuMode          = 3
	maxMemory         = 128 * 1024 * 1024
	maxEncoded        = 32 * 1024 * 1024
	maxJSON           = maxMemory*4/3 + maxSnapshotMetadata
)

type Event struct {
	Kind  int `json:"kind"`
	Value int `json:"value"`
}
type StateView struct {
	Audio          *AudioJournal    `json:"audio,omitempty"`
	GameOver       *GameOverState   `json:"gameOver,omitempty"`
	Ending         *EndingState     `json:"ending,omitempty"`
	Field          *FieldState      `json:"field,omitempty"`
	PaletteEffect  *PaletteEffect   `json:"paletteEffect"`
	SceneReady     bool             `json:"sceneReady"`
	ClearColor     int              `json:"clearColor"`
	Raster         *RasterGraph     `json:"raster,omitempty"`
	Cooperation    CooperativeMagic `json:"cooperation"`
	Battle         BattleState      `json:"battle"`
	BattleAssets   [][2]int         `json:"battleAssets,omitempty"`
	Menu           MenuView         `json:"menu"`
	Overlay        []UIOp           `json:"overlay"`
	DialogWaiting  bool             `json:"dialogWaiting"`
	DialogLine     int              `json:"dialogLine"`
	DialogPage     []int            `json:"dialogPage"`
	CameraX        int              `json:"cameraX"`
	CameraY        int              `json:"cameraY"`
	Map            int              `json:"map"`
	Palette        int              `json:"palette"`
	Night          bool             `json:"night"`
	Face           int              `json:"face"`
	DialogPosition int              `json:"dialogPosition"`
	Draws          []DrawOp         `json:"draws"`
	HeldDirection  int              `json:"heldDirection"`
	PendingKeys    uint32           `json:"pendingKeys"`
	Keys           uint32           `json:"keys"`
	Mode           int              `json:"mode"`
	Entering       bool             `json:"entering"`
	MovingPlayer   int              `json:"movingPlayer"`
	Scene          int              `json:"scene"`
	X              int              `json:"x"`
	Y              int              `json:"y"`
	Party          []int            `json:"party"`
	Sprite         int              `json:"sprite"`
	HP             int              `json:"hp"`
	MP             int              `json:"mp"`
	Cash           uint32           `json:"cash"`
	Seed           uint32           `json:"seed"`
	Clock          uint32           `json:"clock"`
	PC             uint32           `json:"pc"`
	Messages       uint32           `json:"messages"`
}

type PaletteEffect struct {
	RedStep int   `json:"redStep,omitempty"`
	Palette int   `json:"palette"`
	Night   bool  `json:"night"`
	Factors []int `json:"factors"`
}

type RasterGraph struct {
	Version int         `json:"version"`
	Root    int         `json:"root"`
	Nodes   [][16]int   `json:"nodes"`
	Base    *RasterBase `json:"base,omitempty"`
	Backup  *RasterBase `json:"backup,omitempty"`
}

type CooperativeMagic struct {
	Pending         bool   `json:"pending"`
	Player          int    `json:"player"`
	Magic           int    `json:"magic"`
	Target          int    `json:"target"`
	AffectedMask    int    `json:"affectedMask"`
	ContributorMask int    `json:"contributorMask"`
	HPCost          int    `json:"hpCost"`
	Name            string `json:"name"`
}

type BattleState struct {
	Active   bool           `json:"active"`
	Result   int            `json:"result"`
	Phase    int            `json:"phase"`
	State    int            `json:"state"`
	Menu     int            `json:"menu"`
	Current  int            `json:"current"`
	Selected int            `json:"selected"`
	Action   int            `json:"action"`
	Object   int            `json:"object"`
	Target   int            `json:"target"`
	Auto     bool           `json:"auto"`
	Repeat   bool           `json:"repeat"`
	Force    bool           `json:"force"`
	Flee     bool           `json:"flee"`
	Players  []BattlePlayer `json:"players"`
	Enemies  []BattleEnemy  `json:"enemies"`
}
type BattlePlayer struct {
	Role      int  `json:"role"`
	HP        int  `json:"hp"`
	MaxHP     int  `json:"maxHP"`
	MP        int  `json:"mp"`
	MaxMP     int  `json:"maxMP"`
	Level     int  `json:"level"`
	State     int  `json:"state"`
	Action    int  `json:"action"`
	Object    int  `json:"object"`
	Target    int  `json:"target"`
	Frame     int  `json:"frame"`
	Defending bool `json:"defending"`
}
type BattleEnemy struct {
	Object     int `json:"object"`
	HP         int `json:"hp"`
	State      int `json:"state"`
	Frame      int `json:"frame"`
	X          int `json:"x"`
	Y          int `json:"y"`
	ColorShift int `json:"colorShift"`
}
type MenuItem struct {
	Value   int    `json:"value"`
	Word    int    `json:"word"`
	Enabled bool   `json:"enabled"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Label   string `json:"label"`
}
type MenuView struct {
	Kind     int        `json:"kind"`
	Depth    int        `json:"depth"`
	Selected int        `json:"selected"`
	Role     int        `json:"role"`
	Items    []MenuItem `json:"items"`
}

type UIOp struct {
	Kind   int `json:"kind"`
	Source int `json:"source"`
	ID     int `json:"id"`
	Frame  int `json:"frame"`
	X      int `json:"x"`
	Y      int `json:"y"`
	Color  int `json:"color"`
	Small  int `json:"small"`
}
type DrawOp struct {
	Kind  int `json:"kind"`
	A     int `json:"a"`
	B     int `json:"b"`
	C     int `json:"c"`
	D     int `json:"d"`
	X     int `json:"x"`
	BaseY int `json:"baseY"`
	SortY int `json:"sortY"`
}
type Snapshot struct {
	Entry        string    `json:"entry"`
	Version      int       `json:"version"`
	BuildID      string    `json:"buildID"`
	ResourceID   string    `json:"resourceID"`
	MemoryID     string    `json:"memoryID"`
	StackPointer uint32    `json:"stackPointer"`
	Memory       []byte    `json:"memory"`
	Event        Event     `json:"event"`
	View         StateView `json:"view"`
}
type Input struct {
	Direction int
	Keys      uint32
	Response  uint32
}
type TimedResult struct {
	Audio       []TimedAudioCommand
	Snapshot    Snapshot
	RemainingMS uint32
	Steps       int
	InputReset  bool
}
type Projection struct {
	StateView
	Event      Event    `json:"event"`
	Text       string   `json:"text,omitempty"`
	PartyNames []string `json:"partyNames"`
}
type resourceFile struct {
	name string
	data []byte
}
type packedText struct {
	data    []byte
	offsets []uint32
}
type execution struct {
	event    *Event
	response uint64
}
type executionKey struct{}

type Runtime struct {
	vm                      wazero.Runtime
	program                 wazero.CompiledModule
	buildID, resourceID     string
	minimum                 uint32
	sequence                atomic.Uint64
	files                   []resourceFile
	words, messages         []string
	wordTable, messageTable packedText
	roleNames               []string
	provenance              *buildStamp
}

func identity(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func snapshotIdentity(memory []byte, stack uint32) string {
	hash := sha256.New()
	hash.Write(memory)
	var pointer [4]byte
	binary.LittleEndian.PutUint32(pointer[:], stack)
	hash.Write(pointer[:])
	return hex.EncodeToString(hash.Sum(nil))
}

func NewRuntime(ctx context.Context, program []byte, files fs.FS) (*Runtime, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	buildID := identity(program)
	provenance, stamped, err := readBuildStamp(program, buildID)
	if err != nil {
		return nil, err
	}
	tables, err := resources.Load98Tables(files)
	if err != nil {
		return nil, err
	}
	r := &Runtime{buildID: buildID, provenance: provenance}
	hash := sha256.New()
	for _, name := range []string{"sss.mkf", "data.mkf", "word.dat", "m.msg"} {
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, err
		}
		hash.Write([]byte(name))
		hash.Write([]byte{0})
		binary.Write(hash, binary.LittleEndian, uint64(len(data)))
		hash.Write(data)
		if name == "sss.mkf" || name == "data.mkf" {
			r.files = append(r.files, resourceFile{name, bytes.Clone(data)})
		}
	}
	r.resourceID = hex.EncodeToString(hash.Sum(nil))
	bundle, err := resources.BuildRuleBundle(files)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(bundle.Files))
	for name := range bundle.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		hash.Write([]byte(name))
		hash.Write(bundle.Files[name])
	}
	r.resourceID = hex.EncodeToString(hash.Sum(nil))
	return r.initializeRulesRuntime(ctx, program, tables, bundle.Files, stamped)
}

func (r *Runtime) initializeRulesRuntime(ctx context.Context, program []byte, tables *resources.Tables, native map[string][]byte, stamped bool) (*Runtime, error) {
	names := make([]string, 0, len(native))
	for name := range native {
		names = append(names, name)
	}
	sort.Strings(names)
	r.files = nil
	for _, name := range names {
		r.files = append(r.files, resourceFile{name, bytes.Clone(native[name])})
	}
	var err error
	decode := func(input [][]byte) ([]string, error) {
		result := make([]string, len(input))
		for i, data := range input {
			text, err := simplifiedchinese.GBK.NewDecoder().String(string(data))
			if err != nil {
				return nil, err
			}
			result[i] = text
		}
		return result, nil
	}
	if r.words, err = decode(tables.Words); err != nil {
		return nil, err
	}
	if r.messages, err = decode(tables.Messages); err != nil {
		return nil, err
	}
	r.roleNames = make([]string, 6)
	for role := range r.roleNames {
		word := int(tables.PlayerRoles[3][role])
		if word >= len(r.words) {
			return nil, errors.New("invalid original character name")
		}
		r.roleNames[role] = r.words[word]
	}
	r.wordTable, r.messageTable = packText(r.words), packText(r.messages)
	r.vm = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigCompiler().WithMemoryLimitPages(maxMemory/65536).WithCloseOnContextDone(true))
	fail := func(err error) (*Runtime, error) { r.vm.Close(context.Background()); return nil, err }
	r.program, err = r.vm.CompileModule(ctx, program)
	if err != nil {
		return fail(err)
	}
	for _, name := range []string{"_initialize", "wb_allocate", "wb_bind_file", "wb_bind_text", "wb_boot", "wb_run_opening", "wb_run_campaign", "wb_input", "wb_release", "wb_budget", "wb_view", "wb_scene_ops", "wb_scene_state", "wb_field_info", "wb_palette_state", "wb_ui_ops", "wb_dialog_info", "wb_menu_info", "wb_battle_info", "wb_coop_info", "wb_raster_graph", "wb_platform_players", "wb_continuation", "asyncify_get_state", "asyncify_start_unwind", "asyncify_stop_unwind", "asyncify_start_rewind", "asyncify_stop_rewind"} {
		if r.program.ExportedFunctions()[name] == nil {
			return fail(fmt.Errorf("native PAL ABI missing %s", name))
		}
	}
	memory := r.program.ExportedMemories()["memory"]
	if memory == nil || memory.Min() == 0 || memory.Min() > maxMemory/65536 {
		return fail(errors.New("invalid native PAL memory declaration"))
	}
	r.minimum = memory.Min() * 65536
	imports := map[string]map[string]api.FunctionDefinition{}
	for _, fn := range r.program.ImportedFunctions() {
		module, name, _ := fn.Import()
		if imports[module] == nil {
			imports[module] = map[string]api.FunctionDefinition{}
		}
		imports[module][name] = fn
	}
	if len(r.program.ImportedMemories()) != 0 {
		return fail(errors.New("native PAL cannot use shared host memory"))
	}
	for module, functions := range imports {
		builder := r.vm.NewHostModuleBuilder(module)
		for name, fn := range functions {
			call := api.GoModuleFunc(func(_ context.Context, _ api.Module, _ []uint64) {
				panic("unsupported native PAL host call: " + module + "." + name)
			})
			if module == "pal" && name == "yield" {
				call = api.GoModuleFunc(nativeYield)
			}
			if module == "pal" && name == "fail" {
				call = api.GoModuleFunc(nativeFail)
			}
			if module == "pal" && name == "inflate" {
				call = api.GoModuleFunc(nativeInflate)
			}
			builder.NewFunctionBuilder().WithGoModuleFunction(call, fn.ParamTypes(), fn.ResultTypes()).Export(name)
		}
		if _, err = builder.Instantiate(ctx); err != nil {
			return fail(err)
		}
	}
	if stamped {
		if err = r.verifyCompiledProvenance(ctx); err != nil {
			return fail(err)
		}
	}
	return r, nil
}

func packText(strings []string) packedText {
	p := packedText{data: make([]byte, len(strings)*4), offsets: make([]uint32, len(strings))}
	for i, text := range strings {
		p.offsets[i] = uint32(len(p.data))
		for _, char := range text {
			p.data = binary.LittleEndian.AppendUint32(p.data, uint32(char))
		}
		p.data = append(p.data, 0, 0, 0, 0)
	}
	return p
}

func nativeFail(_ context.Context, module api.Module, stack []uint64) {
	pointer := uint32(stack[0])
	message := make([]byte, 0, 128)
	for i := uint32(0); i < 4096; i++ {
		value, ok := module.Memory().ReadByte(pointer + i)
		if !ok || value == 0 {
			break
		}
		message = append(message, value)
	}
	panic("native PAL: " + string(message))
}

func nativeInflate(_ context.Context, module api.Module, stack []uint64) {
	source, size, target, capacity := uint32(stack[0]), uint32(stack[1]), uint32(stack[2]), uint32(stack[3])
	if size > 8*1024*1024 || capacity > 8*1024*1024 {
		panic("native metadata exceeds limit")
	}
	data, ok := module.Memory().Read(source, size)
	if !ok {
		panic("native metadata exceeds input memory")
	}
	input := bytes.NewReader(data)
	reader, err := zlib.NewReader(input)
	if err != nil {
		panic(err)
	}
	defer reader.Close()
	decoded, err := io.ReadAll(io.LimitReader(reader, int64(capacity)+1))
	if err != nil || len(decoded) > int(capacity) || input.Len() != 0 {
		panic("native metadata failed validation")
	}
	if !module.Memory().Write(target, decoded) {
		panic("native metadata exceeds output memory")
	}
	stack[0] = uint64(len(decoded))
}

func call(ctx context.Context, module api.Module, name string, args ...uint64) (uint64, error) {
	service := name == "wb_audio_info" || name == "wb_raster_seed_info" || name == "wb_input" || name == "wb_budget" || name == "wb_release" || name == "wb_view" || name == "wb_scene_ops" || name == "wb_scene_state" || name == "wb_field_info" || name == "wb_palette_state" || name == "wb_ui_ops" || name == "wb_dialog_info" || name == "wb_menu_info" || name == "wb_battle_info" || name == "wb_battle_assets" || name == "wb_coop_info" || name == "wb_raster_graph" || name == "wb_platform_players"
	if service {
		pointer, ok := module.ExportedGlobal("__stack_pointer").(api.MutableGlobal)
		top := module.ExportedGlobal("__stack_high")
		if !ok || top == nil {
			return 0, errors.New("native PAL stack boundary is unavailable")
		}
		previous := pointer.Get()
		address := top.Get()
		if service {
			base := module.ExportedGlobal("wb_service_stack")
			if base == nil {
				return 0, errors.New("native PAL service stack is unavailable")
			}
			address = base.Get() + 262144
		}
		if address > uint64(module.Memory().Size()) || address%16 != 0 {
			return 0, errors.New("native PAL stack exceeds memory")
		}
		pointer.Set(address)
		defer pointer.Set(previous)
	}
	fn := module.ExportedFunction(name)
	if fn == nil {
		return 0, fmt.Errorf("missing native export %s", name)
	}
	result, err := fn.Call(ctx, args...)
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0], nil
}

func nativeYield(ctx context.Context, module api.Module, stack []uint64) {
	e, ok := ctx.Value(executionKey{}).(*execution)
	if !ok {
		panic("native PAL yielded outside a rule step")
	}
	state, err := call(ctx, module, "asyncify_get_state")
	if err != nil {
		panic(err)
	}
	if state == 2 {
		if _, err = call(ctx, module, "asyncify_stop_rewind"); err != nil {
			panic(err)
		}
		stack[0] = e.response
		return
	}
	if state != 0 {
		panic("invalid native PAL unwind state")
	}
	e.event = &Event{int(uint32(stack[0])), int(uint32(stack[1]))}
	address, err := call(ctx, module, "wb_continuation")
	if err != nil {
		panic(err)
	}
	if _, err = call(ctx, module, "asyncify_start_unwind", address); err != nil {
		panic(err)
	}
	stack[0] = 0
}

func (r *Runtime) instance(ctx context.Context) (api.Module, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.vm.InstantiateModule(ctx, r.program, wazero.NewModuleConfig().WithName(fmt.Sprintf("pal-native-%d", r.sequence.Add(1))).WithStartFunctions("_initialize"))
}

func write(ctx context.Context, module api.Module, data []byte) (uint64, error) {
	address, err := call(ctx, module, "wb_allocate", uint64(len(data)))
	if err != nil {
		return 0, err
	}
	if address == 0 || !module.Memory().Write(uint32(address), data) {
		return 0, errors.New("native PAL resource allocation failed")
	}
	return address, nil
}

func writeText(ctx context.Context, module api.Module, table packedText) (uint64, error) {
	address, err := call(ctx, module, "wb_allocate", uint64(len(table.data)))
	if err != nil {
		return 0, err
	}
	if address == 0 {
		return 0, errors.New("native PAL text allocation failed")
	}
	data := bytes.Clone(table.data)
	for i, offset := range table.offsets {
		binary.LittleEndian.PutUint32(data[i*4:], uint32(address)+offset)
	}
	if !module.Memory().Write(uint32(address), data) {
		return 0, errors.New("native PAL text exceeds memory")
	}
	return address, nil
}

func (r *Runtime) StartOpening(ctx context.Context, seed uint32) (Snapshot, error) {
	return r.start(ctx, seed, "opening", 1)
}

func (r *Runtime) StartCampaign(ctx context.Context, seed uint32) (Snapshot, error) {
	return r.StartCampaignForPlayers(ctx, seed, 1)
}

func (r *Runtime) StartCampaignForPlayers(ctx context.Context, seed uint32, players int) (Snapshot, error) {
	if players < 1 || players > 3 {
		return Snapshot{}, errors.New("invalid PAL platform player count")
	}
	return r.start(ctx, seed, "campaign", players)
}

func (r *Runtime) start(ctx context.Context, seed uint32, entry string, players int) (Snapshot, error) {
	module, err := r.instance(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	defer module.Close(context.Background())
	for _, file := range r.files {
		name, err := write(ctx, module, append([]byte(file.name), 0))
		if err != nil {
			return Snapshot{}, err
		}
		data, err := write(ctx, module, file.data)
		if err != nil {
			return Snapshot{}, err
		}
		ok, err := call(ctx, module, "wb_bind_file", name, data, uint64(len(file.data)))
		if err != nil {
			return Snapshot{}, err
		}
		if ok != 1 {
			return Snapshot{}, errors.New("native PAL rejected rule resource")
		}
	}
	for i, table := range []packedText{r.wordTable, r.messageTable} {
		address, err := writeText(ctx, module, table)
		if err != nil {
			return Snapshot{}, err
		}
		if _, err = call(ctx, module, "wb_bind_text", uint64(i), address, uint64(len(table.offsets))); err != nil {
			return Snapshot{}, err
		}
	}
	ok, err := call(ctx, module, "wb_boot", uint64(seed))
	if err != nil {
		return Snapshot{}, err
	}
	if ok != 1 {
		return Snapshot{}, errors.New("native PAL initialization failed")
	}
	if _, err = call(ctx, module, "wb_platform_players", uint64(players)); err != nil {
		return Snapshot{}, err
	}
	return r.advance(ctx, module, false, entry, Input{Direction: 4})
}

func (r *Runtime) Step(ctx context.Context, snapshot Snapshot, response uint32) (Snapshot, error) {
	return r.StepWithInput(ctx, snapshot, Input{Direction: 4, Response: response})
}

func (r *Runtime) StepWithInput(ctx context.Context, snapshot Snapshot, input Input) (Snapshot, error) {
	if snapshot.Event.Kind == SceneComplete {
		return Snapshot{}, errors.New("opening script already completed")
	}
	module, err := r.restore(ctx, snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	defer module.Close(context.Background())
	return r.advance(ctx, module, true, snapshot.Entry, input)
}

func (r *Runtime) restore(ctx context.Context, snapshot Snapshot) (api.Module, error) {
	if err := r.validate(snapshot); err != nil {
		return nil, err
	}
	return r.restoreValidated(ctx, snapshot)
}

func (r *Runtime) restoreValidated(ctx context.Context, snapshot Snapshot) (api.Module, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	module, err := r.instance(ctx)
	if err != nil {
		return nil, err
	}
	if size := module.Memory().Size(); uint32(len(snapshot.Memory)) > size {
		if _, ok := module.Memory().Grow((uint32(len(snapshot.Memory)) - size) / 65536); !ok {
			module.Close(context.Background())
			return nil, errors.New("native PAL cannot restore memory")
		}
	}
	if !module.Memory().Write(0, snapshot.Memory) {
		module.Close(context.Background())
		return nil, errors.New("native PAL snapshot exceeds memory")
	}
	pointer, ok := module.ExportedGlobal("__stack_pointer").(api.MutableGlobal)
	low, high := module.ExportedGlobal("__stack_low"), module.ExportedGlobal("__stack_high")
	if !ok || low == nil || high == nil || uint64(snapshot.StackPointer) < low.Get() || uint64(snapshot.StackPointer) > high.Get() {
		module.Close(context.Background())
		return nil, errors.New("invalid native PAL saved stack pointer")
	}
	pointer.Set(uint64(snapshot.StackPointer))
	return module, nil
}

func (r *Runtime) ReleaseInputs(ctx context.Context, snapshot Snapshot) (Snapshot, error) {
	module, err := r.restore(ctx, snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	defer module.Close(context.Background())
	if _, err = call(ctx, module, "wb_release"); err != nil {
		return Snapshot{}, err
	}
	return r.capture(ctx, module, snapshot.Entry, snapshot.Event)
}

func (r *Runtime) AdvanceTimed(ctx context.Context, snapshot Snapshot, input Input, remainingMS, elapsedMS uint32) (TimedResult, error) {
	if input.Direction < 0 || input.Direction > 4 || input.Keys&^uint32(0x3ffff) != 0 {
		return TimedResult{}, errors.New("invalid native input")
	}
	return r.AdvanceScheduled(ctx, snapshot, remainingMS, elapsedMS, func(uint32) Input {
		next := input
		input.Keys, input.Response = 0, 0
		return next
	})
}

func (r *Runtime) AdvanceScheduled(ctx context.Context, snapshot Snapshot, remainingMS, elapsedMS uint32, inputAt func(uint32) Input) (TimedResult, error) {
	if err := r.validate(snapshot); err != nil {
		return TimedResult{}, err
	}
	return r.advanceScheduledValidated(ctx, snapshot, remainingMS, elapsedMS, inputAt, nil)
}

func (r *Runtime) advanceScheduledValidated(ctx context.Context, snapshot Snapshot, remainingMS, elapsedMS uint32, inputAt func(uint32) Input, memory []byte) (TimedResult, error) {
	result := TimedResult{Snapshot: snapshot, RemainingMS: remainingMS}
	if err := ctx.Err(); err != nil {
		return TimedResult{}, err
	}
	timed := func(e Event) bool { return e.Kind == Delay || e.Kind == Frame }
	if inputAt == nil {
		return TimedResult{}, errors.New("missing native input schedule")
	}
	if elapsedMS > 250 || (!timed(snapshot.Event) && remainingMS != 0) || (timed(snapshot.Event) && remainingMS > uint32(snapshot.Event.Value)) {
		return TimedResult{}, errors.New("invalid native timing budget")
	}
	if !timed(snapshot.Event) {
		return result, nil
	}
	if remainingMS > elapsedMS {
		result.RemainingMS -= elapsedMS
		return result, nil
	}
	module, err := r.restoreValidated(ctx, snapshot)
	if err != nil {
		return TimedResult{}, err
	}
	defer module.Close(context.Background())
	event := snapshot.Event
	boundary, err := nativeInputBoundary(ctx, module)
	if err != nil {
		return TimedResult{}, err
	}
	budget := elapsedMS
	audioSerial := uint32(0)
	if snapshot.View.Audio != nil {
		audioSerial = snapshot.View.Audio.Serial
	}
	for timed(event) && result.RemainingMS <= elapsedMS {
		if result.Steps >= 4096 {
			return TimedResult{}, errors.New("native timed continuation exceeds budget")
		}
		at := budget - elapsedMS + result.RemainingMS
		elapsedMS -= result.RemainingMS
		input := Input{Direction: 4}
		if !result.InputReset {
			input = inputAt(at)
		}
		event, err = r.run(ctx, module, true, snapshot.Entry, input)
		if err != nil {
			return TimedResult{}, err
		}
		if snapshot.View.Audio != nil {
			journal, e := observeAudio(ctx, module)
			if e != nil {
				return TimedResult{}, e
			}
			commands, e := audioSince(journal, audioSerial)
			if e != nil {
				return TimedResult{}, e
			}
			for _, command := range commands {
				result.Audio = append(result.Audio, TimedAudioCommand{OffsetMS: at, Command: command})
			}
			audioSerial = journal.Serial
		}
		nextBoundary, e := nativeInputBoundary(ctx, module)
		if e != nil {
			return TimedResult{}, e
		}
		if boundary != nextBoundary {
			result.InputReset = true
			if _, e = call(ctx, module, "wb_release"); e != nil {
				return TimedResult{}, e
			}
		}
		boundary = nextBoundary
		result.Steps++
		result.RemainingMS = 0
		if timed(event) {
			result.RemainingMS = uint32(event.Value)
		}
	}
	if timed(event) {
		result.RemainingMS -= elapsedMS
	}
	result.Snapshot, err = r.captureWithMemory(ctx, module, snapshot.Entry, event, memory)
	return result, err
}

func nativeInputBoundary(ctx context.Context, module api.Module) ([14]int, error) {
	var key [14]int
	battle, err := nativeBattleBoundary(ctx, module)
	if err != nil {
		return key, err
	}
	if battle[0] != 0 {
		copy(key[:6], battle[:])
	}
	address, err := call(ctx, module, "wb_view")
	if err != nil {
		return key, err
	}
	data, ok := module.Memory().Read(uint32(address), 20*4)
	if !ok {
		return key, errors.New("native input boundary exceeds memory")
	}
	n := func(i int) int { return int(int32(binary.LittleEndian.Uint32(data[i*4:]))) }
	key[6], key[7], key[8], key[10] = n(1), n(17), n(18), n(4)
	for i := 0; i < key[10] && i < 3; i++ {
		key[11+i] = n(5 + i)
	}
	address, err = call(ctx, module, "wb_menu_info")
	if err != nil {
		return key, err
	}
	data, ok = module.Memory().Read(uint32(address), 8)
	if !ok {
		return key, errors.New("native input menu boundary exceeds memory")
	}
	key[9] = int(int32(binary.LittleEndian.Uint32(data[4:])))
	return key, nil
}

func (r *Runtime) advance(ctx context.Context, module api.Module, rewind bool, entry string, input Input) (Snapshot, error) {
	event, err := r.run(ctx, module, rewind, entry, input)
	if err != nil {
		return Snapshot{}, err
	}
	return r.capture(ctx, module, entry, event)
}

func nativeBattleBoundary(ctx context.Context, module api.Module) ([6]int, error) {
	var key [6]int
	address, err := call(ctx, module, "wb_battle_info")
	if err != nil {
		return key, err
	}
	data, ok := module.Memory().Read(uint32(address), 24)
	if !ok {
		return key, errors.New("native battle boundary exceeds memory")
	}
	for i := range key {
		key[i] = int(int32(binary.LittleEndian.Uint32(data[i*4:])))
	}
	return key, nil
}

func observeBattle(ctx context.Context, module api.Module) (BattleState, error) {
	address, err := call(ctx, module, "wb_battle_info")
	if err != nil {
		return BattleState{}, err
	}
	data, ok := module.Memory().Read(uint32(address), 87*4)
	if !ok {
		return BattleState{}, errors.New("native battle observation exceeds memory")
	}
	n := func(i int) int { return int(int32(binary.LittleEndian.Uint32(data[i*4:]))) }
	if n(14) < 1 || n(14) > 3 || n(15) < 0 || n(15) > 5 || n(2) < 0 || n(2) > 1 || n(3) < 0 || n(3) > 5 || n(4) < 0 || n(4) > 5 || n(5) < 0 || n(5) > 2 || (n(0) != 0 && n(3) != 0 && n(5) >= n(14)) {
		return BattleState{}, errors.New("invalid native battle structure")
	}
	b := BattleState{Active: n(0) != 0, Result: n(1), Phase: n(2), State: n(3), Menu: n(4), Current: n(5), Selected: n(6), Action: n(7), Object: n(8), Target: n(9), Auto: n(10) != 0, Repeat: n(11) != 0, Force: n(12) != 0, Flee: n(13) != 0, Players: make([]BattlePlayer, n(14)), Enemies: make([]BattleEnemy, n(15))}
	for i := range b.Players {
		p := 16 + i*12
		b.Players[i] = BattlePlayer{n(p), n(p + 1), n(p + 2), n(p + 3), n(p + 4), n(p + 5), n(p + 6), n(p + 7), n(p + 8), n(p + 9), n(p + 10), n(p+11) != 0}
	}
	for i := range b.Enemies {
		p := 52 + i*7
		b.Enemies[i] = BattleEnemy{n(p), n(p + 1), n(p + 2), n(p + 3), n(p + 4), n(p + 5), n(p + 6)}
	}
	return b, nil
}

func observeRaster(ctx context.Context, module api.Module) (*RasterGraph, error) {
	address, err := call(ctx, module, "wb_raster_graph")
	if err != nil {
		return nil, err
	}
	header, ok := module.Memory().Read(uint32(address), 12)
	if !ok {
		return nil, errors.New("invalid native raster header")
	}
	get := func(i int) int { return int(int32(binary.LittleEndian.Uint32(header[i*4:]))) }
	if get(0) != 1 || get(1) < 0 || get(1) > 16384 || get(2) < 0 || get(2) > get(1) {
		return nil, errors.New("invalid native raster dimensions")
	}
	if get(1) == 0 {
		if get(2) != 0 {
			return nil, errors.New("invalid native empty raster")
		}
		return nil, nil
	}
	graph := &RasterGraph{Version: 1, Root: get(2), Nodes: make([][16]int, get(1))}
	data, ok := module.Memory().Read(uint32(address)+12, uint32(get(1)*64))
	if !ok {
		return nil, errors.New("native raster exceeds memory")
	}
	for i := range graph.Nodes {
		for j := range graph.Nodes[i] {
			graph.Nodes[i][j] = int(int32(binary.LittleEndian.Uint32(data[i*64+j*4:])))
		}
	}
	for _, n := range graph.Nodes {
		if n[0] == 9 {
			slot := n[5]
			if slot != 0 && slot != 1 {
				return nil, errors.New("invalid native raster scene slot")
			}
			if slot == 0 && graph.Base == nil {
				graph.Base, err = observeRasterBase(ctx, module, 0)
			}
			if slot == 1 && graph.Backup == nil {
				graph.Backup, err = observeRasterBase(ctx, module, 1)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	return graph, validateRaster(graph)
}

func validateRaster(g *RasterGraph) error {
	if g == nil {
		return nil
	}
	if g.Version != 1 || len(g.Nodes) < 1 || len(g.Nodes) > 16384 || g.Root < 1 || g.Root > len(g.Nodes) {
		return errors.New("invalid saved PAL raster")
	}
	for i, n := range g.Nodes {
		if n[0] < 0 || n[0] > 11 || n[1] < 0 || n[1] > i || n[2] < 0 || n[2] > i || n[3] < 1 || n[3] > 4096 || n[4] < 1 || n[4] > 4096 {
			return errors.New("invalid saved PAL raster dependency")
		}
		if n[0] == 1 && (n[5] < 1 || n[5] > 9 || n[6] < 0 || n[6] > 65535 || n[7] < 0 || n[7] > 4096 || n[10] < 0 || n[10] > 3 || n[11] < 0 || n[11] > 255 || n[12] < -255 || n[12] > 255) {
			return errors.New("invalid saved PAL raster image")
		}
		if n[0] == 9 {
			base := g.Base
			if n[5] == 1 {
				base = g.Backup
			}
			if n[5] < 0 || n[5] > 1 || n[3] != 320 || n[4] != 200 || validateRasterBase(base) != nil {
				return errors.New("invalid saved PAL raster scene")
			}
		}
		if n[0] == 10 && (n[3] != 320 || n[4] != 200 || n[1] == 0 || n[5] < 0 || n[5] > 4095 || n[6] < 0 || n[7] < n[6] || n[7] > 4095) {
			return errors.New("invalid saved PAL RNG range")
		}
		if n[0] == 11 && (n[1] == 0 || n[2] != 0 || n[5] < 0 || n[5] > 255 || n[6] < 0 || n[6] > 255) {
			return errors.New("invalid saved PAL palette index replacement")
		}
	}
	last := g.Nodes[g.Root-1]
	if last[3] != 320 || last[4] != 200 {
		return errors.New("invalid saved PAL raster screen")
	}
	return nil
}

func (r *Runtime) run(ctx context.Context, module api.Module, rewind bool, entry string, input Input) (Event, error) {
	if input.Direction < 0 || input.Direction > 4 || input.Keys&^uint32(0x3ffff) != 0 {
		return Event{}, errors.New("invalid native input")
	}
	if _, err := call(ctx, module, "wb_input", uint64(input.Direction), uint64(input.Keys)); err != nil {
		return Event{}, err
	}
	if _, err := call(ctx, module, "wb_budget", 1000000); err != nil {
		return Event{}, err
	}
	e := &execution{response: uint64(input.Response)}
	ctx = context.WithValue(ctx, executionKey{}, e)
	if rewind {
		address, err := call(ctx, module, "wb_continuation")
		if err != nil {
			return Event{}, err
		}
		if _, err = call(ctx, module, "asyncify_start_rewind", address); err != nil {
			return Event{}, err
		}
	}
	function := "wb_run_opening"
	if entry == "campaign" {
		function = "wb_run_campaign"
	}
	if _, err := call(ctx, module, function); err != nil {
		return Event{}, err
	}
	state, err := call(ctx, module, "asyncify_get_state")
	if err != nil {
		return Event{}, err
	}
	if state != 1 || e.event == nil {
		return Event{}, errors.New("native PAL stopped without a continuation")
	}
	if _, err = call(ctx, module, "asyncify_stop_unwind"); err != nil {
		return Event{}, err
	}
	return *e.event, nil
}

func (r *Runtime) capture(ctx context.Context, module api.Module, entry string, event Event) (Snapshot, error) {
	return r.captureWithMemory(ctx, module, entry, event, nil)
}

func (r *Runtime) captureWithMemory(ctx context.Context, module api.Module, entry string, event Event, owned []byte) (Snapshot, error) {
	address, err := call(ctx, module, "wb_view")
	if err != nil {
		return Snapshot{}, err
	}
	data, ok := module.Memory().Read(uint32(address), 128)
	if !ok {
		return Snapshot{}, errors.New("native PAL view exceeds memory")
	}
	get := func(i int) uint32 { return binary.LittleEndian.Uint32(data[i*4:]) }
	if get(0) != 1 || get(4) < 1 || get(4) > 3 {
		return Snapshot{}, errors.New("native PAL returned an invalid view")
	}
	v := StateView{Scene: int(get(1)), X: int(get(2)), Y: int(get(3)), Party: make([]int, get(4)), Sprite: int(get(8)), HP: int(get(9)), MP: int(get(10)), Cash: get(11), Seed: get(12), Clock: get(13), PC: get(14), Messages: get(15)}
	v.Audio, err = observeAudio(ctx, module)
	if err != nil {
		return Snapshot{}, err
	}
	v.Ending, err = observeEnding(ctx, module, event)
	if err != nil {
		return Snapshot{}, err
	}
	v.GameOver, err = observeGameOver(ctx, module, event)
	if err != nil {
		return Snapshot{}, err
	}
	sceneState, err := call(ctx, module, "wb_scene_state")
	if err != nil {
		return Snapshot{}, err
	}
	v.SceneReady, v.ClearColor = sceneState&1 != 0, int(sceneState>>8)&255
	paletteAddress, err := call(ctx, module, "wb_palette_state")
	if err != nil {
		return Snapshot{}, err
	}
	paletteData, ok := module.Memory().Read(uint32(paletteAddress), 48)
	if !ok {
		return Snapshot{}, errors.New("native palette state exceeds memory")
	}
	pg := func(i int) int { return int(int32(binary.LittleEndian.Uint32(paletteData[i*4:]))) }
	if pg(2) < 0 || pg(2) > 8 {
		return Snapshot{}, errors.New("native palette fade exceeds limit")
	}
	v.PaletteEffect = &PaletteEffect{Palette: pg(0), Night: pg(1) != 0, Factors: make([]int, pg(2))}
	if module.ExportedFunction("wb_palette_red_step") != nil {
		step, err := call(ctx, module, "wb_palette_red_step")
		if err != nil {
			return Snapshot{}, err
		}
		v.PaletteEffect.RedStep = int(step)
	}
	for i := range v.PaletteEffect.Factors {
		v.PaletteEffect.Factors[i] = pg(3 + i)
	}
	v.Mode, v.Entering, v.MovingPlayer = int(get(17)), get(18) != 0, int(get(19))
	v.HeldDirection, v.PendingKeys, v.Keys = int(get(20)), get(21), get(22)
	v.CameraX, v.CameraY, v.Map, v.Palette, v.Night = int(int32(get(23))), int(int32(get(24))), int(get(25)), int(get(26)), get(27) != 0
	v.Face, v.DialogPosition = int(get(30)), int(get(31))
	v.Battle, err = observeBattle(ctx, module)
	if err != nil {
		return Snapshot{}, err
	}
	if v.Battle.Active {
		v.BattleAssets, err = observeBattleAssets(ctx, module)
		if err != nil {
			return Snapshot{}, err
		}
	}
	v.Raster, err = observeRaster(ctx, module)
	if err != nil {
		return Snapshot{}, err
	}
	coopAddress, err := call(ctx, module, "wb_coop_info")
	if err != nil {
		return Snapshot{}, err
	}
	coop, ok := module.Memory().Read(uint32(coopAddress), 32)
	if !ok {
		return Snapshot{}, errors.New("native cooperative action exceeds memory")
	}
	cg := func(i int) int { return int(int32(binary.LittleEndian.Uint32(coop[i*4:]))) }
	if cg(0) != 0 {
		if cg(0) != 1 || cg(2) < 0 || cg(2) >= len(r.words) {
			return Snapshot{}, errors.New("invalid native cooperative magic")
		}
		v.Cooperation = CooperativeMagic{true, cg(1), cg(2), cg(3), cg(4), cg(5), cg(6), r.words[cg(2)]}
	}
	menuAddress, err := call(ctx, module, "wb_menu_info")
	if err != nil {
		return Snapshot{}, err
	}
	menuHeader, ok := module.Memory().Read(uint32(menuAddress), 24)
	if !ok {
		return Snapshot{}, errors.New("invalid native menu observation")
	}
	mg := func(i int) int { return int(int32(binary.LittleEndian.Uint32(menuHeader[i*4:]))) }
	if mg(0) < 0 || mg(0) > 16 || mg(1) < 0 || mg(1) > 13 || mg(2) < 0 || mg(2) > 128 || mg(4) < -1 || mg(4) >= 6 {
		return Snapshot{}, errors.New("invalid native menu structure")
	}
	v.Menu = MenuView{Kind: mg(1), Depth: mg(0), Selected: mg(3), Role: mg(4), Items: make([]MenuItem, mg(2))}
	menuItems, ok := module.Memory().Read(uint32(menuAddress)+24, uint32(mg(2)*20))
	if !ok {
		return Snapshot{}, errors.New("native menu items exceed memory")
	}
	for i := range v.Menu.Items {
		p := menuItems[i*20:]
		n := func(j int) int { return int(int32(binary.LittleEndian.Uint32(p[j*4:]))) }
		word := n(1)
		if word < 0 || word >= len(r.words) {
			return Snapshot{}, errors.New("invalid native menu label")
		}
		v.Menu.Items[i] = MenuItem{n(0), word, n(2) != 0, n(3), n(4), r.words[word]}
	}
	uiAddress, err := call(ctx, module, "wb_ui_ops")
	if err != nil {
		return Snapshot{}, err
	}
	uiHeader, ok := module.Memory().Read(uint32(uiAddress), 4)
	if !ok {
		return Snapshot{}, errors.New("invalid native UI observation")
	}
	uiCount := binary.LittleEndian.Uint32(uiHeader)
	if uiCount > 8192 {
		return Snapshot{}, errors.New("native UI observation exceeds limit")
	}
	uiData, ok := module.Memory().Read(uint32(uiAddress)+4, uiCount*32)
	if !ok {
		return Snapshot{}, errors.New("native UI operations exceed memory")
	}
	v.Overlay = make([]UIOp, uiCount)
	for i := range v.Overlay {
		p := uiData[i*32:]
		n := func(j int) int { return int(int32(binary.LittleEndian.Uint32(p[j*4:]))) }
		v.Overlay[i] = UIOp{n(0), n(1), n(2), n(3), n(4), n(5), n(6), n(7)}
	}
	dialogAddress, err := call(ctx, module, "wb_dialog_info")
	if err != nil {
		return Snapshot{}, err
	}
	dialog, ok := module.Memory().Read(uint32(dialogAddress), 38*4)
	if !ok {
		return Snapshot{}, errors.New("native dialogue observation exceeds memory")
	}
	d := func(i int) int { return int(int32(binary.LittleEndian.Uint32(dialog[i*4:]))) }
	if d(4) < 0 || d(4) > 32 {
		return Snapshot{}, errors.New("native dialogue page exceeds limit")
	}
	v.DialogWaiting, v.DialogLine = d(0) != 0, d(1)
	v.DialogPage = make([]int, d(4))
	for i := range v.DialogPage {
		v.DialogPage[i] = d(6 + i)
	}
	operations, err := call(ctx, module, "wb_scene_ops")
	if err != nil {
		return Snapshot{}, err
	}
	header, ok := module.Memory().Read(uint32(operations), 4)
	if !ok {
		return Snapshot{}, errors.New("invalid native scene observation")
	}
	count := binary.LittleEndian.Uint32(header)
	if count > 8192 {
		return Snapshot{}, errors.New("native scene observation exceeds limit")
	}
	ops, ok := module.Memory().Read(uint32(operations)+4, count*32)
	if !ok {
		return Snapshot{}, errors.New("native scene operations exceed memory")
	}
	v.Draws = make([]DrawOp, count)
	for i := range v.Draws {
		p := ops[i*32:]
		n := func(j int) int { return int(int32(binary.LittleEndian.Uint32(p[j*4:]))) }
		v.Draws[i] = DrawOp{n(0), n(1), n(2), n(3), n(4), n(5), n(6), n(7)}
	}
	sort.SliceStable(v.Draws, func(i, j int) bool { return v.Draws[i].SortY < v.Draws[j].SortY })
	for i := range v.Party {
		v.Party[i] = int(get(5 + i))
		if v.Party[i] >= len(r.roleNames) {
			return Snapshot{}, errors.New("native PAL returned an invalid party")
		}
	}
	v.Field, err = observeField(ctx, module)
	if err != nil {
		return Snapshot{}, err
	}
	memory, ok := module.Memory().Read(0, module.Memory().Size())
	if !ok || len(memory) > maxMemory {
		return Snapshot{}, errors.New("native PAL snapshot exceeds memory budget")
	}
	stack := uint32(module.ExportedGlobal("__stack_pointer").Get())
	result := Snapshot{Entry: entry, Version: 2, BuildID: r.buildID, ResourceID: r.resourceID, StackPointer: stack, Memory: memory, Event: event, View: v}
	if err := r.validateMetadata(result); err != nil {
		return Snapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if cap(owned) >= len(memory) {
		owned = owned[:len(memory):len(memory)]
		copy(owned, memory)
	} else {
		owned = bytes.Clone(memory)
	}
	result.Memory = owned
	result.MemoryID = snapshotIdentity(owned, stack)
	return result, nil
}

func (r *Runtime) validate(s Snapshot) error {
	if err := r.validateMetadata(s); err != nil {
		return err
	}
	if snapshotIdentity(s.Memory, s.StackPointer) != s.MemoryID {
		return errors.New("corrupt native PAL snapshot memory")
	}
	return nil
}

func (r *Runtime) validateMetadata(s Snapshot) error {
	if err := validateSceneDraws(s.View.Draws); err != nil {
		return err
	}
	if err := validateField(s.View); err != nil {
		return err
	}
	if p := s.View.PaletteEffect; p != nil {
		if p.Palette < 0 || p.Palette > 65535 || len(p.Factors) > 8 || p.RedStep < 0 || p.RedStep > 32 {
			return errors.New("invalid saved palette effect")
		}
		for _, f := range p.Factors {
			if f < 0 || f > 64 {
				return errors.New("invalid saved palette intensity")
			}
		}
	}
	if err := validateRaster(s.View.Raster); err != nil {
		return err
	}
	if err := validateBattleAssets(s.View.BattleAssets, s.View.Battle.Active && r.program.ExportedFunctions()["wb_battle_assets"] != nil); err != nil {
		return err
	}
	if s.Version != 2 || s.BuildID != r.buildID || s.ResourceID != r.resourceID {
		return errors.New("incompatible native PAL snapshot identity")
	}
	if s.Entry != "opening" && s.Entry != "campaign" {
		return errors.New("invalid native entry")
	}
	if len(s.Memory) < int(r.minimum) || len(s.Memory) > maxMemory || len(s.Memory)%65536 != 0 || s.StackPointer == 0 || s.StackPointer%16 != 0 {
		return errors.New("corrupt native PAL snapshot memory")
	}
	if s.Event.Kind < Delay || s.Event.Kind > GameOver || s.Event.Value < 0 {
		return errors.New("invalid native PAL continuation event")
	}
	if err := validateEnding(s.View.Ending, s.Event); err != nil {
		return err
	}
	if err := validateGameOver(s.View.GameOver, s.Event); err != nil {
		return err
	}
	if (s.View.Audio != nil) != (r.program.ExportedFunctions()["wb_audio_info"] != nil) {
		return errors.New("native audio capability does not match this snapshot")
	}
	if err := validateAudio(s.View.Audio); err != nil {
		return err
	}
	if s.View.Audio != nil {
		for _, command := range s.View.Audio.Commands {
			if command.Clock > s.View.Clock {
				return errors.New("native audio command is ahead of its rule clock")
			}
		}
	}
	if s.View.GameOver != nil && (s.View.Ending != nil || r.program.ExportedFunctions()["wb_game_over_info"] == nil) {
		return errors.New("native failure is not supported by this core")
	}
	if s.View.Ending != nil && r.program.ExportedFunctions()["wb_ending_info"] == nil {
		return errors.New("native PAL core has no ending timeline")
	}
	if s.Event.Kind == Dialogue && s.Event.Value >= len(r.messages) {
		return errors.New("invalid native PAL message")
	}
	if len(s.View.Overlay) > 8192 || len(s.View.DialogPage) > 32 || (s.Event.Kind == Dialogue && !s.View.DialogWaiting) {
		return errors.New("invalid native PAL saved presentation")
	}
	for _, id := range s.View.DialogPage {
		if id < 0 || id >= len(r.messages) {
			return errors.New("invalid native PAL saved dialogue page")
		}
	}
	for _, op := range s.View.Overlay {
		if op.Kind < 0 || op.Kind > 1 || op.Source < 0 || op.Source > 5 || op.ID < 0 || op.ID > 65535 || op.Frame < 0 || op.Frame > 4096 || op.Color < 0 || op.Color > 255 || op.Small < 0 || op.Small > 1 {
			return errors.New("invalid native PAL saved UI operation")
		}
	}
	if s.View.Scene < 1 || s.View.Scene > 295 || len(s.View.Party) < 1 || len(s.View.Party) > 3 {
		return errors.New("invalid native PAL saved view")
	}
	b := s.View.Battle
	c := s.View.Cooperation
	if c.Pending != (s.Event.Kind == CooperativeAction) {
		return errors.New("invalid native PAL cooperative continuation")
	}
	if c.Pending && (!b.Active || b.Phase != 0 || c.Player != b.Current || c.Magic != s.Event.Value || c.Magic < 0 || c.Magic >= len(r.words) || c.Name != r.words[c.Magic] || c.Target < -1 || c.Target > 4 || c.AffectedMask != (1<<len(s.View.Party))-1 || c.ContributorMask < 0 || c.ContributorMask&^c.AffectedMask != 0 || c.HPCost < 0 || c.HPCost > 65535) {
		return errors.New("invalid native PAL saved cooperative magic")
	}
	if len(b.Players) != len(s.View.Party) || len(b.Enemies) > 5 || b.Phase < 0 || b.Phase > 1 || b.State < 0 || b.State > 5 || b.Menu < 0 || b.Menu > 5 || b.Selected < 0 || b.Selected > 3 || b.Action < 0 || b.Action > 8 || b.Current < 0 || b.Current > 2 || (b.Active && b.State != 0 && b.Current >= len(b.Players)) {
		return errors.New("invalid native PAL saved battle")
	}
	for i, p := range b.Players {
		if p.Role != s.View.Party[i] || p.HP < 0 || p.HP > 65535 || p.MaxHP < 0 || p.MaxHP > 65535 || p.MP < 0 || p.MP > 65535 || p.MaxMP < 0 || p.MaxMP > 65535 || p.Level < 0 || p.Level > 65535 || p.State < 0 || p.State > 2 || p.Action < 0 || p.Action > 8 || p.Object < 0 || p.Object > 65535 || p.Target < -1 || p.Target > 5 || p.Frame < 0 || p.Frame > 4096 {
			return errors.New("invalid native PAL saved battle player")
		}
	}
	for _, enemy := range b.Enemies {
		if enemy.Object < 0 || enemy.Object > 65535 || enemy.HP < 0 || enemy.HP > 65535 || enemy.State < 0 || enemy.State > 2 || enemy.Frame < 0 || enemy.Frame > 4096 {
			return errors.New("invalid native PAL saved battle enemy")
		}
	}
	for _, role := range s.View.Party {
		if role < 0 || role >= len(r.roleNames) {
			return errors.New("invalid native PAL saved character")
		}
	}
	return nil
}

func (r *Runtime) View(s Snapshot) (Projection, error) {
	if err := r.validate(s); err != nil {
		return Projection{}, err
	}
	return r.project(s), nil
}

func (r *Runtime) project(s Snapshot) Projection {
	v := Projection{StateView: s.View, Event: s.Event, PartyNames: make([]string, len(s.View.Party))}
	v.Field = cloneField(s.View.Field)
	if s.View.Audio != nil {
		copy := *s.View.Audio
		copy.Commands = slices.Clone(copy.Commands)
		v.Audio = &copy
	}
	if s.View.Ending != nil {
		copy := *s.View.Ending
		v.Ending = &copy
	}
	if s.View.GameOver != nil {
		copy := *s.View.GameOver
		v.GameOver = &copy
	}
	v.Party = slices.Clone(s.View.Party)
	v.Draws = slices.Clone(s.View.Draws)
	v.Overlay = slices.Clone(s.View.Overlay)
	v.DialogPage = slices.Clone(s.View.DialogPage)
	v.Menu.Items = slices.Clone(s.View.Menu.Items)
	v.Battle.Players = slices.Clone(s.View.Battle.Players)
	v.Battle.Enemies = slices.Clone(s.View.Battle.Enemies)
	v.BattleAssets = slices.Clone(s.View.BattleAssets)
	if s.View.PaletteEffect != nil {
		copy := *s.View.PaletteEffect
		copy.Factors = slices.Clone(copy.Factors)
		v.PaletteEffect = &copy
	}
	if s.View.Raster != nil {
		copy := *s.View.Raster
		copy.Nodes = slices.Clone(copy.Nodes)
		copy.Base = cloneRasterBase(copy.Base)
		copy.Backup = cloneRasterBase(copy.Backup)
		v.Raster = &copy
	}
	for i, role := range v.Party {
		v.PartyNames[i] = r.roleNames[role]
	}
	if s.Event.Kind == Dialogue {
		lines := make([]string, len(s.View.DialogPage))
		for i, id := range s.View.DialogPage {
			lines[i] = r.messages[id]
		}
		v.Text = strings.Join(lines, "\n")
	}
	return v
}

func (r *Runtime) Close(ctx context.Context) error { return r.vm.Close(ctx) }
