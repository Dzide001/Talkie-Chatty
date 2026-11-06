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
        del invalid[field]
        assert not v.is_valid(invalid), f'{path}: missing {field} accepted'
    invalid = dict(fixture, senderId='forged-member')
    assert not v.is_valid(invalid), 'untrusted sender field accepted'
    invalid = dict(fixture, protocolVersion='unknown')
    assert not v.is_valid(invalid), 'unsupported protocol accepted'
join = {'protocolVersion':'0.1-draft.1', 'invite':'fixture-not-a-real-secret-00000000', 'displayName':'Tester', 'platform':'ios', 'appVersion':'0.0.1', 'capabilities':['lan-webrtc-v1','ptt-apns-v1']}
schemas['join.schema.json'].validate(join)
assert not schemas['join.schema.json'].is_valid(dict(join, capabilities=['invented']))
wake = schemas['wake-registration.schema.json']
wake.validate({'provider':'apns-ptt', 'channelId':'12345678-1234-4234-8234-123456789abc', 'environment':'sandbox', 'token':'aabbccdd'})
assert not wake.is_valid({'provider':'fcm','token':'fixture'})
assert not wake.is_valid({'provider':'apns-ptt','token':'aabbccdd'})
print(f'{len(schemas)} schemas checked; readiness fixtures and negative boundary checks passed')
