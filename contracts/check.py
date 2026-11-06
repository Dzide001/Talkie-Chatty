"""Validate review schemas and positive/negative client fixtures."""
import copy
import json
from pathlib import Path
from jsonschema import Draft202012Validator, FormatChecker

root = Path(__file__).parent / 'v0.1'
schemas = {}
for path in sorted(root.glob('*.schema.json')):
    schema = json.loads(path.read_text())
    Draft202012Validator.check_schema(schema)
    schemas[path.name] = Draft202012Validator(schema, format_checker=FormatChecker())
v = schemas['client-event.schema.json']
for path in sorted((root / 'fixtures').glob('*.json')):
    fixture = json.loads(path.read_text())
    v.validate(fixture)
    for field in ('burstId', 'roomEpoch', 'requestId'):
        invalid = copy.deepcopy(fixture)
        if field not in invalid:
            continue
        del invalid[field]
        assert not v.is_valid(invalid), f'{path}: missing {field} accepted'
    invalid = dict(fixture, senderId='forged-member')
    assert not v.is_valid(invalid), 'untrusted sender field accepted'
    invalid = dict(fixture, protocolVersion='unknown')
    assert not v.is_valid(invalid), 'unsupported protocol accepted'
join = {'protocolVersion':'0.1-draft.2', 'invite':'fixture-not-a-real-secret-00000000', 'displayName':'Tester', 'platform':'ios', 'appVersion':'0.0.1', 'capabilities':['lan-webrtc-v1','ptt-apns-v1']}
schemas['join.schema.json'].validate(join)
assert not schemas['join.schema.json'].is_valid(dict(join, capabilities=['invented']))
wake = schemas['wake-registration.schema.json']
wake.validate({'provider':'apns-ptt', 'channelId':'12345678-1234-4234-8234-123456789abc', 'environment':'sandbox', 'token':'aabbccdd'})
assert not wake.is_valid({'provider':'fcm','token':'fixture'})
assert not wake.is_valid({'provider':'apns-ptt','token':'aabbccdd'})
ready = json.loads((root/'fixtures/audioReady.json').read_text())
del ready['peerId']
assert not v.is_valid(ready), 'unbound peer readiness accepted'
stop = json.loads((root/'fixtures/localStop.json').read_text())
v.validate(stop)  # No grant is needed for cancellation or repeated stop.
resume = json.loads((root/'fixtures/resume.json').read_text())
assert not v.is_valid(dict(resume, lastServerSequence=-1))
server = schemas['server-event.schema.json']
lease = {'protocolVersion':'0.1-draft.2', 'roomId':'r','roomEpoch':'e','serverSequence':1,'type':'floor.lease','requestId':'q','burstId':'b','grantId':'g','speakerId':'s','remainingLeaseMs':2500}
server.validate(lease)
assert not server.is_valid(dict(lease, remainingLeaseMs=3001))
assert not server.is_valid(dict(lease, type='invented'))
print(f'{len(schemas)} schemas checked; lifecycle fixtures and negative boundary checks passed')
