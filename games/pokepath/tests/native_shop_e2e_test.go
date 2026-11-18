package pokepath_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func TestNativePlatformShopRejectsTheOldEggQuoteAndAttributesReceipt(t *testing.T) {
	module, ctx, match := nativeStart(t, 4)
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(match.Data, &envelope); err != nil {
		t.Fatal(err)
	}
	world, err := engine.Unmarshal(envelope["snapshot"])
	if err != nil {
		t.Fatal(err)
	}
	world.Gold = 1500000
	world.Progression.EggPrice = 50000
	envelope["snapshot"], err = engine.Marshal(world)
	if err != nil {
		t.Fatal(err)
	}
	match.Data, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	count, beforeID := len(world.Progression.EggPool), world.NextID
	epoch := world.Epoch
	apply := func(quoted int, buyer int, id string) (json.RawMessage, error) {
		quotedCtx := ctx
		quotedCtx.Actor = buyer
		body := json.RawMessage(fmt.Sprintf(`{"type":"egg","epoch":%d,"quote":%d,"clientInputID":%q}`, epoch, quoted, id))
		replied, applyErr := module.Apply(quotedCtx, match, body)
		if applyErr != nil {
			return nil, applyErr
		}
		return replied.Data, nil
	}
	if _, err := apply(0, 1, "missing"); err == nil {
		t.Fatal("an unquoted egg purchase succeeded")
	}
	bought, err := apply(count, 1, "seat-one")
	if err != nil {
		t.Fatal(err)
	}
	match.Data = bought
	view := nativeObserve(t, module, match, 2)
	state, err := engine.Unmarshal(view.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	last := state.Events[len(state.Events)-1]
	if view.Gold != 1450000 || len(state.Progression.EggPool) != count-1 || last.Type != "shopEgg" || last.Source != 2 || last.Target != beforeID || last.Amount != 50000 || last.Key == "" || !slices.ContainsFunc(state.Team, func(p engine.Pokemon) bool { return p.ID == last.Target && p.Species == last.Key }) {
		t.Fatalf("wrong authoritative cap-price state or seat-specific egg receipt: %+v", last)
	}
	if _, err := apply(count, 3, "late"); err == nil {
		t.Fatal("stale egg quote succeeded when price was already capped")
	}
	if replay, err := apply(count, 1, "seat-one"); err != nil || !bytes.Equal(replay, bought) {
		t.Fatal("retrying the same confirmed egg changed the shared state", err)
	}
	if _, err := apply(count-1, 3, "fresh"); err != nil {
		t.Fatal("freshly rendered quote did not allow the next purchase", err)
	}
}

func TestNativePlatformShopBindsPurchaseToTheVisibleStock(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(match.Data, &envelope); err != nil {
		t.Fatal(err)
	}
	world, err := engine.Unmarshal(envelope["snapshot"])
	if err != nil {
		t.Fatal(err)
	}
	world.Gold = 10000000
	item := world.Progression.ShopStock[0]
	envelope["snapshot"], err = engine.Marshal(world)
	if err != nil {
		t.Fatal(err)
	}
	match.Data, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	field := fmt.Sprintf(`"type":"buyItem","stock":0,"itemID":%q`, item)
	withoutIdentity := json.RawMessage(fmt.Sprintf(`{"type":"buyItem","stock":0,"epoch":%d}`, world.Epoch))
	if _, err := module.Apply(ctx, match, withoutIdentity); err == nil {
		t.Fatal("an unidentified shelf purchase succeeded")
	}
	match = nativeAction(t, module, ctx, match, field)
	after := nativeObserve(t, module, match, 1)
	state, err := engine.Unmarshal(after.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	last := state.Events[len(state.Events)-1]
	price := engine.CatalogData().Items[item].Price
	if last.Type != "shopItem" || last.Key != item || last.Source != 1 || last.Target != 0 || last.Amount != float64(price) || state.Gold != world.Gold-price || state.Progression.ShopStock[0] == item {
		t.Fatalf("ordinary purchase did not restock, debit and retain its own receipt: %+v", last)
	}
	ctx.Actor = 1
	late := json.RawMessage(fmt.Sprintf(`{"epoch":%d,%s}`, after.Epoch, field))
	before := append([]byte(nil), match.Data...)
	if _, err := module.Apply(ctx, match, late); err == nil || !bytes.Equal(before, match.Data) {
		t.Fatal("old purchase was applied to newly stocked goods")
	}
}
