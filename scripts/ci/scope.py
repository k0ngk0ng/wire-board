
"""Changed-game CI planning and test ownership; no cross-game rule execution."""
import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
POKER = ('doudizhu', 'guandan', 'shengji')
LEGACY = ('splendor', 'rail', 'catan', 'carcassonne', 'sanguosha')

CATAN_EXTENDED_TESTS = (
    ('./games/catan/server', 'TestCatanAttackCityNaturalEngine'),
    ('./games/catan/server', 'TestCatanAttackDesertExtendedNatural'),
    ('./games/catan/server', 'TestCatanAttackDesertFourNatural'),
    ('./games/catan/server', 'TestCatanAttackPiratesNatural'),
    ('./games/catan/server', 'TestCatanAttackShoresExtendedNatural'),
    ('./games/catan/server', 'TestCatanAttackTransportKnightsNaturalEngine'),
    ('./games/catan/server', 'TestCatanAttackTransportNaturalEngine'),
    ('./games/catan/server', 'TestCatanAttackTribeNatural'),
    ('./games/catan/server', 'TestCatanAttackWondersNatural'),
    ('./games/catan/server', 'TestCatanCaravansAttackNaturalEngine'),
    ('./games/catan/server', 'TestCatanCaravansDesertSeaNatural'),
    ('./games/catan/server', 'TestCatanCaravansHelpersNatural'),
    ('./games/catan/server', 'TestCatanCaravansIslandsNatural'),
    ('./games/catan/server', 'TestCatanCaravansKnightsNaturalGames'),
    ('./games/catan/server', 'TestCatanCaravansSeaKnightsNatural'),
    ('./games/catan/server', 'TestCatanCaravansShoresExtendedNatural'),
    ('./games/catan/server', 'TestCatanCaravansShoresFourNatural'),
    ('./games/catan/server', 'TestCatanCaravansTransportNaturalEngine'),
    ('./games/catan/server', 'TestCatanCaravansTribeNatural'),
    ('./games/catan/server', 'TestCatanCaravansVariantsNatural'),
    ('./games/catan/server', 'TestCatanCaravansWorldNatural'),
    ('./games/catan/server', 'TestCatanCitiesKnightsBotsComplete'),
    ('./games/catan/server', 'TestCatanCitiesKnightsClothBotsComplete'),
    ('./games/catan/server', 'TestCatanCitiesKnightsSeafarersBotsComplete'),
    ('./games/catan/server', 'TestCatanCitiesKnightsWondersBotsComplete'),
    ('./games/catan/server', 'TestCatanExplorerCityBotNaturalTurnRotations'),
    ('./games/catan/server', 'TestCatanExplorerFishingNaturalMatches'),
    ('./games/catan/server', 'TestCatanExplorerFishingStartsAndNaturalTurns'),
    ('./games/catan/server', 'TestCatanExplorerFullNaturalBotMatches'),
    ('./games/catan/server', 'TestCatanExplorerHelpersNaturalMatches'),
    ('./games/catan/server', 'TestCatanExplorerIntroNaturalMatches'),
    ('./games/catan/server', 'TestCatanExplorerKnightsSetupCompletedLegalOpenings'),
    ('./games/catan/server', 'TestCatanExplorerLairsNaturalBotMatch'),
    ('./games/catan/server', 'TestCatanExplorerPairedNaturalMatches'),
    ('./games/catan/server', 'TestCatanExplorerSpiceNaturalBotMatches'),
    ('./games/catan/server', 'TestCatanFishingAttackKnightsNaturalEngine'),
    ('./games/catan/server', 'TestCatanFishingAttackNaturalEngine'),
    ('./games/catan/server', 'TestCatanFishingAttackSeaNatural'),
    ('./games/catan/server', 'TestCatanFishingCaravansNaturalEngine'),
    ('./games/catan/server', 'TestCatanFishingCaravansSeaNatural'),
    ('./games/catan/server', 'TestCatanFishingCitiesKnightsBotsFinishAndConserve'),
    ('./games/catan/server', 'TestCatanFishingClothBotsFinishAndConserve'),
    ('./games/catan/server', 'TestCatanFishingExtendedSeaNaturalGames'),
    ('./games/catan/server', 'TestCatanFishingFogBotsFinishAndConserve'),
    ('./games/catan/server', 'TestCatanFishingHelpersKnightsNaturalGames'),
    ('./games/catan/server', 'TestCatanFishingHelpersNaturalGames'),
    ('./games/catan/server', 'TestCatanFishingNewWorldBotsComplete'),
    ('./games/catan/server', 'TestCatanFishingRiversNaturalEngine'),
    ('./games/catan/server', 'TestCatanFishingRiversSeaNatural'),
    ('./games/catan/server', 'TestCatanFishingSeaKnightsNaturalGames'),
    ('./games/catan/server', 'TestCatanFishingSeafarersBotsFinishAndConserve'),
    ('./games/catan/server', 'TestCatanFishingShoresNaturalGames'),
    ('./games/catan/server', 'TestCatanFishingTransportNaturalEngine'),
    ('./games/catan/server', 'TestCatanFishingWondersBotsComplete'),
    ('./games/catan/server', 'TestCatanFriendlyRobberBotsComplete'),
    ('./games/catan/server', 'TestCatanFriendlySeafarersBotsComplete'),
    ('./games/catan/server', 'TestCatanFriendlySeafarersHarborsBotsComplete'),
    ('./games/catan/server', 'TestCatanHarborsBotsComplete'),
    ('./games/catan/server', 'TestCatanHelpersKnightsNaturalGames'),
    ('./games/catan/server', 'TestCatanNewWorldBotsComplete'),
    ('./games/catan/server', 'TestCatanNewWorldRearrangedCustomMapsComplete'),
    ('./games/catan/server', 'TestCatanPirateIslandsBotsComplete'),
    ('./games/catan/server', 'TestCatanRiversAttackKnightsNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversAttackNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversCaravansNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversDesertExtended'),
    ('./games/catan/server', 'TestCatanRiversDesertNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversFogExtendedNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversFogNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversKnightsNaturalGames'),
    ('./games/catan/server', 'TestCatanRiversSeaKnightsNatural'),
    ('./games/catan/server', 'TestCatanRiversSeaNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversShoresExtendedNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversTransportNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversTribeExtendedNatural'),
    ('./games/catan/server', 'TestCatanRiversTribeNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversWorldNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversWorldPreparedNaturalEngine'),
    ('./games/catan/server', 'TestCatanRiversWorldSixNaturalEngine'),
    ('./games/catan/server', 'TestCatanSeafarersEventDeckNaturalMatches'),
    ('./games/catan/server', 'TestCatanSeafarersFixedMapsBotsCompleteWithGoldAndHelpers'),
    ('./games/catan/server', 'TestCatanSeafarersHelperEventDeckNaturalMatches'),
    ('./games/catan/server', 'TestCatanTradersHelpersNatural'),
    ('./games/catan/server', 'TestCatanTradersVariantsNatural'),
    ('./games/catan/server', 'TestCatanTransportKnightsNaturalGames'),
    ('./games/catan/server', 'TestCatanTransportNaturalMatches'),
    ('./games/catan/server', 'TestCatanTransportSeaModulesNatural'),
    ('./games/catan/server', 'TestCatanTransportSeaNatural'),
    ('./games/catan/server', 'TestCatanTwoAttackCityNaturalEngine'),
    ('./games/catan/server', 'TestCatanTwoAttackDesertNatural'),
    ('./games/catan/server', 'TestCatanTwoAttackSeaNatural'),
    ('./games/catan/server', 'TestCatanTwoCaravansSeaNatural'),
    ('./games/catan/server', 'TestCatanTwoCaravansShoresLongGames'),
    ('./games/catan/server', 'TestCatanTwoFishingSeafarersNaturalGames'),
    ('./games/catan/server', 'TestCatanTwoHelpersKnightsNaturalGames'),
    ('./games/catan/server', 'TestCatanTwoRiversSeaNatural'),
    ('./games/catan/server', 'TestCatanTwoRiversWorldNatural'),
    ('./games/catan/server', 'TestCatanTwoSeafarersKnightsCompleteGames'),
    ('./games/catan/server', 'TestCatanTwoSeafarersNaturalGames'),
    ('./games/catan/server', 'TestCatanWondersBotsComplete'),
    ('./internal/server', 'TestCatanAttackDesertHTTP'),
    ('./internal/server', 'TestCatanAttackKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanAttackPiratesHTTP'),
    ('./internal/server', 'TestCatanAttackPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanAttackShoresExtendedHTTP'),
    ('./internal/server', 'TestCatanAttackShoresHTTP'),
    ('./internal/server', 'TestCatanAttackShoresThreeHTTP'),
    ('./internal/server', 'TestCatanAttackShoresTwoHTTP'),
    ('./internal/server', 'TestCatanAttackTransportNaturalHTTP'),
    ('./internal/server', 'TestCatanAttackTribeHTTP'),
    ('./internal/server', 'TestCatanAttackWondersHTTP'),
    ('./internal/server', 'TestCatanBaseFixedFullHTTPGames'),
    ('./internal/server', 'TestCatanCaravansAttackNaturalHTTP'),
    ('./internal/server', 'TestCatanCaravansCompleteHTTPGames'),
    ('./internal/server', 'TestCatanCaravansHelpersHTTP'),
    ('./internal/server', 'TestCatanCaravansIslandsHTTP'),
    ('./internal/server', 'TestCatanCaravansIslandsOrdinaryHTTP'),
    ('./internal/server', 'TestCatanCaravansKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanCaravansSeaEventsHTTP'),
    ('./internal/server', 'TestCatanCaravansSeaModulesNaturalHTTP'),
    ('./internal/server', 'TestCatanCaravansSeaPublicHTTP'),
    ('./internal/server', 'TestCatanCaravansShoresHTTP'),
    ('./internal/server', 'TestCatanCaravansTransportNaturalHTTP'),
    ('./internal/server', 'TestCatanCaravansTribeHTTP'),
    ('./internal/server', 'TestCatanCaravansVariantsHTTP'),
    ('./internal/server', 'TestCatanCaravansWorldHTTP'),
    ('./internal/server', 'TestCatanCitiesKnightsConfiguredFullHTTPGames'),
    ('./internal/server', 'TestCatanCitiesKnightsSeafarersConfiguredFullHTTPGames'),
    ('./internal/server', 'TestCatanEventFishingKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanEventFishingNaturalHTTP'),
    ('./internal/server', 'TestCatanEventFishingTribeNaturalHTTP'),
    ('./internal/server', 'TestCatanEventKnightsEndgameMapsFullHTTPGames'),
    ('./internal/server', 'TestCatanEventKnightsPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanEventVariantsKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanEventVariantsPublicNaturalHTTP'),
    ('./internal/server', 'TestCatanExplorerCityNaturalHTTPMatches'),
    ('./internal/server', 'TestCatanExplorerEventsNaturalHTTPGames'),
    ('./internal/server', 'TestCatanExplorerFishNaturalManualHTTPMatch'),
    ('./internal/server', 'TestCatanExplorerFishingNaturalHTTPGames'),
    ('./internal/server', 'TestCatanExplorerFullNaturalHTTPAutoplayMatch'),
    ('./internal/server', 'TestCatanExplorerFullNaturalManualHTTPMatch'),
    ('./internal/server', 'TestCatanExplorerHelpersNaturalHTTPGames'),
    ('./internal/server', 'TestCatanExplorerIntroNaturalHTTPGames'),
    ('./internal/server', 'TestCatanExplorerLairsNaturalHTTPAutoplayMatch'),
    ('./internal/server', 'TestCatanExplorerMissionsPublicCompleteHTTPGames'),
    ('./internal/server', 'TestCatanExplorerNaturalHTTPMatches'),
    ('./internal/server', 'TestCatanExplorerPairedHTTPNaturalMatches'),
    ('./internal/server', 'TestCatanExplorerSpiceNaturalHTTPAutoplayMatch'),
    ('./internal/server', 'TestCatanExplorerSpiceNaturalManualHTTPMatch'),
    ('./internal/server', 'TestCatanExplorerSpicePublicCompleteHTTPGames'),
    ('./internal/server', 'TestCatanExplorerTwoKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanFishingAttackNaturalHTTP'),
    ('./internal/server', 'TestCatanFishingAttackSeaNaturalHTTP'),
    ('./internal/server', 'TestCatanFishingCaravansNaturalHTTP'),
    ('./internal/server', 'TestCatanFishingCitiesKnightsPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingFogExtendedFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingFourMapsExtendedFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingHelpersKnightsNaturalHTTPGames'),
    ('./internal/server', 'TestCatanFishingHelpersNaturalHTTP'),
    ('./internal/server', 'TestCatanFishingKnightsVariantsPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingNewWorldExtendedFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingRiversNaturalHTTP'),
    ('./internal/server', 'TestCatanFishingRiversSeaNaturalHTTP'),
    ('./internal/server', 'TestCatanFishingSeaKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanFishingSeaPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingShoresEventsFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingShoresFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingTransportNaturalHTTP'),
    ('./internal/server', 'TestCatanFishingVariantsPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanFishingWondersExtendedFullHTTPGames'),
    ('./internal/server', 'TestCatanFriendlyKnightsPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanFriendlyPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanFriendlyRobberBaseFullHTTPGames'),
    ('./internal/server', 'TestCatanFriendlySeaConfiguredFullHTTPGames'),
    ('./internal/server', 'TestCatanHarborsBaseFullHTTPGames'),
    ('./internal/server', 'TestCatanHarborsCitiesKnightsFullHTTPGames'),
    ('./internal/server', 'TestCatanHarborsSeaKnightsPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanHarborsSeaPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanHelpersKnightsNaturalHTTPGames'),
    ('./internal/server', 'TestCatanHelpersVariantsPublicFullHTTPGames'),
    ('./internal/server', 'TestCatanNewWorldFullHTTPGames'),
    ('./internal/server', 'TestCatanPirateEventsFullHTTPGames'),
    ('./internal/server', 'TestCatanPirateFullHTTPGames'),
    ('./internal/server', 'TestCatanPirateKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanPublicEventsNaturalHTTPMatches'),
    ('./internal/server', 'TestCatanRiversAttackNaturalHTTP'),
    ('./internal/server', 'TestCatanRiversCaravansNaturalHTTP'),
    ('./internal/server', 'TestCatanRiversCompleteHTTPGames'),
    ('./internal/server', 'TestCatanRiversKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanRiversSeaKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanRiversSeaNaturalHTTP'),
    ('./internal/server', 'TestCatanRiversTransportNaturalHTTP'),
    ('./internal/server', 'TestCatanSeaNestingsE2E'),
    ('./internal/server', 'TestCatanSeafarersPublicCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTradersHelpersPublicHTTP'),
    ('./internal/server', 'TestCatanTradersVariantsPublicHTTP'),
    ('./internal/server', 'TestCatanTransportEventsNaturalHTTP'),
    ('./internal/server', 'TestCatanTransportKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanTransportNaturalHTTPMatches'),
    ('./internal/server', 'TestCatanTransportSeaHTTP'),
    ('./internal/server', 'TestCatanTribeKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanTwoAttackCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoCaravansCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoFishingCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoFishingEventsCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoFishingKnightsCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoFishingSeafarersCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoHelpersKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanTwoHelpersNaturalHTTP'),
    ('./internal/server', 'TestCatanTwoKnightsCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoKnightsEventsCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoRiversCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoSeaHelpersKnightsNaturalHTTP'),
    ('./internal/server', 'TestCatanTwoSeafarersCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoSeafarersKnightsCompleteHTTPGames'),
    ('./internal/server', 'TestCatanTwoVariantsCompleteHTTPGames'),
    ('./internal/server', 'TestCatanWondersFullHTTPGames'),
)

