"""Failure inventory before implementation: unrelated games fan out; frontend-only
changes start real-time Go matches; legacy test paths lose ownership; shared poker
changes miss consumers; rename/deletion or cancelled predecessor misses changes;
unknown manual game/invalid baseline silently passes; a failed required job still
uploads a release binary; shared selection includes full-game loops; test failures
are swallowed and empty optional frontend directories become false failures.
"""
import importlib.util
import unittest
from pathlib import Path
ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('ci_scope', Path(__file__).with_name('scope.py'))
scope = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)

class ScopeTests(unittest.TestCase):
    def plan(self, *paths, manual=''):
        return scope.plan(paths, scope.game_names(ROOT), manual)
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
        self.assertNotIn('shard: [0, 1, 2, 3', workflow)
        self.assertNotIn('WIRE_BOARD_SNOOKER_HTTP_FULL=1 go test', workflow)

if __name__ == '__main__': unittest.main()
