"""Failure inventory before implementation: unrelated games fan out; frontend-only
changes start real-time Go matches; legacy test paths lose ownership; shared poker
changes miss consumers; rename/deletion or cancelled predecessor misses changes;
unknown manual game/invalid baseline silently passes; a failed required job still
uploads a release binary; shared selection includes full-game loops; test failures
are swallowed and empty optional frontend directories become false failures.
"""
import importlib.util
import os
import shlex
import subprocess
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('ci_scope', Path(__file__).with_name('scope.py'))
scope = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)

class ScopeTests(unittest.TestCase):
    def test_application_composition_checks_remain_shared(self):
        for path,name in [('contra_startup_test.go','TestContraStartupRejectsUnboundBuildBeforeDatabase'),
                          ('internal/production/modules_test.go','TestContraProductionRegistrationRequiresIdentities')]:
            self.assertEqual(scope.owner(Path(path),name,scope.game_names(ROOT)),set())
        commands=' '.join(' '.join(c) for c in scope.go_commands(ROOT,'shared',race=True))
        self.assertIn('TestContraStartupRejectsUnboundBuildBeforeDatabase',commands)
        self.assertIn('TestContraProductionRegistrationRequiresIdentities',commands)
        self.assertNotIn('TestBotStandardCampaignE2E',commands)

    def test_contra_build_contracts_are_scoped_and_do_not_launch_campaigns(self):
        commands=scope.build_commands(ROOT,'contra')
        expected={'identity_test.py','media_test.py','media_archive_test.py','builder_inputs_test.py','runtime_assets_test.py','delivery_test.py','ci_runtime_test.py','runtime-delivery.test.mjs'}
        self.assertEqual({Path(c[-1]).name for c in commands},expected)
        self.assertTrue(all(c[:2]==(['node','--test'] if c[-1].endswith('.mjs') else ['python3','-B']) for c in commands))
        for game,index in [('shared',0),('rail',0),('contra',1)]:
            self.assertEqual(scope.build_commands(ROOT,game,index),[])

    # Long campaigns run once through the explicit verified runner, not through
    # inherited diagnostic variables on ordinary/race test selections.
    def test_contra_campaigns_are_explicit_and_scoped(self):
        regular=' '.join(' '.join(c) for c in scope.go_commands(ROOT,'contra'))
        self.assertNotIn('TestBotStandardCampaignE2E',regular)
        expected=[{'mode':mode,'players':players} for mode in ['standard-terminal','assisted-clear'] for players in [1,2]]
        selected=self.plan('games/contra/engine/bot.go',contra_campaigns=True)
        self.assertTrue(selected['has_contra_campaigns'])
        self.assertTrue(selected['contra_campaign_requested'])
        self.assertEqual(selected['contra_campaign_matrix'],{'include':expected})
        for paths in [('games/contra/engine/bot.go',),('games/contra/server/input.go',),('games/contra/web/Board.tsx',),('games/rail/server/rules.go',),('main.go',),()]:
            selected=self.plan(*paths)
            self.assertFalse(selected['has_contra_campaigns'])
            self.assertFalse(selected['contra_campaign_requested'])
            self.assertEqual(selected['contra_campaign_matrix'],{'include':[]})
        self.assertFalse(self.plan(manual='contra')['has_contra_campaigns'])
        self.assertFalse(self.plan(manual='all')['has_contra_campaigns'])
        self.assertEqual(self.plan(contra_campaigns=True)['contra_campaign_matrix'],{'include':expected})
        dirty={'PATH':'keep','CONTRA_BOT_MODE':'diagnostic','CONTRA_BOT_PREFIX':'old','CONTRA_BOT_INFINITE_LIVES':'1','CONTRA_BOT_SEATS':'1','CONTRA_BOT_TARGET':'2','CONTRA_BOT_FRAMES':'1','CONTRA_BOT_EVIDENCE':'old','CONTRA_BOT_UNKNOWN':'bad'}
        clean=scope.verification_env('contra',True,dirty)
        self.assertEqual(clean,{'PATH':'keep'});self.assertEqual(dirty['CONTRA_BOT_MODE'],'diagnostic')

    def test_regular_contra_job_does_not_repeat_matrix_campaigns(self):
        parent=ROOT/'.local/ci-scope';parent.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent,prefix='campaign-scope-') as name:
            root=Path(name)
            with patch.object(scope,'ROOT',root),patch.object(scope,'go_commands',return_value=[]),patch.object(scope,'frontend_tests',return_value=[]),patch.object(scope,'build_commands',return_value=[]),patch.object(scope.subprocess,'run') as run:
                scope.run(SimpleNamespace(game='contra',backend=True,shard=0,count=1))
                self.assertFalse(run.called,'ordinary game job repeated campaign execution')

    def test_human_delivery_keeps_backend_checks_without_computer_campaigns(self):
        # Temporary user-authorized human-only delivery keeps rule and runtime
        # checks, while computer campaign research requires a separate opt-in.
        initialization=['games/contra/runtime/bridge.go','games/contra/runtime/bundle_protocol_test.go',
                        'games/contra/build/cmd/probe/main.go','games/contra/server/production_native.go',
                        'games/contra/server/production_assets_native.go','games/contra/server/production_native_test.go',
                        'games/contra/server/visual_loader.go','games/contra/server/fixture_visual_loader_test.go',
                        'games/contra/server/fixture_visual_assets_test.go','games/contra/server/runtime_asset_identity_test.go',
                        'games/contra/server/data/visual-catalog-v1.json.gz']
        for path in initialization:
            result=self.plan(path)
            self.assertEqual(result['games'],[{'game':'contra','backend':True}],path)
            self.assertFalse(result['has_contra_campaigns'],path)
            self.assertFalse(result['contra_campaign_requested'],path)
        for path in ['games/contra/build/identity.py','games/contra/assets/audio-source.json',
                     'games/contra/web/Board.tsx','internal/production/modules.go','main.go','contra_startup_test.go']:
            result=self.plan(path)
            self.assertFalse(result['has_contra_campaigns'],path)
            self.assertFalse(result['contra_campaign_requested'],path)
        for path in ['games/contra/engine/bot.go','games/contra/engine/leveldata/stage-8.json',
                     'games/contra/engine/testdata/route.json.gz','games/contra/server/input.go',
                     'games/contra/server/module.go','games/contra/server/timer.go',
                     'games/contra/server/http_e2e_test.go','games/contra/server/campaign_failure_e2e_test.go']:
            result=self.plan(path)
            self.assertEqual(result['games'],[{'game':'contra','backend':True}],path)
            self.assertFalse(result['contra_campaign_requested'],path)
            self.assertFalse(result['has_contra_campaigns'],path)
        for game in ['contra','all']:
            self.assertFalse(self.plan(manual=game)['contra_campaign_requested'])
            self.assertTrue(self.plan(manual=game,contra_campaigns=True)['contra_campaign_requested'])

    # New failures: rule/AI data under engine is labelled frontend-only; root
    # registry tests disappear; adding broad Production/Review race prefixes
    # accidentally pulls optional engine prototypes or unrelated games.
    def test_contra_rule_and_ai_data_select_only_its_backend(self):
        for path in ['games/contra/engine/leveldata/stage-8.json',
                     'games/contra/engine/audio_control_data.json',
                     'games/contra/engine/testdata/bot-visual-catalog.json.gz',
                     'games/contra/engine/testdata/visible-target.png']:
            self.assertEqual(self.plan(path)['games'], [{'game':'contra','backend':True}], path)
        for path in ['games/contra/web/Board.tsx', 'games/contra/assets/runtime-stages.json',
                     'scripts/contra/browser_input.mjs']:
            self.assertEqual(self.plan(path)['games'], [{'game':'contra','backend':False}], path)
        self.assertEqual(self.plan('games/contra/engine/README.md')['games'], [])

    def test_root_registry_tests_keep_shared_and_game_ownership(self):
        parent=ROOT/'.local/ci-scope';parent.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent,prefix='registry-') as temporary:
            root=Path(temporary)
            for game in ['contra','other']:(root/'games'/game).mkdir(parents=True)
            (root/'games/registry_test.go').write_text('package games\nfunc TestAllRegistryHooks(t *testing.T) {}\nfunc TestContraRegistrationContract(t *testing.T) {}\n')
            (root/'games/other/rules_test.go').write_text('package other\nfunc TestOtherRules(t *testing.T) {}\n')
            shared=scope.go_commands(root,'shared',race=True)
            self.assertTrue(any(c[-1]=='./games' and 'TestAllRegistryHooks' in ' '.join(c) for c in shared))
            self.assertNotIn('TestContraRegistrationContract',' '.join(' '.join(c) for c in shared))
            contra=scope.go_commands(root,'contra')
            self.assertIn('TestContraRegistrationContract',' '.join(' '.join(c) for c in contra))
            self.assertNotIn('TestOtherRules',' '.join(' '.join(c) for c in contra+shared))
        actual=scope.go_commands(ROOT,'shared',race=True)
        self.assertTrue(any(c[-1]=='./games' and 'TestAllIndependentGamesSupportComputers' in ' '.join(c) for c in actual))

    # Colocated browser tests must not silently disappear from the selected
    # game's required checks; discovery must not fan out to other games.
    def test_contra_frontend_includes_colocated_runtime_checks(self):
        found = {str(p.relative_to(ROOT)) for p in scope.frontend_tests(ROOT, 'contra')}
        for path in ['games/contra/tests/background.test.mjs',
                     'games/contra/web/runtime-room.test.mjs',
                     'games/contra/web/controller.test.mjs',
                     'games/contra/web/audio.test.mjs']:
            self.assertIn(path, found)
        self.assertTrue(all(p.startswith('games/contra/') for p in found))
        self.assertEqual(scope.frontend_tests(ROOT, 'missing-game'), [])

    def test_contra_race_covers_room_transport_and_restore(self):
        # Naming heuristics must not omit HTTP/WS, SQLite or paused replay races.
        commands = scope.go_commands(ROOT, 'contra', race=True)
        flat = '\n'.join(' '.join(command) for command in commands)
        for name in ['TestContraHTTPLoadingStreamAndSQLiteRestore',
                     'TestContraHTTPWSAutoplayInputRelease',
                     'TestContraModuleInputClockReplayAndRestore',
                     'TestRuntimePausedRevisionProtocol',
                     'TestRuntimeBundleStageInstallationAndRestore',
                     'TestProductionRequiresBothCompiledIdentities',
                     'TestProductionEmbeddedVisualsAndOfflineCallbacks',
                     'TestProductionArchiveRejectsMalformedData',
                     'TestProductionAndBrowserVisualCatalogueAgree',
                     'TestRuntimeArtifactStageConfirmationAndRestore',
                     'TestSessionBattleReviewIsCommittedAndRestorable',
                     'TestSessionBattleReviewSelectionRejectsFadeAndMenus',
                     'TestRuntimeTerminalUsesPersistedBattleReview']:
            self.assertIn(name, flat)
        self.assertNotIn('TestContraPlatformBrowserFixture', flat)
        self.assertNotIn('TestBotStandardCampaignE2E', flat)
        self.assertNotIn('TestProductionBitBattleCandidates', flat)
        self.assertTrue(all(command[-1].startswith('./games/contra/') for command in commands))

    def plan(self, *paths, manual='', contra_campaigns=False):
        return scope.plan(paths, scope.game_names(ROOT), manual, contra_campaigns=contra_campaigns)
    def test_mobile_and_pairs_frontend_do_not_start_rules(self):
        p = self.plan('games/snooker/web/Table.tsx', 'games/pairs/web/input.ts', 'scripts/snooker/check-gesture.py')
        self.assertEqual([r['game'] for r in p['games']], ['pairs', 'snooker'])
        self.assertTrue(all(not r['backend'] for r in p['games']))
    def test_rules_belong_only_to_their_game(self):
        p = self.plan('games/snooker/server/rules.go')
        self.assertEqual(p['games'], [{'game': 'snooker', 'backend': True}])
    def test_shared_changes_only_select_public_checks(self):
        for f in ['internal/server/server.go', 'internal/gameapi/module.go', 'web/src/main.tsx', '.github/workflows/build.yml', 'go.mod']:
            self.assertEqual(self.plan(f)['games'], [], f)
    def test_poker_shared_component_selects_only_its_consumers(self):
        self.assertEqual([r['game'] for r in self.plan('web/src/poker/Cards.tsx')['games']], ['doudizhu', 'guandan', 'shengji'])
    def test_legacy_game_test_and_asset_script_ownership(self):
        self.assertEqual(self.plan('internal/server/catan_explorer_http_test.go')['games'], [{'game': 'catan', 'backend': True}])
        self.assertEqual(self.plan('scripts/prepare_sanguosha_assets.py')['games'], [{'game': 'sanguosha', 'backend': False}])
    def test_docs_do_not_request_complete_games(self):
        self.assertEqual(self.plan('games/snooker/README.md', 'docs/research/chess-family.md', 'AGENTS.md')['games'], [])
    def test_explicit_full_and_one_game_selection(self):
        # Compare the actual catalog, so adding a game does not require updating
        # a magic count and cannot silently omit the new entry from full runs.
        catalog = sorted(p.name for p in (ROOT/'games').iterdir()
                         if p.is_dir() and not p.name.startswith(('.', '_')))
        self.assertEqual(self.plan(manual='all')['games'],
                         [{'game': game, 'backend': True} for game in catalog])
        self.assertEqual(self.plan(manual='snooker')['games'], [{'game': 'snooker', 'backend': True}])
        with self.assertRaises(ValueError): self.plan(manual='unknown')
    def test_shared_test_selection_excludes_game_full_matches(self):
        commands = scope.go_commands(ROOT, 'shared')
        flat = '\n'.join(' '.join(c) for c in commands)
        self.assertIn('TestAuthCSRFAndStatic', flat)
        self.assertIn('TestModuleRegistrationAndLifecycleE2E', flat)
        self.assertIn('TestConcurrentCommandsOnlyOneCommits', flat)
        for name in ['TestCatan', 'TestBotsFinishBothGames', 'TestAutoPlayAllGamesFinishAndCreditHumanPlayers', 'TestTimeoutAutoplayAllGamesContinueToNaturalFinish']:
            self.assertNotIn(name, flat)
        self.assertNotIn('./games/...', flat)
    def test_game_commands_select_exact_legacy_subtests(self):
        commands = scope.go_commands(ROOT, 'rail')
        flat = '\n'.join(' '.join(c) for c in commands)
        self.assertTrue(any(c[-1].startswith('./games/rail/') for c in commands))
        self.assertIn('TestRailHiddenTasksThroughAPI', flat)
        self.assertIn('^TestBotsFinishBothGames$/^rail-', flat)
        self.assertNotIn('TestCatan', flat)
    def test_basketball_full_acceptance_only_for_backend_changes(self):
        # Failure inventory: full game gates silently stay skipped in main;
        # asset/frontend changes or shared checks start expensive game runs.
        expected = {'BASKETBALL_PREPARATION_MATRIX', 'BASKETBALL_SELECTION_ADMISSION', 'BASKETBALL_FULL_HTTP'}
        self.assertTrue(all(scope.verification_env('basketball', True, {})[key] == '1' for key in expected))
        for game, backend in [('basketball', False), ('shared', True), ('racing', True)]:
            self.assertFalse(expected.intersection(scope.verification_env(game, backend, {})))

    def test_basketball_race_checks_include_stream_restore_and_presentation(self):
        flat = '\n'.join(' '.join(c) for c in scope.go_commands(ROOT, 'basketball', race=True))
        for name in ['TestBasketballRealtimeStreamAndRestore', 'TestCampaignEndingConfirmationDeadline', 'TestRuntimePresentationDisplayLifecycle']:
            self.assertIn(name, flat)
        self.assertNotIn('TestRuntimeOriginalSelectionAdmissionMatrix', flat)
        self.assertNotIn('TestBasketballHTTPCompletePreparedMatch', flat)

    def test_changed_range_covers_old_and_new_paths(self):
        self.assertEqual(scope.changed_names(b'games/pairs/web/Old.tsx\0games/snooker/web/New.tsx\0'), ['games/pairs/web/Old.tsx', 'games/snooker/web/New.tsx'])
    def test_workflow_binary_waits_for_all_selected_checks(self):
        workflow = (ROOT/'.github/workflows/build.yml').read_text()
        self.assertIn('needs: [plan, frontend, shared, games]', workflow)
        self.assertIn("needs.games.result == 'success' || needs.games.result == 'skipped'", workflow)
        self.assertNotIn('needs.contra_campaigns',workflow.split('  verify:',1)[1])
        self.assertIn("matrix: ${{ fromJSON(needs.plan.outputs.contra_campaign_matrix) }}",workflow)
        campaign=workflow.split('  contra_campaigns:',1)[1].split('  verify:',1)[0]
        self.assertIn('timeout-minutes: 110',campaign)
        self.assertIn('continue-on-error: true',campaign)
        self.assertIn('scripts/contra/campaign_modes.py --mode "$CAMPAIGN_MODE" --players "$CAMPAIGN_PLAYERS"',campaign)
        switch=workflow.split('      contra_campaigns:',1)[1].split('permissions:',1)[0]
        self.assertIn('type: boolean',switch)
        self.assertIn('default: false',switch)
        self.assertIn('inputs.contra_campaigns',workflow)
        self.assertIn('--contra-campaigns',workflow)
        self.assertNotIn('shard: [0, 1, 2, 3', workflow)
        self.assertNotIn('WIRE_BOARD_SNOOKER_HTTP_FULL=1 go test', workflow)

    def test_verified_native_build_and_source_container_bind_runtime(self):
        workflow=(ROOT/'.github/workflows/build.yml').read_text().split('  verify:',1)[1]
        steps=['games/contra/build/ci_runtime.py','scripts/contra/accept_production.py','scripts/cdn_delivery.py publish','scripts/cdn_delivery.py trim','games/contra/build/native.py']
        positions=[workflow.index(step) for step in steps]
        self.assertEqual(positions,sorted(positions))
        self.assertIn('--runtime .local/contra-ci-runtime/manifest.json',workflow)
        self.assertIn('--goos linux --goarch amd64',workflow)
        self.assertIn('dist/package/verification-plan.json',workflow)
        self.assertIn('dist/package/wire-board.build.json',workflow)
        self.assertNotIn('go build -trimpath -ldflags="-s -w"',workflow)
        docker=(ROOT/'Dockerfile').read_text()
        for text in ['python3','xz','COPY scripts/ /src/scripts/','COPY internal/ /src/internal/','ARG SOURCE_SHA','ARG WIRE_BOARD_ASSET_BASE','games/contra/build/ci_runtime.py','scripts/cdn_delivery.py verify','scripts/cdn_delivery.py trim','games/contra/build/native.py']:
            self.assertIn(text,docker)
        self.assertNotIn('scripts/cdn_delivery.py publish',docker)
        self.assertLess(docker.index('scripts/cdn_delivery.py trim'),docker.index('games/contra/build/native.py'))
        self.assertNotIn('go build -trimpath -ldflags="-s -w"',docker)

    def test_source_container_requires_matching_commit_asset_base(self):
        docker=(ROOT/'Dockerfile').read_text()
        line=next(line for line in docker.splitlines() if line.startswith('RUN node --input-type=module -e '))
        command=shlex.split(line.removeprefix('RUN '))
        for source,base,passed in [('', '', False),('a'*40,'https://cdn.ichenj.com/bga/web/'+'b'*40+'/',False),('a'*40,'https://cdn.ichenj.com/bga/web/'+'a'*40+'/',True)]:
            result=subprocess.run(command,cwd=ROOT,env={'PATH':os.environ['PATH'],'SOURCE_SHA':source,'WIRE_BOARD_ASSET_BASE':base},capture_output=True,text=True)
            self.assertEqual(result.returncode==0,passed,(source,base,result.stderr))


class RetiredGameScopeTests(unittest.TestCase):
    # Removing only a retired ID from a shared scenario table must not force
    # every surviving game's full rule matrix. Any actual scenario edit must.
    def test_retired_table_member_does_not_change_surviving_scenarios(self):
        before = 'func TestTimeoutAutoplayAllGamesContinueToNaturalFinish(t *testing.T) { for _, kind := range []string{"splendor", "dota"} { play(kind) } }'
        after = before.replace(', "dota"', '')
        self.assertEqual(scope.changed_test_source(before, after, retired={'dota'}), '')
        self.assertIn('TestTimeoutAutoplayAllGamesContinueToNaturalFinish', scope.changed_test_source(before, after.replace('play(kind)', 'playNew(kind)'), retired={'dota'}))
    def test_other_strings_are_not_ignored(self):
        before = 'func TestPublic(t *testing.T) { x := "dota" }'
        self.assertIn('TestPublic', scope.changed_test_source(before, before.replace('dota', 'rail'), retired={'dota'}))

if __name__ == '__main__': unittest.main()
