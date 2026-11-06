"""Deterministically generate the phase-zero wire review schemas (no network)."""
import copy
import json
from pathlib import Path
ROOT = Path(__file__).parent / 'v0.1'
VERSION = '0.1-draft.2'
ID = {'type':'string','minLength':1,'maxLength':128}
NULL_ID = {'anyOf':[ID,{'type':'null'}]}
SEQ = {'type':'integer','minimum':0}
def enum(*values): return {'enum':list(values)}
def obj(properties, required=None):
    return {'type':'object','additionalProperties':False,'properties':properties,'required':list(properties) if required is None else required}
def save(name, value):
    value={'$schema':'https://json-schema.org/draft/2020-12/schema',**value}
    (ROOT/name).write_text(json.dumps(value,indent=2)+'\n')
base={'protocolVersion':{'const':VERSION},'roomId':ID,'roomEpoch':ID,'requestId':ID}
burst={'burstId':ID}
def client(kind, props): return obj({**base,'type':{'const':kind},**props})
client_events=[
 client('floor.request',{}),client('floor.cancel',burst),client('localStop',burst),
 client('floor.release',{**burst,'grantId':ID}),client('floor.renew',{**burst,'grantId':ID}),client('floor.sync',burst),
 client('wakeAcked',burst),client('audioReady',{**burst,'peerId':ID}),
 client('audioUnavailable',{**burst,'reason':enum('AUDIO_INTERRUPTED','PEER_UNREACHABLE','LAN_DENIED','MIC_DENIED','NETWORK_CHANGED','PLAYOUT_FAILED')}),
 client('audioInterrupted',{'burstId':NULL_ID,'reason':enum('FOCUS_LOST','CALL','ROUTE_LOST')}),
 client('audioResumed',{}),
 client('networkChanged',{'burstId':NULL_ID,'reason':enum('WIFI_LOST','INTERFACE_CHANGED')}),
 client('permissionState',{'burstId':NULL_ID,'permission':enum('microphone','localNetwork'),'state':enum('granted','denied')}),
 client('resume',{'lastServerSequence':SEQ}),
]
for kind in ('signal.offer','signal.answer'):
 client_events.append(client(kind,{**burst,'recipientId':ID,'sdp':{'type':'string','minLength':1,'maxLength':32768}}))
client_events.append(client('signal.ice',{**burst,'recipientId':ID,'candidate':{'type':'string','maxLength':2048},'sdpMid':{'type':['string','null'],'maxLength':128},'sdpMLineIndex':{'type':['integer','null'],'minimum':0,'maximum':64}}))
stop_reason=enum('USER_RELEASED','AUDIO_INTERRUPTED','MIC_DENIED','LAN_DENIED','NETWORK_CHANGED','PLAYOUT_FAILED','PEER_UNREACHABLE')
for item in client_events:
    if item['properties']['type']['const'] in ('floor.release','floor.cancel','localStop'):
        item['properties']['reason']=stop_reason
