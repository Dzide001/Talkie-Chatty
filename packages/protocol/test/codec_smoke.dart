import 'dart:convert';
import 'dart:io';

import '../lib/protocol.dart';

void check(bool ok) {
  if (!ok) throw StateError('codec check failed');
}

void rejects(void Function() action) {
  try {
    action();
  } on FormatException {
    return;
  }
  throw StateError('invalid input accepted');
}

void main() {
  for (final file in Directory(
    'contracts/v0.1/fixtures',
  ).listSync().whereType<File>()) {
    final json = jsonDecode(file.readAsStringSync()) as Map<String, dynamic>;
    final model = clientEventFromJson(json);
    check(jsonEncode(model.toJson()) == jsonEncode(json));
    rejects(() => clientEventFromJson({...json, 'senderId': 'forged'}));
  }
  final ready = ClientAudioReady(
    roomId: 'r',
    roomEpoch: 'e',
    requestId: 'q',
    burstId: 'b',
    peerId: 's',
  );
  check(clientEventFromJson(ready.toJson()) is ClientAudioReady);
  final noPeer = ready.toJson()..remove('peerId');
  rejects(() => clientEventFromJson(noPeer));
  final stop = ClientLocalStop(
    roomId: 'r',
    roomEpoch: 'e',
    requestId: 'q',
    burstId: 'b',
    reason: 'USER_RELEASED',
  );
  check(stop.toJson()['reason'] == 'USER_RELEASED');
  rejects(
    () => ClientLocalStop(
      roomId: 'r',
      roomEpoch: 'e',
      requestId: 'q',
      burstId: 'b',
      reason: 'arbitrary',
    ).toJson(),
  );
  rejects(
    () => wakeRegistrationFromJson({'provider': 'fcm', 'token': 'not-real'}),
  );
  final lease = ServerFloorLease(
    roomId: 'r',
    roomEpoch: 'e',
    serverSequence: 1,
    requestId: 'q',
    burstId: 'b',
    grantId: 'g',
    speakerId: 's',
    remainingLeaseMs: 2000,
  );
  check(serverEventFromJson(lease.toJson()) is ServerFloorLease);
  rejects(
    () => serverEventFromJson({...lease.toJson(), 'remainingLeaseMs': 3001}),
  );
  print('Dart codec fixture roundtrips and boundary checks passed');
}