BASKETBALL_EXTENDED_TESTS = (
    ('./games/basketball/engine', 'TestRuntimePreparedConfigurationsCompleteMatchAndRestore'),
    ('./games/basketball/engine', 'TestRuntimeOriginalSelectionAdmissionMatrix'),
    ('./games/basketball/engine', 'TestRuntimeAllOriginalCharactersAdmission'),
)

STARFORT_EXTENDED_TESTS = (
    ('./games/starfort/tests', 'TestStarfortGeologyNaturalCampaigns'),
    ('./games/starfort/tests', 'TestStarfortLatestNatural'),
    ('./games/starfort/tests', 'TestStarfortRebalanceNaturalCampaign'),
    ('./games/starfort/tests', 'TestStarfortRebalanceNaturalModes'),
    ('./games/starfort/tests', 'TestStarfortRevisionNatural'),
    ('./games/starfort/tests', 'TestStarfortRulesNaturalCampaigns'),
    ('./games/starfort/tests', 'TestStarfortRulesNaturalEndless'),
    ('./games/starfort/tests', 'TestStarfortRulesNaturalLoss'),
    ('./games/starfort/tests', 'TestStarfortSpecializationNaturalCampaign'),
    ('./games/starfort/tests', 'TestStarfortWeatherNaturalCampaign'),
    ('./games/starfort/tests', 'TestStarfortWeatherNaturalModes'),
    ('./games/starfort/tests', 'TestStarfortWorldNaturalCampaigns'),
)

