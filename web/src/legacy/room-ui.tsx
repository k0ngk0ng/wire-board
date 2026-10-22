import type { GameSetupFormProps } from "../ui/create-room";
import { ArrowRight, Bot, Check, Clock, Crown, Flag, Plus } from "lucide-react";
import React, { useContext, useEffect, useState } from "react";
import {
  catanPhases,
  catanSeafarerPhases,
} from "../../../games/catan/web/catan";
import { CatanAttackSeat } from "../../../games/catan/web/catan-attack";
import { catanBaseLayoutName } from "../../../games/catan/web/catan-base-layout";
import { CatanBasePicker } from "../../../games/catan/web/catan-base-setup";
import { caravanResponder } from "../../../games/catan/web/catan-caravans-state";
import { CatanCitiesKnightsSetup } from "../../../games/catan/web/catan-cities-knights-setup";
import { CatanCitySeat } from "../../../games/catan/web/catan-city";
import { ClothPicture } from "../../../games/catan/web/catan-cloth";
import {
  CATAN_EVENT_CATALOGUE,
  catanEventsSupported,
} from "../../../games/catan/web/catan-event-options";
import { CatanEventPicker } from "../../../games/catan/web/catan-event-picker";
import { explorerIsHarbor } from "../../../games/catan/web/catan-explorer-state";
import { fishResponder } from "../../../games/catan/web/catan-fishing-state";
import {
  CatanFriendlyRobberChoice,
  CatanFriendlyRobberPicker,
  CatanFriendlyRobberSeat,
  supportsPublicCatanFriendly,
} from "../../../games/catan/web/catan-friendly-robber";
import {
  CatanHarborsChoice,
  CatanHarborsPicker,
  CatanHarborsSeat,
  supportsPublicCatanHarbors,
} from "../../../games/catan/web/catan-harbors";
import { CatanOptionPicker } from "../../../games/catan/web/catan-helpers";
import {
  catanColorIndex,
  catanSeatColor,
} from "../../../games/catan/web/catan-player-colors";
import { CatanRiverSeat } from "../../../games/catan/web/catan-rivers";
import { isPublicCatanRiversSea } from "../../../games/catan/web/catan-rivers-sea-options";
import { CatanRiversWorldPreview } from "../../../games/catan/web/catan-rivers-world-preview";
import {
  catanRuleContext,
  catanVictoryTarget,
} from "../../../games/catan/web/catan-rule-context";
import { CatanRules } from "../../../games/catan/web/catan-rules";
import {
  CatanCombinationKnightsPicker,
  CatanFishingSeaPicker,
  CatanScenarioPicker,
  isPublicCatanExplorer,
  isPublicCatanFlexible,
  isPublicCatanSea,
  supportsPublicCatanFishingSea,
  supportsPublicCatanFishingSeaExtended,
  supportsPublicCatanKnightsCombination,
  supportsPublicExplorerKnights,
} from "../../../games/catan/web/catan-scenario-setup";
import {
  CatanSeafarersPicker,
  catanScenarioName,
  catanScenarioVictory,
} from "../../../games/catan/web/catan-scenarios";
import { supportsCatanTradersHelpers } from "../../../games/catan/web/catan-traders-helpers";
import { CatanTransportSeat } from "../../../games/catan/web/catan-transport";
import {
  CatanTransportSeaLayout,
  CatanTransportSeaWaitingLayout,
  isCatanTransportSea,
} from "../../../games/catan/web/catan-transport-sea-options";
import {
  catanTwoHelpersNote,
  isPublicCatanAttackSea,
  isPublicCatanCaravanSea,
  supportsCaravanSeaHelpers,
  supportsTwoCatanHelpers,
} from "../../../games/catan/web/catan-two-helpers";
import { supportsTwoCatanSeafarers } from "../../../games/catan/web/catan-two-seafarers";
import { CatanTwoScenarioPicker } from "../../../games/catan/web/catan-two-setup";
import { twoResponder } from "../../../games/catan/web/catan-two-state";
import { supportsTwoCatanVariants } from "../../../games/catan/web/catan-two-variants";
import { CatanWonderSeat } from "../../../games/catan/web/catan-wonders";
import { CatanWorldEditor } from "../../../games/catan/web/catan-world-editor";
import { DotaRules } from "../../../games/dota/web/dota";
import { RailMapsContext, playerColors } from "../../../games/rail/web/context";
import {
  RailMapPicker,
  RailMapRules,
} from "../../../games/rail/web/rail-expansions";
import { SanguoshaRules } from "../../../games/sanguosha/web/sanguosha";
import { SanguoshaOptions } from "../../../games/sanguosha/web/sanguosha-options";
import {
  Gemstone,
  NobleCard,
  gemColors,
  gemDisplayOrder,
} from "../../../games/splendor/web/board";
import {
  SplendorOptionPicker,
  gemPostNames,
  splendorRulesLabel,
} from "../../../games/splendor/web/splendor-expansions";
import {
  extraNobleName,
  nobleArtwork,
} from "../../../games/splendor/web/splendor-nobles";
import { gemNames } from "../../../games/splendor/web/splendor-orient-state";
import { SplendorRules } from "../../../games/splendor/web/splendor-rules";
import { gameName, isIndependentGame } from "../game-api/registry";
import { PlayerName } from "../profiles";
import type {
  CatanOptions,
  RailMapSpec,
  Room,
  SGOptions,
  SplendorOptions,
} from "../types";
import { AssetsContext, Modal } from "../ui/board-host";
import { LegacyCover } from "./cover";
export { LegacyCover } from "./cover";
export function LegacyCreate({
  kind,
  busy,
  onSubmit,
  draft,
  onDraftChange,
}: GameSetupFormProps) {
  const maps = useContext(RailMapsContext),
    assets = useContext(AssetsContext);
  const [railMap, setRailMap] = useState("usa");
  const [sgOptions, setSGOptions] = useState<SGOptions>({ deck: "standard" });
  const [catanOptions, setCatanOptions] = useState<CatanOptions>({});
  const [catanBaseLayout, setCatanBaseLayout] = useState("variable");
  const [transportSeaLayout, setTransportSeaLayout] = useState("fixed");
  const [catanTwoScenario, setCatanTwoScenario] = useState("");
  const [catanScenario, setCatanScenario] = useState("");
  const [catanFishing, setCatanFishing] = useState(false);
  const [catanFishingLakes, setCatanFishingLakes] = useState(true);
  const [catanHarbors, setCatanHarbors] = useState(false);
  const [catanFriendly, setCatanFriendly] = useState(false);
  const [catanSeaKnights, setCatanSeaKnights] = useState(false);
  const [catanEvents, setCatanEvents] = useState(false);
  const [gemOptions, setGemOptions] = useState<SplendorOptions>({});
  const map = maps.find((m) => m.id === railMap);
  const k = kind;
  const [capacity, setCapacity] = useState(4);
  const variantScenario = capacity === 2 ? catanTwoScenario : catanScenario;
  const variantsAvailable =
    capacity === 2
      ? supportsTwoCatanVariants(catanTwoScenario)
      : capacity >= 3 &&
        (capacity <= 4 ||
          catanOptions.fiveSix ||
          supportsCatanTradersHelpers(catanScenario));
  // Creation, visibility and scenario changes use the same supported scope.
  const fishingAvailable =
    (capacity === 2 && variantScenario === "cities-knights") ||
    supportsPublicCatanFishingSea(variantScenario) ||
    isPublicCatanExplorer(variantScenario) ||
    [
      "rivers",
      "caravans",
      "barbarian-attack",
      "transport",
      "transport-shores",
      "transport-desert",
    ].includes(variantScenario);
  const fishingEnabled = fishingAvailable && catanFishing;
  const knightAddonAvailableFor = (scenario: string, players: number) =>
    (players >= 3 ||
      isPublicCatanFlexible(scenario) ||
      supportsTwoCatanSeafarers(scenario) ||
      ["caravans", "rivers", "barbarian-attack"].includes(scenario)) &&
    supportsPublicCatanKnightsCombination(scenario);
  const knightsAvailable = knightAddonAvailableFor(variantScenario, capacity);
  const variantKnights =
    variantScenario === "cities-knights" ||
    (knightsAvailable && catanSeaKnights);
  const eventsAvailable = catanEventsSupported({
    kind: k,
    capacity,
    catanTwoScenario: capacity === 2 ? catanTwoScenario : undefined,
    catanScenario: capacity === 2 ? catanTwoScenario : catanScenario,
    catanFishing: fishingEnabled,
    catanCitiesKnights: variantKnights
      ? { layout: "variable", rules: "" }
      : undefined,
    catanHarbors: { enabled: catanHarbors, rules: "" },
    catanFriendlyRobber: { enabled: catanFriendly, rules: "" },
  });
  useEffect(() => {
    if (!eventsAvailable) setCatanEvents(false);
  }, [eventsAvailable]);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        onSubmit({
          name: f.get("name"),
          password: f.get("password"),
          kind: k,
          railMap: k === "rail" ? railMap : undefined,
          sanguoshaOptions: k === "sanguosha" ? sgOptions : undefined,
          splendorOptions: k === "splendor" ? gemOptions : undefined,
          catanOptions:
            k === "catan"
              ? capacity === 2 &&
                !isPublicCatanExplorer(catanTwoScenario) &&
                !supportsTwoCatanHelpers(catanTwoScenario)
                ? {}
                : catanOptions
              : undefined,
          catanBaseConfiguration:
            k === "catan" &&
            capacity > 4 &&
            catanOptions.fiveSix &&
            !catanScenario
              ? { layout: catanBaseLayout }
              : undefined,
          catanScenario:
            k === "catan"
              ? capacity === 2
                ? isPublicCatanFlexible(catanTwoScenario)
                  ? catanTwoScenario
                  : undefined
                : catanScenario
              : undefined,
          catanFriendlyRobber:
            k === "catan" &&
            variantsAvailable &&
            catanFriendly &&
            supportsPublicCatanFriendly(variantScenario)
              ? { enabled: true }
              : undefined,
          catanHarbors:
            k === "catan" &&
            variantsAvailable &&
            catanHarbors &&
            supportsPublicCatanHarbors(variantScenario)
              ? { enabled: true }
              : undefined,
          catanFishing: k === "catan" && fishingEnabled,
          catanFishingLakes:
            k === "catan" &&
            isPublicCatanExplorer(catanScenario) &&
            catanFishing &&
            catanFishingLakes,
          catanEvents:
            eventsAvailable && catanEvents ? CATAN_EVENT_CATALOGUE : undefined,
          catanCitiesKnights:
            k === "catan" && knightsAvailable && catanSeaKnights
              ? { layout: "variable" }
              : undefined,
          catanTransportSea:
            k === "catan" && isCatanTransportSea(variantScenario)
              ? {
                  scenario:
                    variantScenario === "transport-desert"
                      ? "desert"
                      : "shores",
                  layout: transportSeaLayout,
                }
              : undefined,
          catanTwoScenario:
            k === "catan" &&
            capacity === 2 &&
            !isPublicCatanFlexible(catanTwoScenario)
              ? catanTwoScenario
              : undefined,
          capacity,
        });
      }}
    >
      <label>
        房间名称
        <input
          name="name"
          required
          maxLength={40}
          placeholder="例如：周五晚上的快乐"
          value={draft.name}
          disabled={busy}
          onChange={(e) => onDraftChange({ name: e.target.value })}
        />
      </label>
      <div className="form-grid">
        <label>
          座位上限
          <select
            aria-label="座位上限"
            disabled={busy}
            value={capacity}
            onChange={(e) => {
              const next = Number(e.target.value);
              setCapacity(next);
              if (
                k === "catan" &&
                capacity === 2 &&
                next !== 2 &&
                !isPublicCatanFlexible(catanScenario)
              ) {
                setCatanFishing(false);
                setCatanFishingLakes(false);
              }
              if (
                k === "catan" &&
                next === 2 &&
                !isPublicCatanExplorer(catanScenario)
              )
                setCatanOptions((o) =>
                  supportsTwoCatanHelpers(catanTwoScenario || catanScenario)
                    ? { helpers: o.helpers, allHelpers: o.allHelpers }
                    : {},
                );
            }}
          >
            {(k === "sanguosha" && sgOptions?.mode === "3v3"
              ? [6]
              : k === "dota"
                ? [2, 4, 6]
                : k === "catan" &&
                    [
                      "transport-shores",
                      "transport-desert",
                      "attack-pirates",
                      "attack-wonders",
                      "attack-tribe",
                      "attack-desert",
                    ].includes(catanScenario)
                  ? [2, 3, 4, 5, 6]
                  : k === "catan" && catanScenario === "attack-shores"
                    ? [2, 3, 4, 5, 6]
                    : k === "catan" &&
                        ["caravans-new-world", "caravans-islands"].includes(
                          catanScenario,
                        )
                      ? [2, 3, 4, 5, 6]
                      : Array.from(
                          {
                            length:
                              k === "catan" &&
                              catanScenario === "caravans-shores"
                                ? 5
                                : k === "catan" &&
                                    [
                                      "caravans-desert",
                                      "caravans-tribe",
                                    ].includes(catanScenario)
                                  ? 5
                                  : k === "sanguosha"
                                    ? 5
                                    : k === "catan"
                                      ? [
                                          "land-ho",
                                          "spices-for-catan",
                                          "pirate-lairs",
                                          "fish-for-catan",
                                          "explorers-and-pirates",
                                          "transport",
                                          "rivers-caravans",
                                          "rivers-attack",
                                          "rivers-transport",
                                          "caravans-attack",
                                          "caravans-transport",
                                          "attack-transport",
                                          "rivers-shores",
                                          "rivers-fog",
                                          "rivers-desert",
                                          "rivers-desert-belt",
                                          "rivers-tribe",
                                          "transport-shores",
                                          "transport-desert",
                                          "attack-pirates",
                                          "attack-wonders",
                                          "attack-tribe",
                                          "attack-desert",
                                          "attack-shores",
                                          "caravans-shores",
                                          "caravans-new-world",
                                          "caravans-islands",
                                          "caravans-tribe",
                                          "caravans-desert",
                                          "rivers-new-world",
                                        ].includes(catanScenario)
                                        ? 5
                                        : catanScenario === "barbarian-attack"
                                          ? 4
                                          : catanOptions.fiveSix ||
                                              (capacity !== 2 &&
                                                catanSeaKnights) ||
                                              (capacity !== 2 &&
                                                catanFishing &&
                                                !isPublicCatanExplorer(
                                                  catanScenario,
                                                )) ||
                                              [
                                                "cities-knights",
                                                "fishing",
                                                "barbarian-attack",
                                              ].includes(catanScenario)
                                            ? 2
                                            : 3
                                      : k === "rail"
                                        ? (map?.maxPlayers || 5) - 1
                                        : k === "carcassonne"
                                          ? 4
                                          : 3,
                          },
                          (_, i) =>
                            k === "catan" && catanScenario === "caravans-shores"
                              ? [2, 3, 4, 5, 6][i]
                              : i +
                                (k === "catan" &&
                                catanScenario === "caravans-shores"
                                  ? 4
                                  : k === "catan" &&
                                      [
                                        "caravans-desert",
                                        "caravans-tribe",
                                      ].includes(catanScenario)
                                    ? 2
                                    : k === "sanguosha"
                                      ? 4
                                      : k === "catan"
                                        ? isPublicCatanFlexible(catanScenario)
                                          ? 2
                                          : catanOptions.fiveSix
                                            ? 5
                                            : (capacity !== 2 &&
                                                  catanSeaKnights &&
                                                  !isPublicCatanExplorer(
                                                    catanScenario,
                                                  )) ||
                                                (capacity !== 2 &&
                                                  catanFishing &&
                                                  !isPublicCatanExplorer(
                                                    catanScenario,
                                                  )) ||
                                                [
                                                  "cities-knights",
                                                  "fishing",
                                                  "barbarian-attack",
                                                ].includes(catanScenario)
                                              ? 3
                                              : 2
                                        : 2),
                        )
            ).map((n) => (
              <option key={n} value={n}>
                {n} 人
              </option>
            ))}
          </select>
        </label>
        <label>
          房间密码 <small>可选</small>
          <input
            name="password"
            value={draft.password}
            disabled={busy}
            onChange={(e) => onDraftChange({ password: e.target.value })}
            type="password"
            maxLength={72}
            placeholder="留空即可直接加入"
            autoComplete="off"
          />
        </label>
      </div>
      {k === "sanguosha" && (
        <SanguoshaOptions
          value={sgOptions}
          onChange={(options) => {
            setSGOptions(options);
            if (options.mode === "3v3") setCapacity(6);
          }}
        />
      )}
      {k === "catan" && capacity === 2 && (
        <CatanTwoScenarioPicker
          value={catanTwoScenario}
          knightsEnabled={knightsAvailable && catanSeaKnights}
          fishingEnabled={fishingEnabled}
          target={
            catanVictoryTarget(
              catanTwoScenario,
              variantKnights,
              fishingEnabled,
            ) + (variantsAvailable && catanHarbors ? 1 : 0)
          }
          onChange={(scenario) => {
            setCatanTwoScenario(scenario);
            if (!supportsTwoCatanVariants(scenario)) {
              setCatanHarbors(false);
              setCatanFriendly(false);
            }
            setCatanScenario(isPublicCatanFlexible(scenario) ? scenario : "");
            if (!isPublicCatanExplorer(scenario)) {
              if (!supportsTwoCatanHelpers(scenario)) setCatanOptions({});
              if (
                !supportsPublicCatanFishingSea(scenario) &&
                scenario !== "cities-knights" &&
                ![
                  "rivers",
                  "caravans",
                  "barbarian-attack",
                  "transport",
                  "transport-shores",
                  "transport-desert",
                ].includes(scenario)
              )
                setCatanFishing(false);
              setCatanFishingLakes(false);
            }
            if (!knightAddonAvailableFor(scenario, 2))
              setCatanSeaKnights(false);
          }}
        />
      )}
      {k === "catan" &&
        capacity >= 3 &&
        (capacity <= 4 ||
          (catanOptions.fiveSix &&
            (!catanScenario ||
              catanScenario === "cities-knights" ||
              isPublicCatanSea(catanScenario) ||
              ["rivers", "caravans", "fishing"].includes(catanScenario))) ||
          (catanFishing &&
            supportsPublicCatanFishingSeaExtended(catanScenario)) ||
          [
            "land-ho",
            "spices-for-catan",
            "pirate-lairs",
            "fish-for-catan",
            "explorers-and-pirates",
            "barbarian-attack",
            "transport",
            "rivers-caravans",
            "rivers-attack",
            "rivers-transport",
            "caravans-attack",
            "caravans-transport",
            "attack-transport",
            "rivers-shores",
            "rivers-fog",
            "rivers-desert",
            "rivers-desert-belt",
            "rivers-tribe",
            "transport-shores",
            "transport-desert",
            "attack-pirates",
            "attack-wonders",
            "attack-tribe",
            "attack-desert",
            "attack-shores",
            "caravans-shores",
            "caravans-new-world",
            "caravans-islands",
            "caravans-tribe",
            "caravans-desert",
            "rivers-new-world",
          ].includes(catanScenario)) && (
          <CatanScenarioPicker
            players={capacity}
            fiveSix={!!catanOptions.fiveSix}
            value={catanScenario}
            helpers={!!catanOptions.helpers}
            knights={catanSeaKnights}
            fishing={catanFishing}
            friendly={catanFriendly}
            harbors={catanHarbors}
            onChange={(scenario) => {
              setCatanScenario(scenario);
              if (
                [
                  "transport-shores",
                  "transport-desert",
                  "attack-pirates",
                  "attack-wonders",
                  "attack-tribe",
                  "attack-desert",
                ].includes(scenario)
              ) {
                setCapacity(Math.min(6, Math.max(2, capacity)));
                setCatanOptions((o) => ({
                  helpers: o.helpers,
                  allHelpers: o.allHelpers,
                }));
              }
              if (scenario === "attack-shores") {
                setCapacity(Math.min(6, Math.max(2, capacity)));
                setCatanOptions((o) => ({
                  helpers: o.helpers,
                  allHelpers: o.allHelpers,
                }));
              }
              if (scenario === "caravans-islands")
                setCapacity(Math.min(6, Math.max(2, capacity)));
              if (scenario === "caravans-shores")
                setCapacity(Math.max(2, capacity));
              if (!supportsPublicCatanFriendly(scenario))
                setCatanFriendly(false);
              if (!supportsPublicCatanHarbors(scenario)) setCatanHarbors(false);
              if (
                !supportsPublicCatanFishingSea(scenario) &&
                !isPublicCatanExplorer(scenario) &&
                ![
                  "rivers",
                  "caravans",
                  "barbarian-attack",
                  "transport",
                  "transport-shores",
                  "transport-desert",
                ].includes(scenario)
              )
                setCatanFishing(false);
              if (!supportsPublicCatanKnightsCombination(scenario))
                setCatanSeaKnights(false);
              if (isPublicCatanFlexible(scenario))
                setCatanTwoScenario(scenario);
              if (
                [
                  "rivers-caravans",
                  "rivers-attack",
                  "rivers-transport",
                  "caravans-attack",
                  "caravans-transport",
                  "attack-transport",
                  "rivers-shores",
                  "rivers-fog",
                  "rivers-desert",
                  "rivers-desert-belt",
                  "rivers-tribe",
                  "transport-shores",
                  "transport-desert",
                  "attack-pirates",
                  "attack-wonders",
                  "attack-tribe",
                  "attack-desert",
                  "attack-shores",
                  "caravans-shores",
                  "caravans-new-world",
                  "caravans-islands",
                  "caravans-tribe",
                  "caravans-desert",
                  "rivers-new-world",
                ].includes(scenario || "")
              )
                setCatanOptions((o) => ({
                  helpers: o.helpers,
                  allHelpers: o.allHelpers,
                }));
              if (
                ![
                  "land-ho",
                  "spices-for-catan",
                  "pirate-lairs",
                  "fish-for-catan",
                  "explorers-and-pirates",
                  "barbarian-attack",
                  "transport",
                  "rivers-caravans",
                  "rivers-attack",
                  "rivers-transport",
                  "caravans-attack",
                  "caravans-transport",
                  "attack-transport",
                  "rivers-shores",
                  "rivers-fog",
                  "rivers-desert",
                  "rivers-desert-belt",
                  "rivers-tribe",
                  "transport-shores",
                  "transport-desert",
                  "attack-pirates",
                  "attack-wonders",
                  "attack-tribe",
                  "attack-desert",
                  "attack-shores",
                  "caravans-shores",
                  "caravans-new-world",
                  "caravans-islands",
                  "caravans-tribe",
                  "caravans-desert",
                  "rivers-new-world",
                ].includes(scenario) &&
                !(
                  catanOptions.fiveSix &&
                  (!scenario ||
                    scenario === "cities-knights" ||
                    isPublicCatanSea(scenario) ||
                    ["rivers", "caravans", "fishing"].includes(scenario))
                ) &&
                !(
                  catanFishing &&
                  supportsPublicCatanFishingSeaExtended(scenario)
                )
              )
                setCapacity(Math.min(4, capacity));
              if (["rivers", "caravans", "cities-knights"].includes(scenario))
                setCatanOptions({
                  fiveSix: catanOptions.fiveSix,
                  helpers: catanOptions.helpers,
                  allHelpers: catanOptions.allHelpers,
                });
              else if (isPublicCatanExplorer(scenario))
                setCatanOptions({
                  helpers: catanOptions.helpers,
                  allHelpers: catanOptions.allHelpers,
                });
              else if (
                scenario &&
                scenario !== "fishing" &&
                !isPublicCatanSea(scenario)
              )
                setCatanOptions(
                  supportsCatanTradersHelpers(scenario)
                    ? {
                        helpers: catanOptions.helpers,
                        allHelpers: catanOptions.allHelpers,
                      }
                    : {},
                );
            }}
          />
        )}
      {k === "catan" && isCatanTransportSea(variantScenario) && (
        <CatanTransportSeaLayout
          value={transportSeaLayout}
          onChange={setTransportSeaLayout}
        />
      )}
      {k === "catan" && fishingAvailable && (
        <CatanFishingSeaPicker
          transport={
            variantScenario === "transport" ||
            isCatanTransportSea(variantScenario)
          }
          transportSea={isCatanTransportSea(variantScenario)}
          attack={["barbarian-attack", "rivers-attack"].includes(
            variantScenario,
          )}
          attackSea={isPublicCatanAttackSea(variantScenario)}
          caravans={variantScenario === "caravans"}
          rivers={variantScenario === "rivers"}
          knights={
            catanSeaKnights ||
            (capacity === 2 && catanTwoScenario === "cities-knights")
          }
          value={catanFishing}
          two={capacity === 2 && supportsTwoCatanSeafarers(catanTwoScenario)}
          riversSea={isPublicCatanRiversSea(variantScenario)}
          caravansSea={isPublicCatanCaravanSea(variantScenario)}
          shores={variantScenario === "shores"}
          explorer={isPublicCatanExplorer(catanScenario)}
          extended={capacity > 4}
          onChange={setCatanFishing}
          lakes={catanFishingLakes}
          onLakes={setCatanFishingLakes}
        />
      )}
      {k === "catan" && knightsAvailable && (
        <CatanCombinationKnightsPicker
          attack={["barbarian-attack", "rivers-attack"].includes(
            variantScenario,
          )}
          riversAttack={variantScenario === "rivers-attack"}
          riversTransport={variantScenario === "rivers-transport"}
          caravansAttack={variantScenario === "caravans-attack"}
          attackTransport={variantScenario === "attack-transport"}
          caravansTransport={variantScenario === "caravans-transport"}
          transport={
            variantScenario === "transport" ||
            isCatanTransportSea(variantScenario)
          }
          transportSea={isCatanTransportSea(variantScenario)}
          rivers={variantScenario === "rivers"}
          riversSea={isPublicCatanRiversSea(variantScenario)}
          caravansSea={isPublicCatanCaravanSea(variantScenario)}
          attackSea={isPublicCatanAttackSea(variantScenario)}
          riversCaravans={variantScenario === "rivers-caravans"}
          caravans={["caravans", "rivers-caravans"].includes(variantScenario)}
          tribe={variantScenario === "tribe"}
          pirateIslands={catanScenario === "pirate_islands"}
          explorer={supportsPublicExplorerKnights(catanScenario)}
          two={capacity === 2}
          intro={catanScenario === "land-ho"}
          fishing={fishingEnabled || variantScenario === "fishing"}
          harbors={catanHarbors}
          value={catanSeaKnights}
          helpers={!!catanOptions.helpers}
          onChange={setCatanSeaKnights}
        />
      )}
      {k === "catan" &&
        variantsAvailable &&
        supportsPublicCatanHarbors(variantScenario) && (
          <CatanHarborsChoice
            two={capacity === 2}
            value={catanHarbors}
            onChange={setCatanHarbors}
            helpers={!!catanOptions.helpers}
            conquered={[
              "barbarian-attack",
              "attack-transport",
              "caravans-attack",
              "rivers-attack",
              "attack-shores",
              "attack-desert",
              "attack-tribe",
              "attack-wonders",
              "attack-pirates",
            ].includes(variantScenario)}
            target={
              catanVictoryTarget(
                variantScenario,
                variantKnights,
                catanFishing,
              ) + (catanHarbors ? 1 : 0)
            }
          />
        )}
      {k === "catan" &&
        variantsAvailable &&
        supportsPublicCatanFriendly(variantScenario) && (
          <CatanFriendlyRobberChoice
            two={capacity === 2}
            value={catanFriendly}
            onChange={setCatanFriendly}
            knights={variantKnights}
            scenario={variantScenario}
            traders={supportsCatanTradersHelpers(variantScenario)}
            noRobber={[
              "barbarian-attack",
              "transport",
              "attack-transport",
              "caravans-transport",
              "caravans-attack",
              "rivers-transport",
              "rivers-attack",
              "attack-shores",
              "attack-desert",
              "attack-tribe",
              "attack-wonders",
              "attack-pirates",
              "transport-shores",
              "transport-desert",
            ].includes(variantScenario)}
            fishing={catanFishing || variantScenario === "fishing"}
          />
        )}
      {k === "catan" &&
        capacity === 2 &&
        supportsTwoCatanHelpers(catanTwoScenario) && (
          <>
            <CatanOptionPicker
              value={catanOptions}
              traders={supportsCatanTradersHelpers(variantScenario)}
              onChange={setCatanOptions}
              fiveSixAvailable={false}
              citiesKnights={variantKnights}
              fishing={catanFishing || catanTwoScenario === "fishing"}
              harbors={catanHarbors}
              friendlyRobber={catanFriendly}
            />
            {catanOptions.helpers && (
              <p className="muted small">{catanTwoHelpersNote}</p>
            )}
          </>
        )}
      {k === "catan" &&
        (isPublicCatanExplorer(
          capacity === 2 ? catanTwoScenario : catanScenario,
        ) ||
          (capacity !== 2 &&
            supportsCatanTradersHelpers(catanScenario) &&
            !["rivers", "caravans"].includes(catanScenario))) && (
          <CatanOptionPicker
            explorer={isPublicCatanExplorer(
              capacity === 2 ? catanTwoScenario : catanScenario,
            )}
            seafarers={supportsCaravanSeaHelpers(catanScenario)}
            fiveSixAvailable={false}
            value={catanOptions}
            traders={supportsCatanTradersHelpers(variantScenario)}
            onChange={setCatanOptions}
          />
        )}
      {k === "catan" &&
        capacity !== 2 &&
        (!catanFishing ||
          supportsPublicCatanFishingSeaExtended(catanScenario)) &&
        (!catanScenario ||
          ["cities-knights", "rivers", "caravans", "fishing"].includes(
            catanScenario,
          ) ||
          isPublicCatanSea(catanScenario)) && (
          <CatanOptionPicker
            value={catanOptions}
            traders={supportsCatanTradersHelpers(variantScenario)}
            helpersAvailable
            fishing={fishingEnabled || variantScenario === "fishing"}
            seafarers={isPublicCatanSea(catanScenario)}
            citiesKnights={
              catanSeaKnights || catanScenario === "cities-knights"
            }
            harbors={catanHarbors}
            friendlyRobber={catanFriendly}
            fiveSixAvailable={
              !catanFishing ||
              supportsPublicCatanFishingSeaExtended(catanScenario)
            }
            onChange={(o) => {
              setCatanOptions(o);
              setCapacity(
                o.fiveSix ? Math.max(5, capacity) : Math.min(4, capacity),
              );
            }}
          />
        )}
      {k === "catan" &&
        capacity > 4 &&
        catanOptions.fiveSix &&
        !catanScenario && (
          <fieldset className="catan-helper-options catan-scenario-options">
            <legend>基础地图布局</legend>
            <label>
              地图布局
              <select
                value={catanBaseLayout}
                onChange={(e) => setCatanBaseLayout(e.target.value)}
              >
                <option value="variable">随机布局</option>
                <option value="fixed">固定新手布局</option>
              </select>
            </label>
            <p>
              {catanBaseLayout === "fixed"
                ? "使用规则书固定地图，随机分配颜色，预放两座村庄和两条道路并领取起始资源。五人局保留剩余颜色的两座中立村庄，直接开始掷骰。"
                : "随机安排地形，开局后依次选择村庄和道路的位置。"}
            </p>
          </fieldset>
        )}
      {eventsAvailable && (
        <CatanEventPicker
          value={catanEvents}
          explorer={[
            "land-ho",
            "pirate-lairs",
            "fish-for-catan",
            "spices-for-catan",
            "explorers-and-pirates",
          ].includes(catanScenario)}
          transport={catanScenario === "transport"}
          pirateIslands={catanScenario === "pirate_islands"}
          fishing={fishingEnabled || variantScenario === "fishing"}
          friendly={catanFriendly}
          tradersNoRobber={
            supportsCatanTradersHelpers(variantScenario) &&
            (variantScenario.includes("attack") ||
              variantScenario.includes("transport"))
          }
          cloth={
            (capacity === 2 ? catanTwoScenario : catanScenario) === "cloth"
          }
          citiesKnights={variantKnights}
          disabled={busy}
          onChange={setCatanEvents}
        />
      )}
      {k === "splendor" && (
        <SplendorOptionPicker value={gemOptions} onChange={setGemOptions} />
      )}
      {k === "rail" && (
        <RailMapPicker
          maps={maps}
          assets={assets}
          value={railMap}
          onChange={(id) => {
            setRailMap(id);
            setCapacity(
              Math.min(capacity, maps.find((m) => m.id === id)!.maxPlayers),
            );
          }}
        />
      )}
      <p className="muted small">
        至少{" "}
        {k === "sanguosha"
          ? 4
          : k === "catan"
            ? capacity === 2 || isPublicCatanFlexible(catanScenario)
              ? 2
              : catanOptions.fiveSix
                ? 5
                : 3
            : 2}
        个座位即可开始，不必坐满。一个人也可以在房间里添加电脑玩家体验。
        {k === "dota" && " 兵线争锋需 2、4 或 6 人，开始后再分队。"}
      </p>
      <button className="primary wide" disabled={busy} type="submit">
        创建房间
        <ArrowRight size={18} />
      </button>
    </form>
  );
}
export function LegacyWaiting({
  room,
  busy,
  host,
  command,
}: {
  room: Room;
  busy: boolean;
  host: boolean;
  command: (t: string, extra?: Record<string, unknown>) => void;
}) {
  const [mapDirty, setMapDirty] = useState(false);
  const maps = useContext(RailMapsContext),
    assets = useContext(AssetsContext);
  const map = maps.find((m) => m.id === (room.railMap || "usa"));
  const seaInfo = room.catanSeafarersChoices?.find(
    (s) => s.id === room.catanSeafarers?.scenario,
  );
  const scenarioSelectable =
    room.kind === "catan" &&
    !room.catanTwoRules &&
    room.capacity >= 3 &&
    (room.capacity <= 4 ||
      (room.catanOptions?.fiveSix &&
        (!room.catanScenario ||
          room.catanScenario === "cities-knights" ||
          isPublicCatanSea(room.catanScenario) ||
          ["rivers", "caravans", "fishing"].includes(
            room.catanScenario || "",
          ))) ||
      (room.catanFishing &&
        supportsPublicCatanFishingSeaExtended(room.catanScenario)) ||
      [
        "land-ho",
        "spices-for-catan",
        "pirate-lairs",
        "fish-for-catan",
        "explorers-and-pirates",
        "barbarian-attack",
        "transport",
        "rivers-caravans",
        "rivers-attack",
        "rivers-transport",
        "caravans-attack",
        "caravans-transport",
        "attack-transport",
        "rivers-shores",
        "rivers-fog",
        "rivers-desert",
        "rivers-desert-belt",
        "rivers-tribe",
        "transport-shores",
        "transport-desert",
        "attack-pirates",
        "attack-wonders",
        "attack-tribe",
        "attack-desert",
        "attack-shores",
        "caravans-shores",
        "caravans-new-world",
        "caravans-islands",
        "caravans-tribe",
        "caravans-desert",
        "rivers-new-world",
      ].includes(room.catanScenario || "")) &&
    (!room.catanOptions?.fiveSix ||
      room.catanFishing ||
      !room.catanScenario ||
      room.catanScenario === "cities-knights" ||
      isPublicCatanSea(room.catanScenario) ||
      ["rivers", "caravans", "fishing"].includes(room.catanScenario || "")) &&
    (!room.catanBaseConfiguration || !room.catanScenario) &&
    (!room.catanCitiesKnights ||
      room.catanScenario === "cities-knights" ||
      room.catanScenario === "fishing" ||
      supportsPublicCatanKnightsCombination(room.catanScenario) ||
      isPublicCatanSea(room.catanScenario)) &&
    (!room.catanSeafarers || isPublicCatanSea(room.catanScenario)) &&
    (!room.catanNewWorldMap ||
      ["new_world", "caravans-new-world"].includes(room.catanScenario || ""));
  const twoLabel =
    room.kind === "catan" && room.catanTwoRules
      ? `双人卡坦${room.catanTwoScenario ? "＋" + catanScenarioName(room.catanTwoScenario) : ""}${room.catanCitiesKnights ? "＋城市与骑士" : ""}${room.catanFishing ? "＋渔夫" : ""}${room.catanOptions?.helpers ? "＋Helpers" : ""}${room.catanHarbors?.enabled ? "＋港口霸主" : ""}${room.catanFriendlyRobber?.enabled ? "＋友善强盗" : ""} · 2 人 · ${catanRuleContext(room).target} 分获胜`
      : "";
  const ready =
    room.seats.every((p) => p.ready) &&
    (!room.catanFriendlyRobber?.enabled ||
      room.seats.length >=
        (room.catanFriendlyRobberAvailability?.minPlayers || 3)) &&
    (room.kind !== "dota" || room.seats.length % 2 === 0) &&
    room.seats.length >=
      (room.kind === "sanguosha"
        ? 4
        : room.kind === "catan"
          ? room.catanScenario === "attack-shores"
            ? 2
            : room.catanTwoRules || isPublicCatanFlexible(room.catanScenario)
              ? 2
              : room.catanOptions?.fiveSix
                ? 5
                : 3
          : 2);
  return (
    <div className="waiting-layout">
      {room.kind === "catan" && room.catanScenario === "rivers-new-world" && (
        <CatanRiversWorldPreview
          onDirty={setMapDirty}
          room={room}
          host={host}
          busy={busy}
          command={command}
        />
      )}
      {room.kind === "catan" && room.catanNewWorldMap && (
        <CatanWorldEditor
          room={room}
          assets={assets}
          host={host}
          busy={busy}
          command={command}
          onDirty={setMapDirty}
        />
      )}

      <div className="waiting-cover">
        <Cover kind={room.kind} />
        <h2>{gameName(room.kind)}</h2>
        <p>
          {room.kind === "dota"
            ? "先入座，再结盟；让每一次出牌推动兵线。"
            : room.kind === "sanguosha"
              ? "与好友过招，也与身份博弈。"
              : room.kind === "carcassonne"
                ? "一块接一块，拼出属于你们的城邦。"
                : room.kind === "catan"
                  ? "一座岛屿，无数种通往胜利的道路。"
                  : room.kind === "splendor"
                    ? "每颗宝石，都是通往胜利的可能。"
                    : "下一站，会是你的目的地吗？"}
        </p>
        <span className="tag">
          {room.kind === "dota"
            ? "1v1 / 2v2 / 3v3 · 开始后分队 · 全部拆遗迹"
            : room.kind === "sanguosha"
              ? room.sanguoshaOptions?.mode === "3v3"
                ? "3v3 竞技 · 6 人 · 抓将与主帅阵亡判负"
                : room.sanguoshaOptions?.mode === "hegemony"
                  ? "基础国战 · 4–8 人 · 60 将 / 108 张牌"
                  : `身份局 · 4–8 人 · ${room.sanguoshaOptions?.deck === "military" ? "标准＋军争 / 160 张牌" : "标准 / 108 张牌"}`
              : room.kind === "carcassonne"
                ? "基础版 · 2–5 人 · 包含农民"
                : room.kind === "catan"
                  ? room.catanTwoRules
                    ? twoLabel
                    : `${room.catanScenario ? (seaInfo ? `航海家 · ${seaInfo.name}` : catanScenarioName(room.catanScenario)) : room.catanCitiesKnights ? `城市与骑士 · ${seaInfo ? `航海家 · ${seaInfo.name}` : "随机地图"}` : seaInfo ? `航海家 · ${seaInfo.name}` : room.catanNewWorldMap ? "航海家 · 新世界" : `基础版${room.catanBaseConfiguration ? ` · ${catanBaseLayoutName(room.catanBaseConfiguration.layout)}` : ""}`}${room.catanCitiesKnights ? "＋城市与骑士" : ""}${room.catanFishing ? "＋渔夫" : ""}${room.catanOptions?.helpers ? "＋Helpers" : ""}${room.catanHarbors?.enabled ? "＋港口霸主" : ""}${room.catanFriendlyRobber?.enabled ? "＋友善强盗" : ""} · ${
                        room.catanOptions?.fiveSix
                          ? "5–6 人 · 配对回合"
                          : room.catanFriendlyRobber?.enabled &&
                              room.catanFriendlyRobberAvailability
                                ?.minPlayers === 4
                            ? "4 人"
                            : ["barbarian-attack"].includes(
                                  room.catanScenario || "",
                                )
                              ? "3–6 人 · 五六人配对回合"
                              : [
                                    "land-ho",
                                    "spices-for-catan",
                                    "pirate-lairs",
                                    "fish-for-catan",
                                    "explorers-and-pirates",
                                    "transport",
                                    "rivers-caravans",
                                    "rivers-attack",
                                    "rivers-transport",
                                    "caravans-attack",
                                    "caravans-transport",
                                    "attack-transport",
                                    "rivers-shores",
                                    "rivers-fog",
                                    "rivers-desert",
                                    "rivers-desert-belt",
                                    "rivers-tribe",
                                    "transport-shores",
                                    "transport-desert",
                                    "attack-pirates",
                                    "attack-wonders",
                                    "attack-tribe",
                                    "attack-desert",
                                    "attack-shores",
                                    "caravans-shores",
                                    "caravans-new-world",
                                    "caravans-islands",
                                    "caravans-tribe",
                                    "caravans-desert",
                                    "rivers-new-world",
                                  ].includes(room.catanScenario || "")
                                ? "2–6 人 · 五六人配对回合"
                                : isPublicCatanFlexible(room.catanScenario)
                                  ? "2–4 人"
                                  : "3–4 人"
                      } · ${catanScenarioVictory(catanRuleContext(room).scenario, catanRuleContext(room).target)}`
                  : room.kind === "splendor"
                    ? `${splendorRulesLabel(room.splendorOptions)} · 2–4 人`
                    : `${map?.name || "美国"}地图 · 2–${map?.maxPlayers || 5} 人`}
        </span>
      </div>
      <div className="waiting-seats">
        <span className="eyebrow">TAKE YOUR SEAT</span>
        <h2>朋友或电脑，到齐就开局。</h2>
        {(twoLabel ||
          (room.kind === "catan" &&
            room.capacity === 2 &&
            isPublicCatanFlexible(room.catanScenario))) && (
          <CatanTwoScenarioPicker
            value={
              isPublicCatanFlexible(room.catanScenario)
                ? room.catanScenario!
                : room.catanTwoScenario || ""
            }
            disabled={!host || busy}
            target={catanRuleContext(room).target}
            fishingEnabled={!!room.catanFishing}
            knightsEnabled={!!room.catanCitiesKnights}
            helpersEnabled={!!room.catanOptions?.helpers}
            variantsEnabled={
              !!room.catanFriendlyRobber?.enabled ||
              !!room.catanHarbors?.enabled
            }
            onChange={(catanTwoScenario) =>
              isPublicCatanFlexible(catanTwoScenario)
                ? command("catan_scenario", { catanScenario: catanTwoScenario })
                : command("catan_two_scenario", { catanTwoScenario })
            }
          />
        )}
        {scenarioSelectable && (
          <CatanScenarioPicker
            value={room.catanScenario || ""}
            players={room.capacity}
            fiveSix={!!room.catanOptions?.fiveSix}
            fishing={!!room.catanFishing}
            friendly={!!room.catanFriendlyRobber?.enabled}
            harbors={!!room.catanHarbors?.enabled}
            helpers={!!room.catanOptions?.helpers}
            knights={
              !!room.catanCitiesKnights &&
              supportsPublicCatanKnightsCombination(room.catanScenario)
            }
            disabled={!host || busy || mapDirty}
            onChange={(catanScenario) =>
              command("catan_scenario", { catanScenario })
            }
          />
        )}
        <CatanTransportSeaWaitingLayout
          room={room}
          disabled={!host || busy || mapDirty}
          command={command}
        />
        {room.kind === "catan" &&
          ((!!room.catanTwoRules &&
            room.catanTwoScenario === "cities-knights") ||
            supportsTwoCatanSeafarers(room.catanTwoScenario) ||
            isPublicCatanExplorer(room.catanScenario) ||
            [
              "rivers",
              "caravans",
              "barbarian-attack",
              "transport",
              "transport-shores",
              "transport-desert",
            ].includes(room.catanTwoScenario || room.catanScenario || "") ||
            ((room.capacity <= 4 ||
              supportsPublicCatanFishingSeaExtended(room.catanScenario)) &&
              supportsPublicCatanFishingSea(room.catanScenario))) && (
            <CatanFishingSeaPicker
              transport={
                room.catanScenario === "transport" ||
                isCatanTransportSea(room.catanScenario)
              }
              transportSea={isCatanTransportSea(room.catanScenario)}
              attack={["barbarian-attack", "rivers-attack"].includes(
                room.catanTwoScenario || room.catanScenario || "",
              )}
              attackSea={isPublicCatanAttackSea(room.catanScenario)}
              caravans={
                (room.catanTwoScenario || room.catanScenario) === "caravans"
              }
              rivers={
                (room.catanTwoScenario || room.catanScenario) === "rivers"
              }
              knights={
                !!room.catanTwoRules &&
                room.catanTwoScenario === "cities-knights"
              }
              value={!!room.catanFishing}
              two={!!room.catanTwoRules}
              riversSea={isPublicCatanRiversSea(room.catanScenario)}
              caravansSea={isPublicCatanCaravanSea(room.catanScenario)}
              shores={
                (room.catanTwoScenario || room.catanScenario) === "shores"
              }
              explorer={isPublicCatanExplorer(room.catanScenario)}
              lakes={!!room.catanFishingLakes}
              onLakes={(enabled) => command("catan_fishing_lakes", { enabled })}
              fixedRequired={
                (["desert", "tribe"].includes(
                  room.catanTwoScenario || room.catanScenario || "",
                ) ||
                  (room.capacity > 4 &&
                    ["islands", "cloth"].includes(room.catanScenario || ""))) &&
                room.catanSeafarers?.layout !== "fixed"
              }
              extended={room.capacity > 4}
              disabled={!host || busy || mapDirty}
              onChange={(enabled) => command("catan_fishing", { enabled })}
            />
          )}
        {room.kind === "catan" &&
          (room.capacity >= 3 ||
            isPublicCatanFlexible(room.catanScenario) ||
            supportsTwoCatanSeafarers(room.catanTwoScenario) ||
            ["caravans", "rivers", "barbarian-attack"].includes(
              room.catanTwoScenario || "",
            )) &&
          supportsPublicCatanKnightsCombination(
            room.catanTwoScenario || room.catanScenario,
          ) && (
            <CatanCombinationKnightsPicker
              attack={["barbarian-attack", "rivers-attack"].includes(
                room.catanTwoScenario || room.catanScenario || "",
              )}
              transport={
                room.catanScenario === "transport" ||
                isCatanTransportSea(room.catanScenario)
              }
              transportSea={isCatanTransportSea(room.catanScenario)}
              riversCaravans={room.catanScenario === "rivers-caravans"}
              riversAttack={room.catanScenario === "rivers-attack"}
              riversTransport={room.catanScenario === "rivers-transport"}
              caravansAttack={room.catanScenario === "caravans-attack"}
              attackTransport={room.catanScenario === "attack-transport"}
              caravansTransport={room.catanScenario === "caravans-transport"}
              rivers={
                (room.catanTwoScenario || room.catanScenario) === "rivers"
              }
              riversSea={isPublicCatanRiversSea(room.catanScenario)}
              caravansSea={isPublicCatanCaravanSea(room.catanScenario)}
              attackSea={isPublicCatanAttackSea(room.catanScenario)}
              caravans={["caravans", "rivers-caravans"].includes(
                room.catanTwoScenario || room.catanScenario || "",
              )}
              tribe={(room.catanTwoScenario || room.catanScenario) === "tribe"}
              pirateIslands={room.catanScenario === "pirate_islands"}
              explorer={supportsPublicExplorerKnights(room.catanScenario)}
              two={room.capacity === 2}
              intro={room.catanScenario === "land-ho"}
              fishing={!!room.catanFishing || room.catanScenario === "fishing"}
              harbors={!!room.catanHarbors?.enabled}
              value={!!room.catanCitiesKnights}
              helpers={!!room.catanOptions?.helpers}
              disabled={!host || busy || mapDirty}
              onChange={(enabled) =>
                command("catan_cities_knights", {
                  catanCitiesKnights: enabled ? { layout: "variable" } : null,
                })
              }
            />
          )}
        {catanEventsSupported(room) && (
          <CatanEventPicker
            value={!!room.catanEvents}
            explorer={[
              "land-ho",
              "pirate-lairs",
              "fish-for-catan",
              "spices-for-catan",
              "explorers-and-pirates",
            ].includes(room.catanScenario || "")}
            transport={
              room.catanScenario === "transport" ||
              isCatanTransportSea(room.catanScenario)
            }
            pirateIslands={
              (room.catanSeafarers?.scenario || room.catanScenario) ===
              "pirate_islands"
            }
            fishing={room.catanFishing || room.catanScenario === "fishing"}
            friendly={!!room.catanFriendlyRobber?.enabled}
            tradersNoRobber={
              catanRuleContext(room).attack || catanRuleContext(room).transport
            }
            cloth={
              (room.catanSeafarers?.scenario || room.catanScenario) === "cloth"
            }
            citiesKnights={catanRuleContext(room).citiesKnights}
            disabled={!host || busy || mapDirty}
            onChange={(enabled) => command("catan_events", { enabled })}
          />
        )}
        {room.kind === "catan" && !room.catanTwoRules && (
          <CatanCitiesKnightsSetup room={room} />
        )}
        {room.kind === "catan" &&
          (room.catanTwoRules
            ? supportsTwoCatanVariants(room.catanTwoScenario)
            : supportsPublicCatanFriendly(room.catanScenario)) && (
            <CatanFriendlyRobberPicker
              room={room}
              disabled={!host || busy || mapDirty}
              command={command}
            />
          )}
        {room.kind === "catan" &&
          (room.catanTwoRules
            ? supportsTwoCatanVariants(room.catanTwoScenario)
            : supportsPublicCatanHarbors(room.catanScenario)) && (
            <CatanHarborsPicker
              room={room}
              disabled={!host || busy || mapDirty}
              command={command}
            />
          )}
        {room.kind === "catan" &&
          !room.catanTwoRules &&
          !room.catanScenario && (
            <CatanBasePicker
              room={room}
              disabled={!host || busy || mapDirty}
              command={command}
            />
          )}
        {room.kind === "catan" &&
          (!room.catanTwoRules ||
            supportsTwoCatanSeafarers(room.catanTwoScenario)) &&
          (!room.catanScenario || isPublicCatanSea(room.catanScenario)) && (
            <CatanSeafarersPicker
              room={room}
              hideScenario={
                !!room.catanTwoRules || isPublicCatanSea(room.catanScenario)
              }
              disabled={!host || busy || mapDirty}
              command={command}
            />
          )}
        {room.kind === "catan" &&
          room.catanTwoRules &&
          supportsTwoCatanHelpers(room.catanTwoScenario) && (
            <>
              <CatanOptionPicker
                value={room.catanOptions}
                traders={supportsCatanTradersHelpers(
                  room.catanScenario || room.catanTwoScenario,
                )}
                fiveSixAvailable={false}
                citiesKnights={
                  !!room.catanCitiesKnights ||
                  room.catanTwoScenario === "cities-knights"
                }
                fishing={
                  !!room.catanFishing || room.catanTwoScenario === "fishing"
                }
                harbors={!!room.catanHarbors?.enabled}
                friendlyRobber={!!room.catanFriendlyRobber?.enabled}
                disabled={!host || busy || mapDirty}
                onChange={(catanOptions) =>
                  command("catan_options", { catanOptions })
                }
              />
              {room.catanOptions?.helpers && (
                <p className="muted small">{catanTwoHelpersNote}</p>
              )}
            </>
          )}
        {room.kind === "catan" &&
          (isPublicCatanExplorer(room.catanScenario) ||
            (supportsCatanTradersHelpers(room.catanScenario) &&
              !["rivers", "caravans"].includes(room.catanScenario || ""))) && (
            <CatanOptionPicker
              explorer={isPublicCatanExplorer(room.catanScenario)}
              fiveSixAvailable={false}
              value={room.catanOptions}
              traders={supportsCatanTradersHelpers(
                room.catanScenario || room.catanTwoScenario,
              )}
              disabled={!host || busy || mapDirty}
              onChange={(catanOptions) =>
                command("catan_options", { catanOptions })
              }
            />
          )}
        {room.kind === "catan" &&
          !room.catanTwoRules &&
          (!room.catanFishing ||
            supportsPublicCatanFishingSeaExtended(room.catanScenario)) &&
          (!room.catanScenario ||
            ["cities-knights", "rivers", "caravans", "fishing"].includes(
              room.catanScenario || "",
            ) ||
            isPublicCatanSea(room.catanScenario)) && (
            <CatanOptionPicker
              value={room.catanOptions}
              traders={supportsCatanTradersHelpers(
                room.catanScenario || room.catanTwoScenario,
              )}
              helpersAvailable
              fishing={!!room.catanFishing || room.catanScenario === "fishing"}
              fiveSixAvailable={
                !room.catanFishing ||
                supportsPublicCatanFishingSeaExtended(room.catanScenario)
              }
              seafarers={!!room.catanSeafarers || !!room.catanNewWorldMap}
              citiesKnights={!!room.catanCitiesKnights}
              harbors={!!room.catanHarbors?.enabled}
              friendlyRobber={!!room.catanFriendlyRobber?.enabled}
              disabled={!host || busy || mapDirty}
              onChange={(catanOptions) =>
                command("catan_options", { catanOptions })
              }
            />
          )}
        {room.kind === "splendor" && (
          <SplendorOptionPicker
            value={room.splendorOptions}
            disabled={!host || busy}
            onChange={(splendorOptions) =>
              command("splendor_options", { splendorOptions })
            }
          />
        )}
        {room.kind === "sanguosha" && (
          <>
            <SanguoshaOptions
              value={room.sanguoshaOptions}
              disabled={!host || busy}
              onChange={(sanguoshaOptions) =>
                command("sanguosha_options", { sanguoshaOptions })
              }
            />
            {host && (
              <p className="muted small">更换规则后，所有玩家需要重新准备。</p>
            )}
          </>
        )}
        {room.kind === "rail" && (
          <>
            <RailMapPicker
              maps={maps}
              assets={assets}
              value={room.railMap || "usa"}
              disabled={!host || busy}
              onChange={(railMap) => command("rail_map", { railMap })}
            />
            {host && (
              <p className="muted small">更换地图后，所有玩家需要重新准备。</p>
            )}
          </>
        )}
        <div className="seats-grid">
          {Array.from({ length: room.capacity }, (_, i) => {
            const p = room.seats[i];
            return (
              <div key={i} className={`waiting-seat ${p ? "filled" : ""}`}>
                <span
                  className="avatar"
                  style={{ background: p ? playerColors[i] : undefined }}
                >
                  {p ? p.name[0] : <Plus size={22} />}
                </span>
                <h3>
                  {p ? <PlayerName user={p} /> : "虚位以待"}
                  {p?.id === room.host && <Crown size={14} />}
                  {p?.bot && <small className="bot-badge">AI</small>}
                </h3>
                <span>
                  {p ? (p.ready ? "✓ 已准备" : "等待准备") : "邀请一位朋友"}
                </span>
                {host && p?.bot && (
                  <button
                    className="bot-remove"
                    disabled={busy}
                    aria-label={`移除${p.name}`}
                    onClick={() => command("remove_bot", { target: p.id })}
                  >
                    移除
                  </button>
                )}
              </div>
            );
          })}
        </div>
        {!room.spectating && (
          <>
            <div className="waiting-actions">
              {host && room.seats.length < room.capacity && (
                <button
                  className="outline"
                  disabled={busy}
                  onClick={() => command("add_bot")}
                >
                  添加电脑玩家
                </button>
              )}
              <button
                disabled={busy || mapDirty}
                className={room.seats[room.you]?.ready ? "outline" : "primary"}
                onClick={() => command("ready")}
              >
                {room.seats[room.you]?.ready ? "取消准备" : "我准备好了"}
                <Check size={18} />
              </button>
              {host && (
                <button
                  className="primary gold"
                  disabled={busy || !ready || mapDirty}
                  onClick={() => command("start")}
                >
                  开始游戏
                  <ArrowRight size={18} />
                </button>
              )}
            </div>
          </>
        )}
        <p className="muted small">
          {room.spectating
            ? "等待玩家准备下一局。"
            : host
              ? "电脑自动准备。你准备好后即可开始，也可以继续邀请朋友。"
              : "准备好后，等待房主开始游戏。"}
        </p>
      </div>
    </div>
  );
}
export function LegacyPlayers({ room }: { room: Room }) {
  const assets = useContext(AssetsContext);
  const g = room.game!;
  const [nobleOwner, setNobleOwner] = useState<number>();
  useEffect(() => setNobleOwner(undefined), [room.id]);
  return (
    <div className="players-strip">
      {room.seats.map((p, i) => {
        const stats =
          g.splendor?.players[i] ||
          g.rail?.players[i] ||
          g.catan?.players[i] ||
          g.carcassonne?.players[i];
        return (
          <div
            data-player-seat={i}
            className={`player-panel ${g.splendor ? "splendor-player" : ""} ${i === (twoResponder(room) ?? caravanResponder(room) ?? fishResponder(room) ?? g.catan?.cardEvent?.players[0] ?? g.catan?.citiesKnights?.pending?.players[0] ?? g.catan?.seafarers?.pirateIslands?.raid?.rewards[0] ?? g.catan?.seafarers?.tribe?.pending?.player ?? g.catan?.helperPending?.player ?? g.catan?.goldPending?.claims[0]?.player ?? g.turn) && !g.finished ? "current" : ""} ${i === room.you ? "self" : ""} ${stats?.eliminated ? "eliminated" : ""}`}
            key={p.id}
          >
            <span
              className="avatar"
              style={{
                background: g.catan
                  ? catanSeatColor(g.catan, i)
                  : playerColors[i],
                color:
                  g.catan && catanColorIndex(g.catan, i) === 2
                    ? "#564432"
                    : undefined,
              }}
            >
              {p.name[0]}
            </span>
            <div className="player-info">
              <strong>
                <PlayerName user={p} />
                {p.left && (
                  <small>{stats?.eliminated ? "超时离场" : "已离桌"}</small>
                )}
                {i === room.you && <small>你</small>}
                {g.splendor && i === (g.splendor.startPlayer ?? 0) && (
                  <span
                    className="first-player-badge"
                    title="本局先手 · 最后一轮以此座位为起点，所有玩家行动次数相同"
                  >
                    <Flag size={11} aria-hidden="true" />
                    先手
                  </span>
                )}
                {p.bot && <small className="bot-badge">AI</small>}
                {p.autoPlay && (
                  <span className="autoplay-badge" title="由电脑代为行动">
                    <Bot size={12} aria-hidden="true" />
                    托管
                  </span>
                )}
              </strong>
              {g.splendor ? (
                <div
                  className="player-gem-resources"
                  aria-label={`${p.name}的宝石：上方永久折扣，下方现有筹码`}
                >
                  {gemDisplayOrder.map((c) => {
                    const { bonus, tokens } = g.splendor!.players[i];
                    const label =
                      c === 5
                        ? `黄金筹码 ${tokens[c]}`
                        : `${gemNames[c]}：永久折扣 ${bonus[c]}，现有筹码 ${tokens[c]}`;
                    return (
                      <span
                        key={c}
                        className={`player-gem-stack gem-color-${c} ${c === 5 ? "gold-only" : ""}`}
                        style={{ "--gem": gemColors[c] } as React.CSSProperties}
                        role="img"
                        aria-label={label}
                        title={label}
                      >
                        {c !== 5 && (
                          <span className="player-gem-card" aria-hidden="true">
                            <Gemstone color={c} size={13} />
                            <b>{bonus[c]}</b>
                          </span>
                        )}
                        <span className="player-gem-token" aria-hidden="true">
                          <b>{tokens[c]}</b>
                        </span>
                        {c === 5 && (
                          <span
                            className="player-gold-label"
                            aria-hidden="true"
                          >
                            黄金
                          </span>
                        )}
                      </span>
                    );
                  })}
                </div>
              ) : g.carcassonne ? (
                <small>
                  可用随从 <b>{g.carcassonne.players[i].meeples} / 7</b>
                </small>
              ) : g.catan ? (
                <small>
                  {g.catan.citiesKnights ? "资源与商品" : "资源"}{" "}
                  {g.catan.players[i].resourceCount} ·{" "}
                  {g.catan.explorer
                    ? "金币"
                    : g.catan.citiesKnights
                      ? "进步牌"
                      : "发展卡"}{" "}
                  {g.catan.explorer?.economy.gold[i] ??
                    g.catan.citiesKnights?.players[i].progressCount ??
                    g.catan.players[i].devCount}
                  <br />
                  {g.catan.explorer ? (
                    <>
                      船只{" "}
                      {
                        g.catan.explorer.fleet.positions.filter(
                          (at, slot) => Math.floor(slot / 3) === i && at >= 0,
                        ).length
                      }{" "}
                      / 3 · 港口{" "}
                      {
                        g.catan.vertices.filter(
                          (v) =>
                            v.owner === i && explorerIsHarbor(g.catan!, v.id),
                        ).length
                      }
                      {g.catan.citiesKnights && (
                        <CatanCitySeat game={g.catan} seat={i} />
                      )}
                    </>
                  ) : g.catan.citiesKnights ? (
                    <CatanCitySeat game={g.catan} seat={i} />
                  ) : g.catan.seafarers?.pirateIslands ? (
                    <>
                      战舰{" "}
                      {
                        g.catan.edges.filter(
                          (e) => e.owner === i && e.ship && e.warship,
                        ).length
                      }{" "}
                      ·{" "}
                      {g.catan.seafarers.pirateIslands.fortresses[i]
                        .strength === 0
                        ? "✓ 要塞已收复"
                        : `要塞 ${g.catan.seafarers.pirateIslands.fortresses[i].strength}/3`}
                    </>
                  ) : (
                    <>
                      {g.catan.seafarers ? "路线" : "道路"}{" "}
                      {g.catan.players[i].roadLength}
                      {!g.catan.attack && (
                        <> · 骑士 {g.catan.players[i].knights}</>
                      )}
                    </>
                  )}
                  {g.catan.two && g.catan.two.tokenRule !== "none" && (
                    <>
                      {" "}
                      · 筹码 <b>{g.catan.two.tokens[i]}</b>
                    </>
                  )}
                  {g.catan.friendlyRobber?.protectedPlayers.includes(i) && (
                    <>
                      <br />
                      <CatanFriendlyRobberSeat game={g.catan} seat={i} />
                    </>
                  )}
                  {g.catan.transport && (
                    <>
                      <br />
                      <CatanTransportSeat
                        game={g.catan}
                        seat={i}
                        assets={assets}
                      />
                    </>
                  )}
                  {g.catan.attack && (
                    <>
                      <br />
                      <CatanAttackSeat
                        game={g.catan}
                        seat={i}
                        assets={assets}
                      />
                    </>
                  )}
                  {g.catan.rivers && (
                    <>
                      <br />
                      <CatanRiverSeat game={g.catan} seat={i} assets={assets} />
                    </>
                  )}
                  {g.catan.harbors && (
                    <>
                      <br />
                      <CatanHarborsSeat game={g.catan} seat={i} />
                    </>
                  )}
                  {g.catan.seafarers?.cloth && (
                    <>
                      <br />
                      <span className="catan-cloth-seat">
                        <ClothPicture assets={assets} />
                        布匹 {g.catan.seafarers.cloth.held[i]} ·{" "}
                        {Math.floor(g.catan.seafarers.cloth.held[i] / 2)} 分
                      </span>
                    </>
                  )}
                  {g.catan.seafarers?.wonders && (
                    <>
                      <br />
                      <CatanWonderSeat game={g.catan} seat={i} />
                    </>
                  )}
                  {g.catan.seafarers?.tribe && (
                    <>
                      <br />
                      <span className="catan-tribe-seat-ports">
                        部落奖励 {g.catan.seafarers.tribe.points[i]} 分
                      </span>
                      {!!g.catan.seafarers.tribe.heldPorts[i]?.length && (
                        <span className="catan-tribe-seat-ports">
                          {" "}
                          · 暂存港口{" "}
                          {g.catan.seafarers.tribe.heldPorts[i]!.length}
                        </span>
                      )}
                    </>
                  )}
                  {g.catan.paired &&
                    (i === g.catan.paired.primary ||
                      i === g.catan.paired.secondary) && (
                      <>
                        <br />
                        <b>
                          {i === g.catan.paired.primary
                            ? "① 正常回合"
                            : "② 配对行动"}
                        </b>
                      </>
                    )}
                  {g.catan.players[i].helper && (
                    <>
                      <br />
                      助手{" "}
                      {
                        g.catan.helperRules?.find(
                          (h) => h.id === g.catan!.players[i].helper!.id,
                        )?.name
                      }{" "}
                      {g.catan.players[i].helper?.moon ? "☾" : "☀"}
                    </>
                  )}
                </small>
              ) : (
                <small>
                  车厢 {g.rail!.players[i].trains} · 手牌{" "}
                  {g.rail!.players[i].handCount} · 任务{" "}
                  {g.rail!.players[i].ticketCount}
                  {!!g.rail!.mapInfo.stations &&
                    ` · 车站 ${g.rail!.players[i].stations?.length || 0}/${g.rail!.mapInfo.stations}`}
                </small>
              )}
              {g.splendor && (
                <div className="player-gem-summary">
                  <div className="token-summary">
                    <span
                      className="token-total"
                      title="当前持有的宝石总数，包含黄金；回合结束时最多保留 10 枚"
                    >
                      宝石{" "}
                      {g.splendor.players[i].tokens.reduce(
                        (sum, n) => sum + n,
                        0,
                      )}{" "}
                      / 10
                    </span>
                    <span className="reserved-count">
                      预留{" "}
                      {g.splendor.players[i].reserved?.length ??
                        g.splendor.players[i].reservedCount}
                      /3
                    </span>
                  </div>
                </div>
              )}
              {g.splendor?.options?.cities && (
                <div
                  className="player-gem-cities"
                  aria-label={`${p.name}已达成的城市`}
                >
                  {!!g.splendor.cityEligibility?.[i]?.length && (
                    <Check size={13} />
                  )}
                  {g.splendor.cityEligibility?.[i]?.length
                    ? g.splendor.cityEligibility[i]
                        .map((index) => g.splendor!.cities?.[index]?.name)
                        .join("、")
                    : "尚未达成城市"}
                </div>
              )}
              {g.splendor && !g.splendor.options?.cities && (
                <div
                  className="player-nobles"
                  aria-label={`${p.name}已获得的贵族`}
                >
                  <span className="player-nobles-label">
                    <Crown size={13} />
                    贵族 {g.splendor.players[i].nobles.length}
                  </span>
                  {g.splendor.players[i].nobles.length ? (
                    <div className="player-noble-portraits">
                      {g.splendor.players[i].nobles.map((n) => (
                        <button
                          key={n.id}
                          className="noble owned-noble-thumbnail"
                          style={nobleArtwork(n.id, assets)}
                          aria-label={`查看${p.name}的${extraNobleName(n.id) || `贵族 ${n.id}`}，3 分`}
                          aria-haspopup="dialog"
                          onClick={() => setNobleOwner(i)}
                        >
                          <Crown size={18} />
                          <span className="sr-only">
                            {extraNobleName(n.id) || `贵族 ${n.id}`}
                          </span>
                        </button>
                      ))}
                    </div>
                  ) : (
                    <span className="player-nobles-empty">暂无</span>
                  )}
                </div>
              )}
              {!!g.splendor?.players[i].tradingPosts?.length && (
                <div
                  className="player-gem-posts"
                  aria-label={`${p.name}的贸易站`}
                >
                  {g.splendor.players[i].tradingPosts!.map((id) => (
                    <span
                      key={id}
                      title={
                        g.splendor!.tradingPostRules?.find((r) => r.id === id)
                          ?.description
                      }
                    >
                      {gemPostNames[id]}
                    </span>
                  ))}
                </div>
              )}
            </div>
            <div className="player-score">
              {stats?.score}
              <small>分</small>
            </div>
            {i === g.turn && !g.finished && <span className="turn-dot" />}
          </div>
        );
      })}
      {nobleOwner !== undefined && g.splendor && room.seats[nobleOwner] && (
        <Modal
          title={`${room.seats[nobleOwner].name}已获得的贵族`}
          onClose={() => setNobleOwner(undefined)}
        >
          <p>每位贵族提供 3 分，已计入该玩家总分。</p>
          <div className="owned-nobles-detail">
            {g.splendor.players[nobleOwner].nobles.map((n) => (
              <div key={n.id}>
                <NobleCard noble={n} eligible={false} onClick={() => {}} />
                <strong>{extraNobleName(n.id) || `贵族 ${n.id}`} · 3 分</strong>
              </div>
            ))}
          </div>
        </Modal>
      )}
    </div>
  );
}
export function LegacyTurn({
  room,
  serverNow,
  receivedAt,
}: {
  room: Room;
  serverNow: number;
  receivedAt: number;
}) {
  const g = room.game!;
  const setup = !!g.rail?.setup;
  const autoPlay = !!room.seats[room.you]?.autoPlay;
  const sgActor =
    g.sanguosha?.pending?.player ??
    twoResponder(room) ??
    caravanResponder(room) ??
    fishResponder(room) ??
    g.catan?.cardEvent?.players[0] ??
    g.catan?.citiesKnights?.pending?.players[0] ??
    g.catan?.seafarers?.pirateIslands?.raid?.rewards[0] ??
    g.catan?.seafarers?.tribe?.pending?.player ??
    g.catan?.helperPending?.player ??
    g.catan?.goldPending?.claims[0]?.player ??
    g.turn;
  const mine =
    !room.spectating &&
    (g.dota
      ? !!g.dota.actors?.includes(room.you)
      : g.sanguosha
        ? g.sanguosha.pending
          ? g.sanguosha.pending.canRespond
          : sgActor === room.you
        : twoResponder(room) !== undefined
          ? twoResponder(room) === room.you
          : caravanResponder(room) !== undefined
            ? caravanResponder(room) === room.you
            : fishResponder(room) !== undefined
              ? fishResponder(room) === room.you
              : g.catan?.cardEvent
                ? g.catan.cardEvent.players[0] === room.you
                : g.catan?.citiesKnights?.pending
                  ? g.catan.citiesKnights.pending.players[0] === room.you
                  : g.catan?.seafarers?.pirateIslands?.raid
                    ? g.catan.seafarers.pirateIslands.raid.rewards[0] ===
                      room.you
                    : g.catan?.seafarers?.tribe?.pending
                      ? g.catan.seafarers.tribe.pending.player === room.you
                      : g.catan?.helperPending
                        ? g.catan.helperPending.player === room.you
                        : g.catan?.goldPending
                          ? g.catan.goldPending.claims[0]?.player === room.you
                          : g.phase === "catan_discard"
                            ? (g.catan?.discardDue[room.you] || 0) > 0
                            : setup
                              ? !g.rail?.setupReady?.[room.you]
                              : g.turn === room.you);
  const [tick, setTick] = useState(performance.now());
  const deadline = room.status === "playing" ? room.turnDeadline : 0;
  useEffect(() => {
    if (!deadline) return;
    const timer = setInterval(() => setTick(performance.now()), 250);
    return () => clearInterval(timer);
  }, [deadline]);
  const remaining = deadline
    ? Math.max(
        0,
        Math.min(
          120,
          Math.ceil(
            (deadline - serverNow - Math.max(0, tick - receivedAt)) / 1000,
          ),
        ),
      )
    : 0;
  const expired = !!deadline && remaining === 0;
  const phase: Record<string, string> = {
    ...catanPhases,
    ...(g.catan?.explorer ? { catan_turn: "交易建设，然后开始航行" } : {}),
    ...(g.catan?.seafarers ? catanSeafarerPhases : {}),
    ...(g.catan?.eventDeck ? { catan_roll: "翻开事件牌，再生产资源" } : {}),
    ...(g.catan?.seafarers?.wonders
      ? { catan_robber: catanPhases.catan_robber }
      : {}),
    dota_teams: "确认队伍",
    dota_draft: "选择英雄",
    dota_plan: "暗选行动，全部锁定后揭牌",
    sg_select: "选择武将",
    sg_response: "等待响应",
    sg_play: "选择手牌、技能与目标",
    car_tile: "放置地块",
    car_meeple: "派遣随从",
    turn: "选择一个行动",
    discard: "归还多出的宝石",
    noble: "选择一位贵族",
    gem_post: "选择一个贸易站",
    gem_token: "领取贸易站宝石",
    gem_reserve: "选择一张盲预留卡",
    gem_stronghold: "放置、移动或移除要塞",
    gem_conquest: "选择是否发动征服",
    gem_copy: "选择复制的永久奖励",
    gem_free_card: "免费取得一张发展卡",
    tickets: "选择目的地任务",
    draw: "摸取第二张列车牌",
    finished: "查看本局结算",
  };
  return (
    <div className={`turn-card ${mine ? "mine" : ""}`}>
      <span className="eyebrow">ROUND {String(g.round).padStart(2, "0")}</span>
      <h3>
        {g.finished
          ? "本局已结束"
          : mine && autoPlay
            ? "电脑正在代你行动"
            : g.dota
              ? mine
                ? phase[g.phase]
                : "等待同伴锁定"
              : setup
                ? mine
                  ? "一起选择目的地"
                  : "等待其他人选好"
                : mine
                  ? "轮到你了"
                  : g.sanguosha?.pending?.kind === "nullification"
                    ? "等待无懈响应"
                    : `${room.seats[sgActor]?.name} ${twoResponder(room) !== undefined || caravanResponder(room) !== undefined || fishResponder(room) !== undefined || g.sanguosha?.pending || g.catan?.attack?.pending || g.catan?.attack?.endPlan || g.catan?.cardEvent || g.catan?.citiesKnights?.pending || g.catan?.seafarers?.pirateIslands?.raid || g.catan?.seafarers?.tribe?.pending || g.catan?.helperPending || g.catan?.goldPending ? "正在响应" : "的回合"}`}
      </h3>
      <p>
        {g.finished
          ? "感谢同桌，好局下次再来。"
          : mine && autoPlay
            ? "可随时取消托管，恢复手动操作。"
            : g.dota
              ? "超时自动托管，回来可随时接管。"
              : setup
                ? "所有人同时选牌，超时由电脑选牌并开启托管。"
                : mine
                  ? phase[g.phase]
                  : "稍等片刻，想想下一步。"}
      </p>
      {!!deadline && (
        <div className={`turn-clock ${remaining <= 20 ? "urgent" : ""}`}>
          <div className="clock-reading">
            <Clock size={20} />
            <time aria-label={`本回合剩余 ${remaining} 秒`}>
              {String(Math.floor(remaining / 60)).padStart(2, "0")}:
              {String(remaining % 60).padStart(2, "0")}
            </time>
            <span>
              {expired
                ? "电脑正在接管"
                : g.catan?.attack?.pending || g.catan?.attack?.endPlan
                  ? "蛮族进攻响应 120 秒"
                  : g.phase === "catan_world_fish"
                    ? "渔场安放 120 秒"
                    : g.phase === "catan_world_ports"
                      ? "港口安放 120 秒"
                      : setup
                        ? "共同选牌限时"
                        : g.sanguosha?.pending?.kind.startsWith("sg_3v3")
                          ? "选择限时 120 秒"
                          : g.sanguosha?.pending && !g.sanguosha.selecting
                            ? "响应限时 20 秒"
                            : twoResponder(room) !== undefined
                              ? "双人选择 120 秒"
                              : caravanResponder(room) !== undefined
                                ? "商队响应 120 秒"
                                : fishResponder(room) !== undefined
                                  ? "捕鱼换筹码 120 秒"
                                  : g.catan?.cardEvent
                                    ? "事件牌响应 120 秒"
                                    : g.catan?.citiesKnights?.pending
                                      ? "城市与骑士响应 120 秒"
                                      : g.catan?.seafarers?.pirateIslands?.raid
                                        ? "防守奖励 120 秒"
                                        : g.catan?.seafarers?.tribe?.pending
                                          ? "港口安放 120 秒"
                                          : g.catan?.helperPending
                                            ? "助手选择 120 秒"
                                            : g.catan?.goldPending
                                              ? "金矿选择 120 秒"
                                              : "每回合 120 秒"}
            </span>
          </div>
          {expired && !g.finished && (
            <p>超时自动开启托管，玩家回来后可随时取消。</p>
          )}
        </div>
      )}
      {(g.splendor?.lastRound || (g.rail?.lastRemaining ?? -1) >= 0) &&
        !g.finished && (
          <div className="final-round">
            <Flag size={15} />
            最后一轮{g.rail ? ` · 剩余 ${g.rail.lastRemaining} 回合` : ""}
          </div>
        )}
      <div className="save-state">
        <Check size={13} />
        进度已自动保存
      </div>
    </div>
  );
}
export { Results } from "../ui/results";
export function LegacyRules({
  kind,
  railMap,
  sgOptions,
  room,
}: {
  kind: string;
  railMap?: RailMapSpec;
  sgOptions?: SGOptions;
  room: Room;
}) {
  return (
    <div className="rules">
      {kind === "dota" ? (
        <DotaRules />
      ) : kind === "sanguosha" ? (
        <SanguoshaRules options={sgOptions} />
      ) : kind === "carcassonne" ? (
        <>
          <p>
            卡卡颂基础版，2–5 人，72 张地块，每人 7
            名随从，包含农民。不含河流、修道院长等扩展。
          </p>
          <ol>
            <li>
              抽到地块后旋转并放在相邻空位，所有接边的道路、城市和田地必须匹配。完全无法放置的地块移出本局并重抽。
            </li>
            <li>
              可以在刚放的地块上派遣一名随从，也可以不派遣。相连的整个区域必须没有任何人的随从。
            </li>
            <li>
              完整道路每块 1 分；完整城市每块及每枚盾徽各 2
              分；修道院自身和八个邻格各 1 分。完成后随从回到手中。
            </li>
            <li>
              终局未完成道路每块 1 分，未完成城市每块及每枚盾徽各 1
              分，修道院按已占据邻格计分。
            </li>
            <li>
              农民留在田地直到终局，每座相邻完整城市 3
              分。同一田地的同一城市只计一次；道路和城墙会分隔田地。
            </li>
            <li>
              相连区域随从最多的玩家得分，并列各得全部分数。总分最高者获胜，同分共同获胜。
            </li>
          </ol>
          <p>
            每回合 120
            秒，放地块与派随从共用倒计时。超时自动开启电脑托管，可随时取消并继续手动操作。
          </p>
        </>
      ) : kind === "catan" ? (
        <CatanRules room={room} />
      ) : kind === "splendor" ? (
        <SplendorRules room={room} />
      ) : (
        <>{railMap && <RailMapRules map={railMap} />}</>
      )}
      {kind !== "sanguosha" && kind !== "catan" && (
        <p>
          每回合120秒，弃牌、贵族选择和第二次摸牌共用本回合计时。超时自动开启电脑托管，玩家保留席位，可随时取消托管。房主可直接结束牌桌。
        </p>
      )}
      <p className="muted small">
        操作由服务器验证。每次行动自动保存，刷新或重新登录后回到原来的座位。
      </p>
    </div>
  );
}
const Cover = LegacyCover;
