import uuid


def audit_waiting(browser, evaluate, request, room, wait_for, screenshot):
    samples = []
    for width, height in [(1280, 900), (890, 626), (626, 890), (320, 760)]:
        created = request('main', '/rooms', {'kind': 'cs16', 'name': '准备区域稳定性验收', 'capacity': 10})
        browser('main', 'set', 'viewport', width, height)
        browser('main', 'reload')
        wait_for(lambda: evaluate('main', '!!document.querySelector(".waiting-actions button")'))
        browser('main', 'scrollintoview', '[data-add-bot], .waiting-actions > button:first-child')
        origin = None
        for expected in range(1, 11):
            wait_for(lambda: len(room()['seats']) == expected)
            wait_for(lambda: evaluate('main', 'document.querySelectorAll(".waiting-seat.filled").length') == expected)
            geometry = evaluate('main', '(()=>{const button=document.querySelector("[data-add-bot], .waiting-actions > button:first-child");const rect=button?.getBoundingClientRect();return {scroll:scrollY,x:rect?.x,y:rect?.y,width:rect?.width,height:rect?.height,disabled:button?.disabled,cards:[...document.querySelectorAll(".waiting-seat")].map(e=>e.getBoundingClientRect().height),overflow:document.documentElement.scrollWidth>innerWidth}})()')
            assert geometry['x'] is not None, '添加电脑按钮消失'
            assert not geometry['overflow'], geometry
            assert max(geometry['cards']) <= 72, geometry
            if origin is None:
                origin = geometry
            assert abs(geometry['scroll'] - origin['scroll']) <= 1 and abs(geometry['y'] - origin['y']) <= 1, {'before': origin, 'after': geometry}
            samples.append({'viewport': [width, height], 'players': expected, 'geometry': geometry})
            if expected < 10:
                browser('main', 'click', '[data-add-bot], .waiting-actions > button:first-child')
            else:
                assert geometry['disabled'], geometry
        screenshot('main', 'waiting-stable-' + str(width))
        current = room()
        request('main', '/rooms/' + created['id'], {'type': 'leave', 'version': current['version'], 'nonce': uuid.uuid4().hex})
        wait_for(lambda: room() is None)
    return samples