YUYU_EXTENDED_TESTS = (
    ('./games/yuyuhakusho/audio/score', 'TestObservedOriginalTrajectories'),
    ('./games/yuyuhakusho/audio/score', 'TestOriginalContinuousDriver'),
    ('./games/yuyuhakusho/audio/score', 'TestInstrumentWorkBoundCatalogOriginalTrajectories'),
    ('./games/yuyuhakusho/audio/scoreclock', 'TestNoteVoiceCompleteOriginalCallsAndActualOwners'),
    ('./games/yuyuhakusho/engine', 'TestWorldSoundLiveOriginalPasses'),
)

YUYU_EXTENDED_TRAJECTORIES = (
    'arena-2-duel',
    'arena-2-four',
    'arena-2-pan',
    'arena-2-team',
    'arena-2-three',
    'arena-4-duel',
    'arena-4-four',
    'arena-4-pan',
    'arena-4-team',
    'arena-4-three',
    'arena-6-duel',
    'arena-6-four',
    'arena-6-pan',
    'arena-6-team',
    'arena-6-three',
    'arena-8-duel',
    'arena-8-four',
    'arena-8-pan',
    'arena-8-team',
    'arena-8-three',
    'arena-a-duel',
    'arena-a-four',
    'arena-a-team',
    'arena-a-three',
    'arena-c-duel',
    'arena-c-four',
    'arena-c-pan',
    'arena-c-team',
    'arena-c-three',
    'arena-e-duel',
    'arena-e-pan',
    'arena-e-team',
    'arena-e-three',
    'legacy-four',
    'legacy-itsuki-elder',
    'legacy-itsuki-genkai',
    'legacy-itsuki-hiei',
    'legacy-itsuki-itsuki',
    'legacy-itsuki-jin',
    'legacy-itsuki-kurama',
    'legacy-itsuki-kuwabara',
    'legacy-itsuki-sensui',
    'legacy-itsuki-younger',
    'legacy-itsuki-yusuke',
)

GAME_EXTENDED_TESTS = {'basketball': BASKETBALL_EXTENDED_TESTS, 'catan': CATAN_EXTENDED_TESTS, 'starfort': STARFORT_EXTENDED_TESTS, 'yuyuhakusho': YUYU_EXTENDED_TESTS}
GAME_PARTITIONS = {'catan': (8, 20), 'yuyuhakusho': (4, 16)}

