"""Natural base-game driver for reserved-card, noble and results-page E2E checks.

Failure cases: panel-origin jump, noble flight follows the replacement slot,
wrong player/score/payment, or settled effects replay after a page reload.
All choices use that player's public/private HTTP response, never a fixture.
"""
import time


def run_acquisition(room, command, observe, browser, js, report, out, base):
    verified_reserved = verified_noble = False
    trace = report['matchTrace'] = []
    for step in range(600):
        r = room()
        g, actor = r['game'], r['game']['turn']
        sg = g['splendor']
        if g['finished']:
            assert verified_reserved and verified_noble, 'Match ended before both acquisition paths'
            report['finalScores'] = [p['score'] for p in sg['players']]
            report['winners'] = g['winners']
            browser('open', base)
            time.sleep(.5)
            page = js('({text:document.body.innerText,winner:document.querySelector(".results h2")?.textContent,flights:document.querySelectorAll("[data-card-flight],[data-token-flight]").length,overflow:document.documentElement.scrollWidth>innerWidth})')
            assert page['winner'].strip() == '、'.join(r['seats'][p]['name'] for p in g['winners']) + ' 获胜', page
            assert '获胜' in page['text'] and not page['overflow'], page
            assert page['flights'] == 0, 'Reload replayed finished effects'
            browser('screenshot', str(out / 'finished.png'))
            return
        own = room(actor)['game']['splendor']['players'][actor]
        tokens, bonus = own['tokens'], own['bonus']
        reserved = own.get('reserved') or []
        cards = [c for row in sg['market'] for c in row if c['id'] > 0] + reserved
        nobles = sg['nobles']

        def deficit(c):
            return [max(0, c['cost'][i] - bonus[i] - tokens[i]) for i in range(5)]

        goal = min(nobles, key=lambda n: sum(max(0, n['cost'][i] - bonus[i]) for i in range(5))) if nobles else None

        def priority(c):
            needed = goal and bonus[c['color']] < goal['cost'][c['color']]
            return (0 if needed else 1, sum(max(0, c['cost'][i] - bonus[i]) for i in range(5)), c['points'])

        chosen = None
        if g['phase'] == 'discard':
            give, due = [0] * 6, sum(tokens) - 10
            for i in sorted(range(5), key=lambda i: tokens[i], reverse=True):
                give[i] = min(tokens[i], due)
                due -= give[i]
            action = dict(type='discard', tokens=give)
        elif g['phase'] == 'noble':
            noble = next(n for n in nobles if all(bonus[i] >= n['cost'][i] for i in range(5)))
            action = dict(type='noble', noble=noble['id'])
        else:
            forced = reserved if actor == 0 and not verified_reserved else []
            affordable = [c for c in (forced or cards) if sum(deficit(c)) <= tokens[5]]
            if actor == 0 and not verified_reserved and not reserved:
                chosen = min(sg['market'][0], key=priority)
                action = dict(type='reserve', card=chosen['id'])
            elif affordable:
                desired = [c for c in affordable if c in reserved] if actor == 0 and not verified_reserved else []
                chosen = min(desired or affordable, key=priority)
                action = dict(type='buy', card=chosen['id'])
            else:
                chosen = min(forced or cards, key=lambda c: (sum(deficit(c)), priority(c)))
                need = deficit(chosen)
                colors = sorted([i for i in range(5) if sg['bank'][i] > 0], key=lambda i: (-need[i], -sg['bank'][i]))
                take = [0] * 6
                if colors and sg['bank'][colors[0]] >= 4 and need[colors[0]] >= 2 and sum(need[i] > 0 for i in colors) == 1:
                    take[colors[0]] = 2
                else:
                    for i in colors[:3]:
                        take[i] = 1
                if sum(take):
                    action = dict(type='take', tokens=take)
                elif len(reserved) < 3:
                    action = dict(type='reserve', card=min(sg['market'][0], key=priority)['id'])
                else:
                    action = dict(type='pass')
        gaining = []
        if action['type'] == 'buy':
            after_bonus = bonus.copy()
            after_bonus[chosen['color']] += 1
            gaining = [n for n in nobles if all(after_bonus[i] >= n['cost'][i] for i in range(5))]
        buying_reserved = actor == 0 and action['type'] == 'buy' and chosen in reserved and not verified_reserved
        getting_noble = (len(gaining) == 1 or action['type'] == 'noble') and not verified_noble
        trace.append(dict(actor=actor, action=action))
        if buying_reserved or getting_noble:
            result = observe('reserved-buy' if buying_reserved else 'noble', actor, action, panel=buying_reserved)
            after = room()['game']['splendor']
            p = after['players'][actor]
            gained = len(p['nobles']) - len(own['nobles'])
            assert p['score'] == own['score'] + (chosen['points'] if action['type'] == 'buy' else 0) + 3 * gained
            if action['type'] == 'buy':
                payment = [min(tokens[i], max(0, chosen['cost'][i] - bonus[i])) for i in range(5)]
                payment.append(sum(deficit(chosen)))
                assert [tokens[i] - p['tokens'][i] for i in range(6)] == payment
                assert [after['bank'][i] - sg['bank'][i] for i in range(6)] == payment
                assert any(c['id'] == chosen['id'] for c in p['cards'])
                result['payment'] = payment
            if buying_reserved:
                assert all(c['id'] != chosen['id'] for c in p.get('reserved') or [])
                assert 'buy' in result['visibleCardActions']
                verified_reserved = True
            if getting_noble:
                assert gained == 1 and 'noble' in result['visibleCardActions']
                verified_noble = True
            result['score'] = dict(before=own['score'], after=p['score'], nobles=gained)
        else:
            command(actor, 'action', action=action)
    raise AssertionError('Natural match did not finish within 600 actions')
