"""Reach natural solo/co-op stage-one starts without changing original memory."""
import json

from capture_reference import BASE, run


def main():
    scenarios = BASE / 'scenarios'
    scenarios.mkdir(parents=True, exist_ok=True)
    for mode in ('solo', 'coop'):
        segments = [{'start':120,'end':122,'keys':['start']},
                    {'start':720,'end':722,'keys':['start']}]
        if mode == 'coop':
            segments.insert(0, {'start':100,'end':102,'keys':['select']})
        baseline = {'name':mode+'-baseline', 'frames':1240, 'segments':segments,
                    'captures':[1090,1120,1150,1180,1200,1240], 'save_states':{'1200':mode+'-start'}}
        entry = {'name':mode+'-play-entry', 'load_state':mode+'-start', 'frames':240,
                 'segments':[{'start':1,'end':3,'keys':['start']}],
                 'captures':[60,120,180,200,240], 'save_states':{'200':mode+'-play'}}
        for spec in (baseline, entry):
            path = scenarios / (spec['name']+'.json')
            path.write_text(json.dumps(spec, indent=2)+'\n')
            run(path)


if __name__ == '__main__':
    main()