MULTI = {
    'TestBotsFinishBothGames': ('splendor', 'rail'),
    'TestSoloGamesAIContinuationPersistenceAndRematch': ('splendor', 'rail'),
    'TestBotFinalTurnFinishesRoom': ('splendor', 'rail'),
    'TestBaseGamesRemainBaseAfterExpansionSettings': ('splendor', 'catan'),
    'TestAutoPlayAllGamesFinishAndCreditHumanPlayers': LEGACY,
    'TestTimeoutAutoplayAllGamesContinueToNaturalFinish': LEGACY,
}
PAIRED = {'TestBaseData', 'TestBotsDoNotUseOtherPlayersSecretsOrDeckOrder', 'TestBotHandlesSubstepsAndInvalidSeats'}
WISH_PATHS = {'internal/server/wishes.go', 'internal/server/wish_test.go', 'web/src/wishes.tsx'}
FAST_COMPONENTS = {
    'wishes': (WISH_PATHS, ['internal/server/wish_test.go']),
    'profiles': ({'internal/server/profiles.go', 'internal/server/profiles_test.go'}, ['internal/server/profiles_test.go']),
    'chat': ({'internal/server/chat.go', 'internal/server/chat_test.go'}, ['internal/server/chat_test.go']),
    'network': ({'internal/server/network_status.go', 'internal/server/network_status_e2e_test.go'}, ['internal/server/network_status_e2e_test.go']),
    'game-order': ({'internal/server/game_order.go', 'internal/server/game_order_e2e_test.go', 'web/src/game-order.ts'}, ['internal/server/game_order_e2e_test.go']),
    'account-password': ({'internal/server/account_password.go', 'internal/server/account_password_e2e_test.go'}, ['internal/server/account_password_e2e_test.go']),
}

LINT_TOOL_FILES = {'games/racing/build/lint_go.go'}
ASSET_DELIVERY_FILES = {'scripts/cdn_delivery.py', 'scripts/test_cdn_delivery.py', 'scripts/resource_store.py', 'scripts/resource_store_e2e_test.py', 'scripts/asset_reuse.py', 'scripts/asset_reuse_e2e_test.py'}

def ci_path(path):
    return path in ASSET_DELIVERY_FILES or path.startswith(('.github/workflows/', '.github/actions/', 'scripts/ci/')) or path in {
        'scripts/release-source.cjs', 'scripts/release-source.test.cjs',
        'scripts/cdn_local_ready.py', 'scripts/test_cdn_local_ready.py',
        'games/racing/build/asset-reuse-lint.json',
        'games/racing/build/lint_go.go',
        'games/racing/build/lint.mjs',
    }

def game_names(root):
    return sorted(p.name for p in (root/'games').iterdir() if p.is_dir() and not p.name.startswith(('.', '_')))

def cover_path(path, games):
    return any(path in (f'games/{game}/web/Cover.tsx', f'scripts/{game}/accept_cover.py', f'scripts/{game}/cover_harness.tsx') for game in games)

def plan(paths, games, manual='', legacy_sources=None, contra_campaigns=False, validation='auto', resume_games=()):
    if validation not in ('auto', 'quick', 'full'): raise ValueError('Unknown validation mode: ' + validation)
    if validation == 'full': manual = 'all'
    selected = {}
    game_paths = set()
    active_path = None


    if contra_campaigns and 'contra' not in games:
        raise ValueError('Contra research requested without Contra sources')
    def add(game, backend=False):
        if game in games:
            selected[game] = selected.get(game, False) or backend
            if active_path is not None: game_paths.add(active_path)
    for path in paths:
        active_path = path
        if path.endswith('.md') or path.startswith('docs/'): continue
        if ci_path(path) or cover_path(path, games): continue
        if path == 'cmd/verify-yuyu-arena-aux/main.go':
            add('yuyuhakusho', True)
            continue
        parts = path.split('/')
        if path.startswith('internal/') and path.endswith('_test.go'):
            source = (legacy_sources or {}).get(path)
            if source is None: source = (ROOT/path).read_text() if (ROOT/path).exists() else ''
            for name in re.findall(r'^func ((?:Test|Example|Fuzz)\w*)\(', source, re.M):
                for game in owner(Path(path), name, games):
                    add(game, True)
        if len(parts) > 2 and parts[0] == 'games':


            engine_data = parts[1] == 'contra' and parts[2] == 'engine'
            pal_rules = parts[1] == 'pal' and (parts[2] in ('native', 'engine', 'resources', 'build') or path in {
                'games/pal/assets/source-manifest.json', 'games/pal/assets/authority-release.json', 'games/pal/assets/server-release.json',
            })
            add(parts[1], (path.endswith('.go') and path not in LINT_TOOL_FILES) or parts[2] == 'server' or engine_data or pal_rules or '/tests/testdata/' in path)
            continue
        if path.startswith(('web/src/poker/', 'scripts/poker/', 'internal/gametest/poker_', 'internal/gametest/browser_e2e')):
            for game in POKER: add(game, path.endswith('.go'))
            continue
        if path.startswith(('scripts/', 'internal/')):
            for game in games:
                if re.search(r'(?:/|_)' + re.escape(game) + r'(?:/|_|\.)', path):
                    add(game, path.endswith('.go'))
        if path in ('internal/game/bot_test.go', 'internal/game/game_test.go'):
            for game in ('rail', 'splendor'): add(game, True)
    active_path = None
    if manual:
        if manual not in (*games, 'all', 'shared'): raise ValueError(f'Unknown game: {manual}')
        if manual != 'shared':
            for game in games if manual == 'all' else [manual]: add(game, True)
    for game in resume_games:
        if game not in games: raise ValueError('Unknown resumed game: ' + game)
        add(game, True)
    rows = [{'game': g, 'backend': selected[g]} for g in sorted(selected)]
    matrix = []
    for row in rows:
        count = 1
        matrix.extend({**row, 'shard': i, 'count': count} for i in range(count))
    campaigns = [{'mode': mode, 'players': players}
                 for mode in ('standard-terminal', 'assisted-clear') for players in (1, 2)] if contra_campaigns else []
    code_paths = {p for p in paths if not p.endswith('.md') and not p.startswith('docs/')}
    components = set()
    if any(path.startswith('cmd/') for path in paths): components.add('ci')
    shared = manual in ('shared', 'all')
    frontend_checks = manual == 'all'
    for path in sorted(code_paths - game_paths):
        matched = {name for name, (sources, _) in FAST_COMPONENTS.items() if path in sources}
        if matched:
            components.update(matched)
        elif ci_path(path):
            components.add('ci')
        elif path.startswith('cmd/'):
            components.add('ci')
            shared = True
        elif cover_path(path, games) or path.startswith(('web/src/', 'web/tests/')) or path == 'scripts/check-release-history.py':
            frontend_checks = True
        elif path in ('scripts/update.sh', 'scripts/test_update.py', 'compose.yaml', 'Dockerfile', '.env.example'):
            components.add('deployment')
        else:
            shared = True
    if validation == 'quick' and (shared or rows):
        raise ValueError('Quick validation supports isolated platform components; use auto for shared core or games')
    if shared:
        components.difference_update(FAST_COMPONENTS)
    if frontend_checks: components.add('frontend')
    if validation == 'quick' and any(p.startswith('web/') and not p.endswith('.md') for p in paths):
        components.add('typescript')
    build_required = validation != 'quick' and bool(manual or resume_games or any(not ci_path(p) or p in ASSET_DELIVERY_FILES for p in code_paths))
    return {'games': rows, 'matrix': {'include': matrix}, 'has_games': bool(rows),
            'has_shared': shared, 'has_quick': bool(components),
            'quick_components': sorted(components),
            'quick_go': bool(components.intersection(FAST_COMPONENTS) or 'ci' in components),
            'quick_node_deps': bool(components.intersection({'frontend', 'typescript'})),
            'frontend_checks': frontend_checks, 'build_required': build_required,
            'validation': validation,
            'contra_campaign_requested': bool(contra_campaigns),
            'contra_campaign_matrix': {'include': campaigns}, 'has_contra_campaigns': bool(campaigns)}

