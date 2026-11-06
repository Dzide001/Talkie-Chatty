# Talkie protocol Dart models

Generated typed codecs for 0.1-draft.2. No Flutter or third-party runtime dependencies. AI B may review/import this library using a local path dependency once the draft is agreed; it is not a network client and no backend endpoint is live.

From the repository root:

    python3 contracts/build.py
    python3 contracts/generate_dart.py
    dart format packages/protocol/lib/protocol.dart
    dart analyze packages/protocol
    dart packages/protocol/test/codec_smoke.dart

Do not edit generated protocol.dart directly. Update contracts/build.py and the code generator, regenerate and review the schema/model diff together.

Types include ClientAudioReady, ClientLocalStop, ClientResume, ServerFloorLease, ServerRoster, CreateRequest/Response and JoinRequest/Response. clientEventFromJson/serverEventFromJson dispatch by event type. Codecs reject unknown fields, missing required fields, wrong types/versions, unknown enums, and bounded collection/string/number violations. JSON Schema remains the wire specification; authorization, byte budgets, timing, peer membership and stateful validity must be enforced by the real client/server adapters.

Models do not establish microphone permission, actual audio readiness, media encryption, or lease validity. The README in contracts/v0.1 defines the proposed native gate and monotonic lease-proof rule that AI B must review.