save('client-event.schema.json',{'title':'Client events — draft.2','oneOf':client_events})
metadata={'protocolVersion':{'const':VERSION},'displayName':{'type':'string','minLength':1,'maxLength':48},'platform':enum('ios','android'),'appVersion':{'type':'string','minLength':1,'maxLength':64},'capabilities':{'type':'array','uniqueItems':True,'maxItems':16,'items':enum('lan-webrtc-v1','ptt-apns-v1','android-joined-service-v1')}}
secret={'type':'string','minLength':32,'maxLength':256}
save('create.schema.json',obj(metadata))
save('join.schema.json',obj({**metadata,'invite':secret}))
member=obj({'participantId':ID,'displayName':metadata['displayName'],'platform':metadata['platform'],'controlConnected':{'type':'boolean'},'reachability':enum('online','wakeable','unreachable')})
members={'type':'array','maxItems':4,'items':member}
floor_state=obj({'phase':enum('idle','preparing','active'),'burstId':NULL_ID,'grantId':NULL_ID,'speakerId':NULL_ID,'recipientIds':{'type':'array','uniqueItems':True,'maxItems':3,'items':ID},'remainingMs':{'type':'integer','minimum':0,'maximum':5000}})
boot={'protocolVersion':{'const':VERSION},'roomId':ID,'roomEpoch':ID,'participantId':ID,'memberCredential':secret,'credentialExpiresAt':{'type':'string','format':'date-time'},'serverSequence':SEQ,'members':members}
save('join-response.schema.json',obj(boot))
save('create-response.schema.json',obj({**boot,'adminCredential':secret,'invite':secret,'inviteExpiresAt':{'type':'string','format':'date-time'}}))
save('wake-registration.schema.json',{'oneOf':[obj({'provider':{'const':'apns-ptt'},'channelId':{'type':'string','format':'uuid'},'environment':enum('sandbox','production'),'token':{'type':'string','minLength':2,'maxLength':1024,'pattern':'^(?:[0-9a-fA-F]{2})+$'}}),obj({'provider':{'const':'none'}})]})
sbase={k:v for k,v in base.items() if k!='requestId'}
sbase['serverSequence']={'type':'integer','minimum':1}
def server(kind,props): return obj({**sbase,'type':{'const':kind},**props})
reasons=enum('RELEASED','CANCELLED','LOCAL_STOP','PREPARE_TIMEOUT','LEASE_EXPIRED','BURST_LIMIT','MEMBER_LEFT','MEMBER_REMOVED','SPEAKER_DISCONNECTED','AUDIO_INTERRUPTED','NETWORK_CHANGED','PERMISSION_REVOKED','AUDIO_UNAVAILABLE','EPOCH_CHANGED','NO_RECIPIENTS','PLAYOUT_FAILED','MIC_DENIED','LAN_DENIED')
errors=enum('UNAUTHORIZED','PROTOCOL_MISMATCH','CAPABILITY_UNSUPPORTED','BUSY','NO_RECIPIENTS','STALE_BURST','NOT_RECIPIENT','INVALID_MESSAGE','WAKE_PROVIDER_UNSUPPORTED','RATE_LIMITED','ROOM_FULL','INVITE_EXPIRED','EPOCH_MISMATCH','REQUEST_ID_REUSED','PEER_UNREACHABLE')
server_events=[
 server('floor.reserved',{**burst,'speakerId':ID,'recipientIds':{'type':'array','minItems':1,'maxItems':3,'uniqueItems':True,'items':ID},'prepareTimeoutMs':{'const':5000}}),
 server('floor.granted',{**burst,'grantId':ID,'speakerId':ID,'leaseMs':{'const':3000},'maxBurstMs':{'const':30000}}),
 server('floor.lease',{'requestId':ID,**burst,'grantId':ID,'speakerId':ID,'remainingLeaseMs':{'type':'integer','minimum':0,'maximum':3000}}),
 server('floor.revoked',{**burst,'reason':reasons}),
 server('roster',{'members':members,'floor':floor_state}),
 server('state.snapshot',{'requestId':ID,'participantId':ID,'members':members,'floor':floor_state}),
 server('epochChanged',{'requestId':ID,'previousEpoch':ID,'reauthenticate':{'const':True}}),
 server('ack',{'requestId':ID}),server('error',{'requestId':ID,'code':errors}),
]
for item in client_events:
 if item['properties']['type']['const'].startswith('signal.'):
  props=copy.deepcopy(item['properties']);props.pop('requestId');props['senderId']=ID;props['serverSequence']=sbase['serverSequence']
  server_events.append(obj(props))
save('server-event.schema.json',{'title':'Server events — draft.2','oneOf':server_events})
# Keep the two original client fixtures in place; add focused lifecycle examples.
fixture_base={'protocolVersion':VERSION,'roomId':'room-fixture','roomEpoch':'epoch-fixture','requestId':'request-fixture'}
for kind,extra in [('wakeAcked',{'burstId':'burst-fixture'}),('audioReady',{'burstId':'burst-fixture','peerId':'speaker-fixture'}),('localStop',{'burstId':'burst-fixture'}),('resume',{'lastServerSequence':12}),('networkChanged',{'burstId':None,'reason':'WIFI_LOST'})]:
 (ROOT/'fixtures'/f'{kind}.json').write_text(json.dumps({**fixture_base,'type':kind,**extra},indent=2)+'\n')