def changed_names(raw):
    return [p.decode('utf-8') for p in raw.split(b'\0') if p]

def resume_selection(proof, head, base, games):
    if not proof.get('eligible'): return []
    pending = proof.get('pendingGames', [])
    reused = [row['game'] for row in proof.get('reusedGames', [])]
    if proof.get('head') != head or proof.get('base') != base or len(set(pending + reused)) != len(pending + reused) or set(pending + reused) != set(games):
        raise ValueError('Resume evidence differs from the exact plan inputs')
    return pending

def owner(path, name, games):
    parts = path.parts
    if len(parts) == 1 or parts[:2] == ('internal', 'production'):
        return set()
    if parts[0] == 'games' and len(parts) > 2: return {parts[1]}
    if name in MULTI: return set(MULTI[name])
    if name in PAIRED: return {'rail', 'splendor'}
    named = {g for g in games if g in name.lower() or path.name.startswith(g + '_')}
    return named

def trajectory_cases(root, extended=False):
    rows = json.loads((root / 'games/yuyuhakusho/renderstate/testdata/bridge-reference.json').read_text())['rows']
    names = [row['configuration']['name'] for row in rows]
    if len(names) != len(set(names)): raise ValueError('Duplicate trajectory case')
    return names if extended else [name for name in names if name not in YUYU_EXTENDED_TRAJECTORIES]

def go_commands(root, game, index=0, count=1, race=False, extended=False):
    if count < 1 or not 0 <= index < count: raise ValueError('Invalid shard')
    games = game_names(root)
    if game not in (*games, 'shared'): raise ValueError(f'Unknown game: {game}')
    grouped, special, trajectories = {}, [], []
    trajectory_prefixes = {'TestRenderFrozenBridgeTrajectories': 'RENDERSTATE',
                           'TestSchema2FrozenWholeTrajectories': 'CLIENT_BRIDGE'}
    files = sorted([*root.glob('*_test.go'), *root.glob('internal/**/*_test.go'),
                    *(root/'games').glob('*_test.go')])
    if game != 'shared': files += sorted((root/'games'/game).rglob('*_test.go'))
    for file in files:
        relative = file.relative_to(root)
        if relative.parts[:2] == ('internal', 'gametest'): continue
        for name in re.findall(r'^func ((?:Test|Example|Fuzz)\w*)\(', file.read_text(), re.M):
            if name == 'TestBotStandardCampaignE2E' and relative.parts[:2] == ('games', 'contra'):
                continue
            owners = owner(relative, name, games)
            if (game == 'shared' and owners) or (game != 'shared' and game not in owners): continue
            package = './' + relative.parent.as_posix() if relative.parent.as_posix() != '.' else '.'
            if not extended and (package, name) in GAME_EXTENDED_TESTS.get(game, ()): continue
            if game == 'yuyuhakusho' and not race and name in trajectory_prefixes:
                trajectories.append(('./' + relative.parent.as_posix(), name, trajectory_prefixes[name]))
                continue
            if int.from_bytes(hashlib.sha256(name.encode()).digest()[:8], 'big') % count != index: continue
            if race and game != 'shared':
                selected_race = bool(re.search(r'Concurrent|AutoPlay|Timeout|Spectator|LongGameArtifact', name))
                if game == 'catan' and not extended:
                    selected_race = bool(re.search(r'Concurrent|Spectator', name))
                if game == 'basketball':
                    selected_race = selected_race or name.startswith(('TestBasketballRealtime', 'TestCampaignPresentation', 'TestCampaignEnding', 'TestRuntimePresentation', 'TestDunkSharedTarget', 'TestHighPassHeight')) or name == 'TestBasketballHTTPCompleteOriginalSelection'
                if game == 'contra':
                    selected_race = selected_race or name.startswith(('TestContraHTTP', 'TestContraModule', 'TestRuntimePaused', 'TestRuntimeBundle', 'TestRuntimeHTTP'))
                    area = relative.parts[2] if len(relative.parts) > 3 and relative.parts[:2] == ('games', 'contra') else ''
                    selected_race = selected_race or (
                        area == 'server' and name.startswith(('TestProduction', 'TestRuntimeArtifact')) or
                        area == 'engine' and name.startswith('TestSessionBattleReview') or
                        area == 'runtime' and name == 'TestRuntimeTerminalUsesPersistedBattleReview'
                    )
                if not selected_race: continue
            package = './' + relative.parent.as_posix() if relative.parent.as_posix() != '.' else '.'
            if name in MULTI:
                suffix = '-' if name == 'TestBotsFinishBothGames' else '$'
                special.append((package, '^' + name + '$/^' + game + suffix))
            else:
                grouped.setdefault(package, []).append(name)


    timeout = '20m' if game in GAME_EXTENDED_TESTS and not extended else '150m' if game in ('catan', 'yuyuhakusho') and count == 1 else '90m' if game == 'starfort' and count == 1 else '45m'
    base = ['go', 'test', '-json', '-count=1', '-timeout=' + timeout]
    if race: base.append('-race')
    commands = [base + ['-run', '^(?:' + '|'.join(re.escape(n) for n in sorted(set(names))) + ')$', package]
                for package, names in sorted(grouped.items())]
    commands += [base + ['-run', pattern, package] for package, pattern in special]



    for package, name, prefix in trajectories:
        pattern = '^' + name + '$'
        if not extended:
            chosen = trajectory_cases(root)
            if not chosen: raise ValueError('No required trajectory cases')
            pattern += '/^(?:' + '|'.join(re.escape(case) for case in chosen) + ')$'
        commands.append(['env', prefix + '_SHARDS=' + str(count), prefix + '_SHARD=' + str(index)] + base + ['-run', pattern, package])
    return commands

