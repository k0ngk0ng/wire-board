"""Failure inventory before implementation: unrelated games fan out; frontend-only
changes start real-time Go matches; legacy test paths lose ownership; shared poker
changes miss consumers; rename/deletion or cancelled predecessor misses changes;
unknown manual game/invalid baseline silently passes; a failed required job still
uploads a release binary; shared selection includes full-game loops; test failures
are swallowed and empty optional frontend directories become false failures.

PvZ asset-gate failures declared before implementation: semantic/native/music
checks are omitted, run on unrelated games or repeat on later shards; CI needs
private EXE/PAK/MO3/BASS inputs; Pillow is installed globally; FFmpeg is silently
missing; caches/temporary files escape the workspace; output parents are absent.

Missing-tool failures before the runner fallback: absent FFmpeg aborts before
recording evidence; an unpinned/global download is selected; the bound wheel
executable cannot run; cleanup deletes the failure result with its temporary
tools. FLAC verification must retain full PCM checks without needing ffprobe.
"""
import importlib.util
import json
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
    def test_game_control_policy_tools_select_frontend_checks_only(self):
        for path in ['web/scripts/game-control-lint.mjs', 'web/scripts/game-control-lint.json', 'web/scripts/room-ui-lint.mjs']:
            with self.subTest(path=path):
                result = self.plan(path)
                self.assertEqual(result['quick_components'], ['frontend'])
                self.assertFalse(result['has_shared'])
                self.assertFalse(result['has_games'])
                self.assertTrue(result['frontend_checks'])
                self.assertTrue(result['build_required'])

    def test_frontend_checks_install_go_for_real_http_browser_fixture(self):
        result = self.plan('web/src/ui/result-overlay.tsx')
        self.assertTrue(result['frontend_checks'])
        self.assertTrue(result['quick_go'])
        self.assertFalse(result['has_shared'])
        self.assertFalse(result['has_games'])

    def test_real_http_browser_fixture_restores_frontend_application_inputs(self):
        workflow = (ROOT / '.github/workflows/build.yml').read_text()
        quick = workflow.split('  quick:', 1)[1].split('  frontend:', 1)[0]
        self.assertIn('Restore application inputs for browser E2E', quick)
        self.assertIn('scope: frontend', quick)
        self.assertNotIn('scope: game', quick)

    def test_shared_settlement_acceptance_scripts_use_frontend_lane_only(self):
        for path in ['games/monopoly/tests/accept_ui.py', 'games/hotlinemiami/tests/accept_browser.py', 'scripts/basketball/accept_platform_rooms.py', 'scripts/contra/victory_browser_accept.py', 'scripts/game-loading/check-results.py', 'scripts/accept_draw_result.py']:
            with self.subTest(path=path):
                result = self.plan(path)
                self.assertEqual(result['quick_components'], ['frontend'])
                self.assertFalse(result['has_shared'])
                self.assertFalse(result['has_games'])
                self.assertTrue(result['build_required'])

    def test_shared_action_definitions_are_ci_configuration(self):
        result = self.plan('.github/actions/restore-resources/action.yml')
        self.assertEqual(result['quick_components'], ['ci'])
        self.assertFalse(result['has_shared'])
        self.assertFalse(result['has_games'])
        self.assertFalse(result['build_required'])

    def test_paid_traffic_policy_runs_before_build_selection_and_shared_vet_is_scoped(self):
        workflow = (ROOT / '.github/workflows/build.yml').read_text()
        plan = workflow.split('  plan:', 1)[1].split('  quick:', 1)[0]
        self.assertIn('python3 -B scripts/ci/traffic_lint.py --self-test', plan)
        self.assertLess(plan.index('traffic_lint.py'), plan.index('Find the last verified ancestor'))
        self.assertIn('go vet . ./internal/...', workflow)
        self.assertNotIn('go vet ./...', workflow)

    def test_cover_changes_build_frontend_without_game_ci(self):
        for game in scope.game_names(ROOT):
            paths = [f'games/{game}/web/Cover.tsx', f'scripts/{game}/accept_cover.py', f'scripts/{game}/cover_harness.tsx']
            for path in paths:
                with self.subTest(path=path):
                    result = self.plan(path)
                    self.assertEqual(result['games'], [])
                    self.assertEqual(result['quick_components'], ['frontend'])
                    self.assertFalse(result['has_shared'])
                    self.assertTrue(result['frontend_checks'])
                    self.assertTrue(result['build_required'])

    def test_cover_exemption_preserves_mixed_and_explicit_game_checks(self):
        cover = 'games/basketball/web/Cover.tsx'
        for path, backend in [('games/basketball/web/Board.tsx', False), ('games/basketball/server/module.go', True)]:
            result = self.plan(cover, path)
            self.assertEqual(result['games'], [{'game': 'basketball', 'backend': backend}])
            self.assertTrue(result['frontend_checks'])
        explicit = self.plan(cover, manual='basketball')
        self.assertEqual(explicit['games'], [{'game': 'basketball', 'backend': True}])
        self.assertTrue(self.plan(cover, manual='all')['has_games'])
        self.assertTrue(self.plan('games/unknown/web/Cover.tsx')['has_shared'])

    def test_candidate_smoke_receives_the_complete_oss_configuration(self):
        workflow = (ROOT / '.github/workflows/build.yml').read_text()
        smoke = workflow.split('      - name: Smoke test Linux binary', 1)[1].split('      - uses:', 1)[0]
        self.assertIn('OSS_ENDPOINT: https://${{ vars.OSS_ENDPOINT }}', smoke)
        for key in ['OSS_ACCESS_KEY_ID', 'OSS_ACCESS_KEY_SECRET']:
            self.assertIn(key + ': ${{ secrets.' + key + ' }}', smoke)

    def test_asset_delivery_tools_require_fresh_build_and_their_own_acceptance(self):
        for path in ['scripts/cdn_delivery.py', 'scripts/test_cdn_delivery.py']:
            result = self.plan(path)
            self.assertEqual(result['quick_components'], ['ci'])
            self.assertFalse(result['has_shared'])
            self.assertFalse(result['frontend_checks'])
            self.assertEqual(result['games'], [])
            self.assertTrue(result['build_required'])
        commands = scope.quick_commands(ROOT, ['ci'])
        self.assertIn(['python3', '-B', 'scripts/test_cdn_delivery.py'], commands)
        self.assertIn(['python3', '-B', 'scripts/ci/cdn_concurrency_e2e_test.py'], commands)

    def test_resumed_games_remain_required_and_proof_is_bound_to_the_plan(self):
        result = scope.plan(['scripts/ci/scope.py'], scope.game_names(ROOT), resume_games=('catan',))
        self.assertEqual(result['games'], [{'game': 'catan', 'backend': True}])
        self.assertTrue(result['build_required'])
        self.assertFalse(result['has_shared'])
        proof = {'eligible': True, 'head': 'a' * 40, 'base': 'b' * 40, 'pendingGames': ['catan'], 'reusedGames': [{'game': 'rail'}]}
        self.assertEqual(scope.resume_selection(proof, 'a' * 40, 'b' * 40, ['catan', 'rail']), ['catan'])
        for changed in [dict(proof, head='c' * 40), dict(proof, base='c' * 40), dict(proof, pendingGames=[]), dict(proof, pendingGames=['catan', 'rail'])]:
            with self.assertRaises(ValueError):
                scope.resume_selection(changed, 'a' * 40, 'b' * 40, ['catan', 'rail'])

    def test_stoneage_namespace_checks_install_their_build_dependency(self):
        workflow=(ROOT/'.github/workflows/build.yml').read_text()
        block=workflow.split('      - name: Install dependencies for game rendering checks',1)[1].split('      - name: Prepare PvZ asset verification environment',1)[0]
        self.assertIn("matrix.game == 'stoneage'",block)
        self.assertIn('npm --prefix web ci',block)

    def test_clean_jobs_restore_external_resources_before_validation(self):
        workflow=(ROOT/'.github/workflows/build.yml').read_text()
        for job,following,validation in [('shared','games','Test shared platform behavior'),('games','verify','Install dependencies for game rendering checks'),('verify','complete','Preserve source-bound runtime metadata')]:
            block=workflow.split('  '+job+':',1)[1].split('  '+following+':',1)[0]
            restore='uses: ./.github/actions/restore-resources'
            self.assertIn(restore,block,job)
            self.assertNotIn('scripts/resource_store.py hydrate',block,job)
            self.assertNotIn('OSS_ACCESS_KEY_SECRET',block.split(validation,1)[0],job)
            self.assertLess(block.index(restore),block.index(validation),job)
        research=(ROOT/'.github/workflows/contra-research.yml').read_text()
        self.assertIn('CDN_ORIGIN: ${{ vars.CDN_ORIGIN }}',research)
        self.assertLess(research.index('uses: ./.github/actions/restore-resources'),research.index('scripts/contra/campaign_modes.py'))

    def test_pokepath_frontend_uses_clean_serial_delivery_entry(self):
        temporary=ROOT/'.local/tmp'
        temporary.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=temporary) as folder:
            root=Path(folder)
            base=root/'games/pokepath/tests'
            base.mkdir(parents=True)
            for name in ['ci.test.mjs','tower-order-reference.test.mjs','native-architecture.test.mjs']:
                (base/name).write_text('')
            self.assertEqual(scope.frontend_tests(root,'pokepath'),[base/'ci.test.mjs'])
            (base/'ci.test.mjs').unlink()
            with self.assertRaises(FileNotFoundError):
                scope.frontend_tests(root,'pokepath')

    def test_pokepath_delivery_installs_dependencies_and_keeps_other_games(self):
        workflow=(ROOT/'.github/workflows/build.yml').read_text()
        start=workflow.index('      - name: Install dependencies for game rendering checks')
        finish=workflow.index('      - name: Prepare PvZ asset verification environment',start)
        self.assertIn("matrix.game == 'pokepath'",workflow[start:finish])
        self.assertTrue(scope.frontend_tests(ROOT,'contra'))
        result=self.plan('games/pokepath/engine/combat.go')
        self.assertEqual(result['games'],[{'game':'pokepath','backend':True}])
        self.assertFalse(result['has_shared'])

    def test_release_history_browser_runner_selects_frontend_checks(self):
        result = self.plan('scripts/check-release-history.py')
        self.assertEqual(result['quick_components'], ['frontend'])
        self.assertTrue(result['build_required'])
        self.assertFalse(result['has_shared'])
        self.assertEqual(result['games'], [])
        mixed = self.plan('scripts/check-release-history.py', 'internal/server/static_delivery.go')
        self.assertTrue(mixed['has_shared'])
        self.assertTrue(mixed['frontend_checks'])




    def test_starfort_unsharded_suite_retains_campaigns_with_adequate_time(self):
        for race in [False, True]:
            commands = scope.go_commands(ROOT, 'starfort', race=race, extended=True)
            self.assertTrue(commands)
            self.assertTrue(all('-timeout=90m' in c for c in commands))
        full = ' '.join(' '.join(c) for c in scope.go_commands(ROOT, 'starfort', extended=True))
        for name in ['TestStarfortPublicNatural', 'TestStarfortLatestNatural',
                     'TestStarfortWorldNaturalCampaigns', 'TestStarfortWorldEndlessForty',
                     'TestStarfortPublicRewardChoiceClock']:
            self.assertIn(name, full)
        for game in ['shared', 'rail']:
            self.assertTrue(all('-timeout=45m' in c for c in scope.go_commands(ROOT, game)))
        for shard in range(4):
            self.assertTrue(all('-timeout=45m' in c for c in scope.go_commands(ROOT, 'starfort', shard, 4, extended=True)))




    def test_wish_only_changes_select_targeted_shared_checks(self):
        result=self.plan('internal/server/wishes.go','scripts/ci/scope.py')
        self.assertEqual(result['quick_components'],['ci','wishes'])
        self.assertFalse(result['has_shared'])
        mixed=self.plan('internal/server/wishes.go','internal/server/server.go')
        self.assertTrue(mixed['has_shared'])
        self.assertNotIn('wishes',mixed['quick_components'])
        self.assertTrue(self.plan(manual='all')['has_shared'])

    def test_wish_shared_commands_do_not_run_global_races(self):
        commands = scope.quick_commands(ROOT, ['wishes'])
        self.assertEqual(len(commands), 1)
        self.assertEqual(commands[0][-1], './internal/server')
        self.assertNotIn('-race', commands[0])
        for name in ['TestWishTextExpandedLimit','TestOSSSignatureReferenceVectors','TestAgentFeedDisabledAndStatusGuard']:
            self.assertIn(name, ' '.join(commands[0]))
        self.assertNotIn('shared_scope', self.plan('internal/server/wishes.go'))




    def test_md_trajectory_cases_partition_once_across_existing_shards(self):
        expected = {
            'TestRenderFrozenBridgeTrajectories': ('RENDERSTATE', './games/yuyuhakusho/renderstate'),
            'TestSchema2FrozenWholeTrajectories': ('CLIENT_BRIDGE', './games/yuyuhakusho/runtimebridge'),
        }
        rows=json.loads((ROOT/'games/yuyuhakusho/renderstate/testdata/bridge-reference.json').read_text())['rows']
        self.assertEqual(len(rows),49)
        for name,(prefix,package) in expected.items():
            cases=[]
            for shard in range(16):
                selected=[c for c in scope.go_commands(ROOT,'yuyuhakusho',shard,16,extended=True) if '^'+name+'$' in c]
                self.assertEqual(len(selected),1,(name,shard,selected))
                self.assertEqual(selected[0][:3],['env',prefix+'_SHARDS=16',prefix+'_SHARD='+str(shard)])
                self.assertEqual(selected[0][-1],package)
                cases += [index for index in range(len(rows)) if index%16==shard]
            self.assertEqual(sorted(cases),list(range(49)))
            single=[c for c in scope.go_commands(ROOT,'yuyuhakusho',extended=True) if '^'+name+'$' in c]
            self.assertEqual(len(single),1)
            self.assertEqual(single[0][:3],['env',prefix+'_SHARDS=1',prefix+'_SHARD=0'])
            self.assertFalse(any(name in ' '.join(c) for c in scope.go_commands(ROOT,'yuyuhakusho',race=True)))
            self.assertFalse(any(name in ' '.join(c) for c in scope.go_commands(ROOT,'shared')))

    def test_yuyuhakusho_build_binding_checks_run_only_once(self):
        self.assertEqual(scope.build_commands(ROOT,'yuyuhakusho',0),[['node','--test',str(ROOT/'games/yuyuhakusho/build/runtime-delivery.test.mjs')], ['go','test','-run','^$','./games/yuyuhakusho/cmd/verify-arena-aux']])
        self.assertEqual(scope.build_commands(ROOT,'yuyuhakusho',1),[])

    def test_yuyuhakusho_backend_shards_do_not_repeat_other_games(self):
        selected=self.plan('games/yuyuhakusho/server/stream_view.go')
        self.assertEqual(selected['games'],[{'game':'yuyuhakusho','backend':True}])
        self.assertEqual(len(selected['matrix']['include']),1)
        self.assertTrue(all(row['game']=='yuyuhakusho' and row['count']==1 for row in selected['matrix']['include']))
        frontend=self.plan('games/yuyuhakusho/web/Board.tsx')
        self.assertEqual(frontend['matrix']['include'],[{'game':'yuyuhakusho','backend':False,'shard':0,'count':1}])

    def test_application_composition_checks_remain_shared(self):
        for path,name in [('internal/production/startup_test.go','TestStartupRejectsUnboundBuildBeforeDatabase'),
                          ('internal/production/modules_test.go','TestContraProductionRegistrationRequiresIdentities')]:
            self.assertEqual(scope.owner(Path(path),name,scope.game_names(ROOT)),set())
        commands=' '.join(' '.join(c) for c in scope.go_commands(ROOT,'shared',race=True))
        self.assertIn('TestStartupRejectsUnboundBuildBeforeDatabase',commands)
        self.assertFalse((ROOT/'contra_startup_test.go').exists())

        self.assertEqual(scope.owner(Path('contra_startup_test.go'),'TestContraStartupRejectsUnboundBuildBeforeDatabase',scope.game_names(ROOT)),set())
        self.assertIn('TestContraProductionRegistrationRequiresIdentities',commands)
        self.assertNotIn('TestBotStandardCampaignE2E',commands)

    def test_contra_build_contracts_are_scoped_and_do_not_launch_campaigns(self):
        commands=scope.build_commands(ROOT,'contra')
        expected={'identity_test.py','media_test.py','media_archive_test.py','builder_inputs_test.py','runtime_assets_test.py','delivery_test.py','ci_runtime_test.py','runtime-delivery.test.mjs'}
        self.assertEqual({Path(c[-1]).name for c in commands},expected)
        self.assertTrue(all(c[:2]==(['node','--test'] if c[-1].endswith('.mjs') else ['python3','-B']) for c in commands))
        for game,index in [('shared',0),('rail',0),('contra',1)]:
            self.assertEqual(scope.build_commands(ROOT,game,index),[])

    def test_pvz_asset_build_contracts_are_complete_and_scoped(self):
        base=ROOT/'games/pvz/build';out=ROOT/'.local/pvz/ci-assets'
        expected=[['python3','-B','-m','unittest','discover','-s',str(base),'-p','test_*.py'],
                  ['python3','-B',str(base/'verify_native.py'),'--output',str(out/'native.json')],
                  ['python3','-B',str(base/'verify_music.py'),'--output',str(out/'music.json')]]
        self.assertEqual(scope.build_commands(ROOT,'pvz',0),expected)
        for game,index in [('pvz',1),('pvz',2),('shared',0),('rail',0),('conquest',0)]:
            self.assertEqual(scope.build_commands(ROOT,game,index),[])
        flat=' '.join(' '.join(c) for c in expected)
        for private_source in ['.local/pvz-research','PlantsVsZombies.exe','main.pak','.mo3','BassDecoder','render_music']:
            self.assertNotIn(private_source,flat)

    def test_pvz_asset_changes_do_not_request_other_game_rules(self):
        for path in ['games/pvz/assets/public/pvz/classic-v2/native/peashooter.pvzr',
                     'games/pvz/assets/public/pvz/classic-v2/music/day-main.flac',
                     'games/pvz/build/verify_native.py','games/pvz/build/verify_music.py']:
            self.assertEqual(self.plan(path)['games'],[{'game':'pvz','backend':False}],path)
        self.assertTrue(all(c[-1].startswith('./games/pvz/') for c in scope.go_commands(ROOT,'pvz')))

    def test_pvz_asset_run_creates_only_its_artifact_parent(self):
        parent=ROOT/'.local/ci-scope';parent.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent,prefix='pvz-assets-') as temporary:
            root=Path(temporary);asset_output=root/'.local/pvz/ci-assets'
            def verify_output(*args,**kwargs):
                self.assertTrue(asset_output.is_dir(),'asset gate ran before its output directory existed')
                self.assertEqual(kwargs['cwd'],root)
            with patch.object(scope,'ROOT',root),patch.object(scope,'frontend_tests',return_value=[]),patch.object(scope,'go_commands',return_value=[]),patch.object(scope,'build_commands',return_value=[['python3','asset-check']]),patch.object(scope.subprocess,'run',side_effect=verify_output) as run:
                scope.run(SimpleNamespace(game='pvz',backend=False,shard=0,count=1))
                self.assertEqual(run.call_count,1)
        with tempfile.TemporaryDirectory(dir=parent,prefix='other-assets-') as temporary:
            root=Path(temporary)
            with patch.object(scope,'ROOT',root),patch.object(scope,'frontend_tests',return_value=[]),patch.object(scope,'go_commands',return_value=[]),patch.object(scope,'build_commands',return_value=[]):
                scope.run(SimpleNamespace(game='rail',backend=False,shard=0,count=1))
            self.assertFalse((root/'.local/pvz').exists())

    def test_pvz_workflow_uses_scoped_workspace_asset_environment(self):
        workflow=(ROOT/'.github/workflows/build.yml').read_text()
        gate=workflow.split('      - name: Prepare PvZ asset verification environment\n',1)[1].split('      - name: Validate this game only\n',1)[0]
        self.assertIn("if: matrix.game == 'pvz'",gate)
        for command in ['if ! command -v ffmpeg >/dev/null 2>&1; then',
                        'python3 -m venv .local/pvz/ci-assets/venv',
                        '.local/pvz/ci-assets/venv/bin/python -m pip install Pillow==13.0.0',
                        '.local/pvz/ci-assets/venv/bin/python -m pip install imageio-ffmpeg==0.6.0',
                        '.local/pvz/ci-assets/venv/bin/python -B scripts/ci/scope.py pvz-tools',
                        'prepare-result.json', 'install.log', 'set -o pipefail',
                        'export TMPDIR="$PWD/.local/tmp"',
                        'export PIP_CACHE_DIR="$PWD/.local/pvz/ci-assets/pip-cache"']:
            self.assertIn(command,gate)
        for forbidden in ['apt-get','brew install','sudo','main.pak','.mo3','BASS']:
            self.assertNotIn(forbidden,gate)
        self.assertNotIn('command -v ffprobe',gate)
        run=workflow.split('      - name: Validate this game only\n',1)[1].split('      - uses: actions/upload-artifact@v4',1)[0]
        self.assertIn('if [ "$GAME_ID" = pvz ]; then',run)
        self.assertIn('export PATH="$PWD/.local/pvz/ci-assets/bin:$PWD/.local/pvz/ci-assets/venv/bin:$PATH"',run)
        cleanup=workflow.split('      - name: Remove temporary PvZ asset tools\n',1)[1].split('      - uses: actions/upload-artifact@v4',1)[0]
        self.assertIn('.local/pvz/ci-assets/bin',cleanup)
        self.assertNotIn('rm -rf .local/pvz/ci-assets\n',cleanup)

    def test_pvz_missing_ffmpeg_binds_pinned_workspace_wheel_and_preserves_report(self):
        parent=ROOT/'.local/ci-scope';parent.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent,prefix='pvz-no-ffmpeg-') as temporary:
            root=Path(temporary);package=root/'wheel/imageio_ffmpeg';package.mkdir(parents=True)
            executable=package/'ffmpeg-fixture';executable.write_text('#!/bin/sh\nprintf "ffmpeg version fixture\\n"\n');executable.chmod(0o755)
            fake=SimpleNamespace(__file__=str(package/'__init__.py'),get_ffmpeg_exe=lambda:str(executable))
            with patch('shutil.which',return_value=None),patch.dict('sys.modules',{'imageio_ffmpeg':fake}),patch('importlib.metadata.version',return_value='0.6.0'):
                report=scope.prepare_pvz_asset_tools(root)
            self.assertTrue(report['passed']);self.assertEqual(report['source'],'imageio-ffmpeg==0.6.0')
            self.assertEqual((root/'.local/pvz/ci-assets/bin/ffmpeg').resolve(),executable)
            self.assertEqual(json.loads((root/'.local/pvz/ci-assets/prepare-result.json').read_text())['ffmpegVersion'],'ffmpeg version fixture')

    def test_pvz_existing_ffmpeg_does_not_require_or_install_a_fallback(self):
        parent=ROOT/'.local/ci-scope';parent.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=parent,prefix='pvz-system-ffmpeg-') as temporary:
            root=Path(temporary);executable=root/'system-ffmpeg';executable.write_text('#!/bin/sh\nprintf "ffmpeg version system-fixture\\n"\n');executable.chmod(0o755)
            with patch('shutil.which',return_value=str(executable)),patch.dict('sys.modules',{'imageio_ffmpeg':None}):
                report=scope.prepare_pvz_asset_tools(root)
            self.assertTrue(report['passed']);self.assertEqual(report['source'],'system')

    def test_pvz_bad_fallback_keeps_a_failure_result(self):
        parent=ROOT/'.local/ci-scope';parent.mkdir(parents=True,exist_ok=True)
        for case in ['wrong-version','missing-executable']:
            with self.subTest(case=case),tempfile.TemporaryDirectory(dir=parent,prefix='pvz-tool-failure-') as temporary:
                root=Path(temporary);package=root/'wheel/imageio_ffmpeg';package.mkdir(parents=True)
                fake=SimpleNamespace(__file__=str(package/'__init__.py'),get_ffmpeg_exe=lambda:str(package/'missing-ffmpeg'))
                with patch('shutil.which',return_value=None),patch.dict('sys.modules',{'imageio_ffmpeg':fake}),patch('importlib.metadata.version',return_value='1.0.0' if case=='wrong-version' else '0.6.0'):
                    with self.assertRaises((RuntimeError,ValueError,FileNotFoundError)):
                        scope.prepare_pvz_asset_tools(root)
                result=json.loads((root/'.local/pvz/ci-assets/prepare-result.json').read_text())
                self.assertFalse(result['passed']);self.assertIn('errorType',result)



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
                     'games/contra/web/Board.tsx','internal/production/modules.go','main.go','internal/production/startup_test.go']:
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
    def test_racing_lint_tool_does_not_start_driving_rules(self):
        result = self.plan('games/racing/build/lint_go.go')
        self.assertEqual(result['games'], [])
        self.assertEqual(result['quick_components'], ['ci'])
        self.assertFalse(result['build_required'])
        self.assertEqual(self.plan('games/racing/server/classic_motion.go')['games'], [{'game': 'racing', 'backend': True}])

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


        expected = {'BASKETBALL_PREPARATION_MATRIX', 'BASKETBALL_SELECTION_ADMISSION', 'BASKETBALL_FULL_HTTP'}
        self.assertEqual(scope.verification_env('basketball', True, {}), {'BASKETBALL_FULL_HTTP': '1'})
        self.assertTrue(all(scope.verification_env('basketball', True, {}, extended=True)[key] == '1' for key in expected))
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
        self.assertIn('needs: [plan, quick, frontend, shared, games]', workflow)
        self.assertIn("needs.games.result == 'success' || needs.games.result == 'skipped'", workflow)
        self.assertIn("needs.quick.result == 'success' || needs.quick.result == 'skipped'", workflow)
        self.assertNotIn('  contra_campaigns:', workflow)
        self.assertIn('name: Game ${{ matrix.game }}', workflow)
        self.assertNotIn('matrix.shard', workflow)
        research=(ROOT/'.github/workflows/contra-research.yml').read_text()
        self.assertIn('workflow_dispatch:', research)
        self.assertIn('scripts/contra/campaign_modes.py', research)

    def test_verified_native_build_and_source_container_bind_runtime(self):
        workflow=(ROOT/'.github/workflows/build.yml').read_text().split('  verify:',1)[1]
        steps=['games/contra/build/ci_runtime.py','scripts/cdn_delivery.py upload','scripts/cdn_delivery.py trim','games/contra/build/native.py']
        positions=[workflow.index(step) for step in steps]
        self.assertEqual(positions,sorted(positions))
        self.assertNotIn('scripts/contra/accept_production.py',workflow)
        self.assertIn('--runtime .local/contra-ci-runtime/manifest.json',workflow)
        self.assertIn('--goos linux --goarch amd64',workflow)
        self.assertIn('dist/package/verification-plan.json',workflow)
        self.assertIn('dist/package/wire-board.build.json',workflow)
        self.assertNotIn('scripts/cdn_delivery.py publish',workflow)
        self.assertNotIn('scripts/cdn_delivery.py verify',workflow)
        docker=(ROOT/'Dockerfile').read_text()
        self.assertLess(docker.index('scripts/cdn_delivery.py trim'),docker.index('games/contra/build/native.py'))

    def test_source_container_requires_configured_stable_asset_base(self):
        docker=(ROOT/'Dockerfile').read_text()
        line=next(line for line in docker.splitlines() if line.startswith('RUN node --input-type=module -e '))
        command=shlex.split(line.removeprefix('RUN '))
        origin='https://cdn.example.invalid'
        for source,base,passed in [('', origin+'/bga/', False),('a'*40,origin+'/bga/web/'+'a'*40+'/',False),('a'*40,origin+'/bga/',True),('a'*40,'https://other.example.invalid/bga/',False)]:
            result=subprocess.run(command,cwd=ROOT,env={'PATH':os.environ['PATH'],'SOURCE_SHA':source,'WIRE_BOARD_ASSET_BASE':base,'CDN_ORIGIN':origin},capture_output=True,text=True)
            self.assertEqual(result.returncode==0,passed,(source,base,result.stderr))


class RetiredGameScopeTests(unittest.TestCase):


    def test_retired_table_member_does_not_change_surviving_scenarios(self):
        before = 'func TestTimeoutAutoplayAllGamesContinueToNaturalFinish(t *testing.T) { for _, kind := range []string{"splendor", "dota"} { play(kind) } }'
        after = before.replace(', "dota"', '')
        self.assertEqual(scope.changed_test_source(before, after, retired={'dota'}), '')
        self.assertIn('TestTimeoutAutoplayAllGamesContinueToNaturalFinish', scope.changed_test_source(before, after.replace('play(kind)', 'playNew(kind)'), retired={'dota'}))
    def test_other_strings_are_not_ignored(self):
        before = 'func TestPublic(t *testing.T) { x := "dota" }'
        self.assertIn('TestPublic', scope.changed_test_source(before, before.replace('dota', 'rail'), retired={'dota'}))

if __name__ == '__main__': unittest.main()