def quick_commands(root, components):
    commands = []
    grouped = {}
    for component in components:
        if component in FAST_COMPONENTS:
            for name in FAST_COMPONENTS[component][1]:
                path = root/name
                if not path.is_file(): raise ValueError('Missing fast-lane test file: ' + name)
                tests = re.findall(r'^func ((?:Test|Example|Fuzz)\w*)\(', path.read_text(), re.M)
                if not tests: raise ValueError('No fast-lane tests in: ' + name)
                grouped.setdefault('./' + path.parent.relative_to(root).as_posix(), set()).update(tests)
        elif component == 'ci':
            commands += [
                ['python3', '-B', 'scripts/ci/traffic_lint.py', '--self-test'],
                ['python3', '-B', 'scripts/ci/resources_e2e_test.py'],
                ['python3', '-B', 'scripts/ci/publication_traffic_e2e_test.py'],
                ['python3', '-B', 'scripts/ci/traffic_lint_e2e_test.py'],
                ['python3', '-B', 'scripts/ci/test_scope.py'],
                ['python3', '-B', 'scripts/ci/partition_e2e_test.py'],
                ['python3', '-B', 'scripts/ci/catan_suite_e2e_test.py'],
                ['python3', '-B', 'scripts/ci/game_suites_e2e_test.py'],
                ['python3', '-B', 'scripts/ci/resume_e2e_test.py'],
                ['python3', '-B', 'scripts/ci/game_tools.py'],
                ['python3', '-B', 'scripts/ci/game_tools_e2e_test.py'],
                ['python3', '-B', 'scripts/ci/check-plan.py'],
                ['python3', '-B', 'scripts/ci/check-lanes.py'],
                ['python3', '-B', 'scripts/ci/release-image-e2e.py'],
                ['python3', '-B', 'scripts/ci/pal-release-e2e.py'],
                ['node', '--test', 'scripts/ci/baseline.test.cjs', 'scripts/release-source.test.cjs'],
                ['python3', '-B', 'scripts/test_cdn_local_ready.py'],
                ['python3', '-B', 'scripts/test_cdn_delivery.py'],
                ['python3', '-B', 'scripts/ci/cdn_concurrency_e2e_test.py'],
            ]
        elif component == 'deployment':
            commands += [['bash', '-n', 'scripts/update.sh'], ['python3', '-B', 'scripts/test_update.py']]
        elif component == 'frontend':
            commands += [['node', '--experimental-strip-types', '--test', *[str(p.relative_to(root)) for p in sorted((root/'web/tests').glob('*.test.mjs'))]]]
        elif component == 'typescript':
            commands += [['web/node_modules/.bin/tsc', '-b', 'web/tsconfig.json', '--pretty', 'false']]
        else:
            raise ValueError('Unknown fast-lane component: ' + component)
    commands += [['go', 'test', '-json', '-count=1', '-timeout=5m', '-run',
                  '^(?:' + '|'.join(sorted(names)) + ')$', package]
                 for package, names in sorted(grouped.items())]
    return commands

def verification_env(game, backend, base=None, extended=False):
    explicit = {'WIRE_BOARD_NESTING_ACCEPTANCE', 'BASKETBALL_PREPARATION_MATRIX', 'BASKETBALL_SELECTION_ADMISSION', 'BASKETBALL_FULL_HTTP'}
    env = {k: v for k, v in (os.environ if base is None else base).items()
           if not k.startswith('CONTRA_BOT_') and k not in explicit}
    if backend and game == 'snooker': env['WIRE_BOARD_SNOOKER_HTTP_FULL'] = '1'
    if backend and game == 'catan' and extended: env['WIRE_BOARD_NESTING_ACCEPTANCE'] = '1'
    if backend and game == 'basketball':
        env['BASKETBALL_FULL_HTTP'] = '1'
        if extended: env.update(BASKETBALL_PREPARATION_MATRIX='1', BASKETBALL_SELECTION_ADMISSION='1')
    return env

def frontend_tests(root, game):
    base = root/'games'/game
    if game == 'pokepath':
        entry=base/'tests/ci.test.mjs'
        if not entry.is_file(): raise FileNotFoundError('Missing Pokepath clean delivery entry')
        return [entry]
    return sorted([*(base/'tests').glob('*.test.mjs'),
                   *(base/'web').glob('*.test.mjs')])

def prepare_pvz_asset_tools(root):
    output = root/'.local/pvz/ci-assets'
    output.mkdir(parents=True, exist_ok=True)
    result = output/'prepare-result.json'
    report = {'passed': False, 'python': sys.version.split()[0]}
    result.write_text(json.dumps(report, indent=2)+'\n')
    try:
        selected = shutil.which('ffmpeg')
        report['source'] = 'system'
        if not selected:
            from importlib.metadata import version
            import imageio_ffmpeg
            if version('imageio-ffmpeg') != '0.6.0':
                raise RuntimeError('PvZ fallback requires imageio-ffmpeg==0.6.0')
            selected = imageio_ffmpeg.get_ffmpeg_exe()
            package = Path(imageio_ffmpeg.__file__).resolve().parent
            if not Path(selected).resolve().is_relative_to(package) or not package.is_relative_to(root.resolve()):
                raise ValueError('PvZ fallback must use the workspace wheel executable')
            report['source'] = 'imageio-ffmpeg==0.6.0'
        executable = Path(selected).resolve(strict=True)
        if not executable.is_file() or not os.access(executable, os.X_OK):
            raise ValueError('PvZ FFmpeg is not executable')
        folder = output/'bin'
        folder.mkdir(exist_ok=True)
        bound = folder/'ffmpeg'
        if bound.is_symlink(): bound.unlink()
        elif bound.exists(): raise ValueError('Unexpected regular file at PvZ FFmpeg binding')
        bound.symlink_to(executable)
        tested = subprocess.run([str(bound), '-version'], capture_output=True, text=True, check=True)
        first = tested.stdout.splitlines()[0]
        if not first.startswith('ffmpeg version '):
            raise ValueError('PvZ FFmpeg version response is invalid')
        report.update(passed=True, ffmpegVersion=first, executable=str(executable),
                      executableSHA256=hashlib.sha256(executable.read_bytes()).hexdigest())
    except Exception as error:
        report['errorType'] = type(error).__name__
        result.write_text(json.dumps(report, indent=2)+'\n')
        raise
    result.write_text(json.dumps(report, indent=2)+'\n')
    return report

def build_commands(root, game, index=0):
    if game == 'yuyuhakusho' and index == 0:
        return [['node','--test',str(root/'games/yuyuhakusho/build/runtime-delivery.test.mjs')], ['go','test','-run','^$','./games/yuyuhakusho/cmd/verify-arena-aux']]
    if game == 'pvz' and index == 0:
        base, output = root/'games/pvz/build', root/'.local/pvz/ci-assets'
        return [
            ['python3', '-B', '-m', 'unittest', 'discover', '-s', str(base), '-p', 'test_*.py'],
            ['python3', '-B', str(base/'verify_native.py'), '--output', str(output/'native.json')],
            ['python3', '-B', str(base/'verify_music.py'), '--output', str(output/'music.json')],
        ]
    if game != 'contra' or index != 0:
        return []
    return ([['python3', '-B', str(root/'games/contra/build'/name)]
             for name in ('identity_test.py', 'media_test.py', 'media_archive_test.py', 'builder_inputs_test.py', 'runtime_assets_test.py', 'delivery_test.py', 'ci_runtime_test.py')]
            + [['node', '--test', str(root/'games/contra/build/runtime-delivery.test.mjs')]])

def extended_mode(args):
    return getattr(args, 'extended', False) or getattr(args, 'catan_extended', False)

def run_game_partitions(args):
    output = ROOT/'.local/ci'/args.game
    output.mkdir(parents=True, exist_ok=True)
    destination = output/'partitions.json'
    extended = extended_mode(args)
    count = GAME_PARTITIONS[args.game][1 if extended else 0]
    report = {'passed': False, 'suite': 'extended' if extended else 'standard', 'partitions': count, 'workers': 2, 'completed': [], 'failed': []}
    destination.write_text(json.dumps(report, indent=2)+'\n')
    try:
        with ThreadPoolExecutor(max_workers=2) as pool:
            futures = {pool.submit(run, argparse.Namespace(**{**vars(args), 'shard': index, 'count': count})): index for index in range(count)}
            try:
                for future in as_completed(futures):
                    index = futures[future]
                    try:
                        future.result()
                    except Exception:
                        report['failed'].append(index)
                        for pending in futures:
                            pending.cancel()
                        raise
                    report['completed'].append(index)
                    destination.write_text(json.dumps(report, indent=2)+'\n')
            finally:
                destination.write_text(json.dumps(report, indent=2)+'\n')
        report['passed'] = True
    finally:
        destination.write_text(json.dumps(report, indent=2)+'\n')
    print(f'PASS {args.game}: {report["suite"]}; all {count} backend partitions and selected frontend checks', flush=True)


def deferred_tests(root, game):
    available, result = {}, []
    for package, name in GAME_EXTENDED_TESTS.get(game, ()):
        if package not in available:
            available[package] = {test for path in (root / package.removeprefix('./')).glob('*_test.go') for test in re.findall(r'^func ((?:Test|Example|Fuzz)\w*)\(', path.read_text(), re.M)}
        if name in available[package]: result.append({'package': package, 'name': name})
    return result

def deferred_subtests(root, game):
    if game != 'yuyuhakusho': return []
    fixture = root / 'games/yuyuhakusho/renderstate/testdata/bridge-reference.json'
    if not fixture.is_file(): return []
    names = {row['configuration']['name'] for row in json.loads(fixture.read_text())['rows']}
    return [{'name': parent, 'case': name} for parent in ['TestRenderFrozenBridgeTrajectories', 'TestSchema2FrozenWholeTrajectories'] for name in YUYU_EXTENDED_TRAJECTORIES if name in names]

def run(args):
    extended = extended_mode(args)
    if args.game in GAME_PARTITIONS and args.backend and args.count == 1:
        return run_game_partitions(args)
    output = ROOT/'.local/ci'/args.game
    output.mkdir(parents=True, exist_ok=True)
    if args.game == 'pvz' and args.shard == 0:
        (ROOT/'.local/pvz/ci-assets').mkdir(parents=True, exist_ok=True)
    if args.game == 'quick':
        receipt = json.loads(Path(args.plan).read_text())
        commands = quick_commands(ROOT, receipt['quick_components'])
        if not commands: raise ValueError('The fast lane has no selected checks')
    elif args.game == 'shared':
        commands = go_commands(ROOT, 'shared', race=True)
    else:
        commands = []
        if args.shard == 0:
            frontend = frontend_tests(ROOT, args.game)
            if frontend:
                commands.append(['node', '--experimental-strip-types', '--test', *(str(p) for p in frontend)])
            assets = ROOT/'scripts'/args.game/'verify-assets.py'
            if assets.exists(): commands.append(['python3', '-B', str(assets), '--dist'])
        if args.backend:
            commands += go_commands(ROOT, args.game, args.shard, args.count, extended=extended)
            commands += go_commands(ROOT, args.game, args.shard, args.count, race=True, extended=extended)
    env = verification_env(args.game, args.backend, extended=extended)
    commands += build_commands(ROOT, args.game, args.shard)
    manifest = {'game': args.game, 'backend': args.backend, 'shard': args.shard, 'commands': commands, 'passed': False}
    if args.game in GAME_EXTENDED_TESTS:
        manifest.update(suite='extended' if extended else 'standard', deferredTests=[] if extended or not args.backend else deferred_tests(ROOT, args.game), deferredSubtests=[] if extended or not args.backend else deferred_subtests(ROOT, args.game))
    destination = output/f'result-{args.shard}.json'
    destination.write_text(json.dumps(manifest, indent=2) + '\n')
    for i, command in enumerate(commands):
        print('RUN', ' '.join(command), flush=True)
        with (output/f'{args.shard}-{i}.log').open('w') as log:
            subprocess.run(command, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
    manifest['passed'] = True
    destination.write_text(json.dumps(manifest, indent=2) + '\n')
    print(f'PASS {args.game}: {len(commands)} scoped checks', flush=True)

def test_blocks(source):


    literals = re.compile(r"//[^\n]*|/\*[\s\S]*?\*/|`[^`]*`|\"(?:\\.|[^\"\\])*\"|'(?:\\.|[^'\\])*'")
    masked = literals.sub(lambda match: re.sub(r'[^\n]', ' ', match.group()), source)
    blocks, spans = {}, []
    for match in re.finditer(r'^func ((?:Test|Example|Fuzz)\w*)\(', masked, re.M):
        if spans and match.start() < spans[-1][1]: continue
        start = masked.find('{', match.end())
        if start < 0: return None
        depth, end = 0, None
        for index in range(start, len(masked)):
            if masked[index] == '{': depth += 1
            elif masked[index] == '}':
                depth -= 1
                if depth == 0:
                    end = index + 1
                    break
        if end is None: return None
        blocks[match.group(1)] = source[match.start():end]
        while end < len(source) and source[end].isspace(): end += 1
        spans.append((match.start(), end))
    outside, cursor = [], 0
    for start, end in spans:
        outside.append(source[cursor:start])
        cursor = end
    outside.append(source[cursor:])
    return blocks, ''.join(outside).strip()

def changed_test_source(before, after, retired=()):


    if retired:
        def table(match):
            members = re.findall(r'"([^"\\]*)"', match.group(1))
            return '[]string{' + ','.join(json.dumps(g) for g in members if g not in retired) + '}'
        pattern = r'\[\]string\{\s*((?:"[^"\\]*"\s*,?\s*)*)\}'
        before = re.sub(pattern, table, before)
        after = re.sub(pattern, table, after)
    old, new = test_blocks(before), test_blocks(after)

    if old is None or new is None or old[1] != new[1]: return before + '\n' + after
    names = sorted(name for name in old[0].keys() | new[0].keys() if old[0].get(name) != new[0].get(name))
    return '\n'.join('func ' + name + '(' for name in names)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    p = sub.add_parser('plan')
    p.add_argument('--base', default='')
    p.add_argument('--head', default='HEAD')
    p.add_argument('--manual', default='')
    p.add_argument('--validation', choices=['auto', 'quick', 'full'], default='auto')
    p.add_argument('--resume-proof', type=Path)
    p.add_argument('--contra-campaigns', action='store_true', help='Opt into non-blocking Contra computer research campaigns')
    r = sub.add_parser('run')
    r.add_argument('--game', required=True, choices=['quick', 'shared', *game_names(ROOT)])
    r.add_argument('--plan', default='.local/ci/plan.json')
    r.add_argument('--backend', action='store_true')
    r.add_argument('--shard', type=int, default=0)
    r.add_argument('--count', type=int, default=1)
    r.add_argument('--extended', '--catan-extended', dest='extended', action='store_true', help='Run the explicitly requested full acceptance matrices')
    sub.add_parser('pvz-tools', help='Bind the scoped PvZ media verifier executable')
    args = parser.parse_args()
    if args.command == 'run': return run(args)
    if args.command == 'pvz-tools':
        print(json.dumps(prepare_pvz_asset_tools(ROOT)))
        return
    if args.base:
        subprocess.run(['git', 'merge-base', '--is-ancestor', args.base, args.head], cwd=ROOT, check=True)

        raw = subprocess.check_output(['git', 'diff', '--no-renames', '--name-only', '-z', args.base, args.head], cwd=ROOT)
    else:
        raw = subprocess.check_output(['git', 'ls-files', '-z'], cwd=ROOT)
    paths = changed_names(raw)
    legacy_sources = {}
    active_games = game_names(ROOT)
    retired = {path.split('/')[1] for path in paths if path.startswith('games/') and len(path.split('/')) > 2 and path.split('/')[1] not in active_games}
    for path in paths:
        if path.startswith('internal/') and path.endswith('_test.go'):
            current = subprocess.run(['git', 'show', args.head + ':' + path], cwd=ROOT, text=True, capture_output=True)
            source = current.stdout if current.returncode == 0 else ''
            if args.base:
                old = subprocess.run(['git', 'show', args.base + ':' + path], cwd=ROOT, text=True, capture_output=True)
                source = changed_test_source(old.stdout if old.returncode == 0 else '', source, retired=retired)
            legacy_sources[path] = source
    proof = json.loads(args.resume_proof.read_text()) if args.resume_proof else {'eligible': False}
    resumed = resume_selection(proof, subprocess.check_output(['git', 'rev-parse', args.head], cwd=ROOT, text=True).strip(), args.base, active_games)
    result = plan(paths, game_names(ROOT), args.manual, legacy_sources, contra_campaigns=args.contra_campaigns, validation=args.validation, resume_games=resumed)
    if proof.get('eligible'):
        result['reusedValidation'] = proof
        if args.validation != 'quick': result['build_required'] = True
    result.update(base=args.base, head=args.head, paths=changed_names(raw))
    out = ROOT/'.local/ci'
    out.mkdir(parents=True, exist_ok=True)
    (out/'plan.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'], 'a') as f:
            f.write('matrix=' + json.dumps(result['matrix']) + '\n')
            f.write('has_games=' + str(result['has_games']).lower() + '\n')
            for key in ['has_shared', 'has_quick', 'quick_go', 'quick_node_deps', 'frontend_checks', 'build_required']:
                f.write(key + '=' + str(result[key]).lower() + '\n')
            f.write('contra_campaign_matrix=' + json.dumps(result['contra_campaign_matrix']) + '\n')
            f.write('has_contra_campaigns=' + str(result['has_contra_campaigns']).lower() + '\n')
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as f:
            f.write('## Verification scope\n\nBaseline: `' + (args.base or 'bootstrap') + '`\n\n')
            f.write('Fast checks: ' + (', '.join(result['quick_components']) or 'none') + '\n\n')
            if proof.get('eligible'):
                f.write('Unchanged game validations reused from run ' + str(proof['sourceRun']) + ': ' + ', '.join(row['game'] for row in proof['reusedGames']) + '\n\n')
            f.write('Shared backend: ' + str(result['has_shared']).lower() + '; application build: ' + str(result['build_required']).lower() + '\n\nSelected games:\n\n')
            f.write('\n'.join('- ' + r['game'] + (': frontend/build and Go contracts; optional computer research listed separately' if r['game'] == 'contra' and r['backend'] else ': frontend, rules, full matches and race checks' if r['backend'] else ': frontend/assets only') for r in result['games']) or '- None')
            f.write('\n')
            if result['has_contra_campaigns']:
                f.write('\nOptional Contra computer research explicitly requested: standard terminal and assisted clear, each with one and two seats. These jobs do not gate human gameplay delivery.\n')
    print(json.dumps(result, ensure_ascii=False))

if __name__ == '__main__': main()
