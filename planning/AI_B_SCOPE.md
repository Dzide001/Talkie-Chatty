# AI B implementation brief

You are Agent B, the mobile application and communications collaborator for Talkie Chatty. Read PROJECT_PLAN.md and Agent A's versioned contract before implementation. This brief is a scoped work order; it does not authorize implementing all phases at once.

## Objective

Deliver one Flutter application for Android and iOS with native lifecycle integrations and direct LAN WebRTC audio. Reliable locked-screen/background reception AND transmission are first-release requirements. Internet control and APNs wake-up traffic is allowed. Users have phones only; no local server is permitted.

## Initial assignment

Implement phase zero only:
1. Inventory real Android/iPhone devices, supported SDKs, dependency/native binary versions, Apple signing/capabilities, and Android permission requirements. Report missing access accurately.
2. Review Agent A's proposed protocol before coding its client. Consume generated models and shared fixtures.
3. Create a minimal Flutter app with flutter_webrtc; a Swift PushToTalk bridge; and a Kotlin communication foreground-service/audio bridge.
4. Join an actual PTT channel, register its real token with Agent A's service, background and lock the iPhone, then prove reception after idle.
5. Prove transmission from the iOS system PTT controls and the chosen valid Android system interaction. Treat receiving and transmitting as different test cases.
6. Coordinate WebRTC with OS-managed audio activation. Prove a one-second utterance spoken after the system's permitted cue survives cold connection setup.
7. If speech is clipped, demonstrate a supported coordination solution or bounded RAM pre-roll using a real audio-injection path. Do not assume WebRTC buffers it, invent an API, keep silent audio running, or insert an unsupported extra iOS activation chime.
8. Apply and verify the local Wi-Fi media policy. No external STUN/TURN, no cellular/VPN audio path, no cloud media fallback. Record actual selected-pair/interface evidence with cellular enabled.
9. Supply device/build IDs, logs, timing observations, failure cases, and a go/no-go report before building the full UI.

The first proof needs real devices. Simulator success is useful for development but does not prove background wake, microphone timing, radio routing, or battery behavior.

## Exclusive implementation ownership

- apps/mobile/**
- packages/ptt_platform/**
- packages/lan_audio/**
- tests/mobile/**
- docs/mobile/**
- App-specific dependency and lock files, native manifests, entitlements, and signing configuration templates

Do not edit services/control, contracts, packages/protocol, infrastructure, tests/system, or root CI/workspace files. Request changes from Agent A. Do not create a parallel contract or backend.

## Later authorized deliverables

B-01: App shell, secure installation credential storage, create/join/leave, QR scan/full link input.
B-02: Signaling client using canonical models; peer graph; deterministic offer handling; reconnect/ICE lifecycle.
B-03: Swift channel join/restoration, token lifecycle, PTT delegate handling, and system-managed audio.
B-04: Kotlin service/notification controls, valid permission/start behavior, audio focus, and supported locked interaction.
B-05: Audio send/playout gating, Wi-Fi interface/candidate verification, route changes, burst limits, and immediate local stop.
B-06: Room roster and real readiness/busy/error states, accessible PTT interface, administrator controls.
B-07: Permission denial/recovery, interruptions, wired/Bluetooth route handling, lifecycle and long-idle tests.
B-08: Redacted diagnostics export, reproducible mobile tests, release builds/instructions, and platform support matrix.

## Required invariants

- Do not grant yourself the floor; only the current server grant authorizes speech.
- Do not show connected, delivered, or ready based on a mock or a push submission.
- Joined membership, socket connected, OS audio active, and media ready are separate states.
- Readiness requires current burst/room generation, established peer/DTLS connection, attached remote audio, and initialized native playout.
- Local release/cancel/interruption/network change stops capture immediately, even if a release message fails.
- An unavailable listener cannot be silently omitted under the first release's all-ready policy.
- A host-type ICE candidate alone does not prove LAN transport. Verify interface and actual route.
- Do not capture microphone audio until user-authorized transmission. Any needed pre-roll is bounded RAM only and is erased on delivery/cancel.
- No certificate bypass, public media relay, recording, background keepalive trick, new account/chat features, or local server dependency.
- Do not remove locked-screen transmit from scope because receiving is easier.

## Tests and completion evidence

Test Android→iOS, iOS→Android, each same-platform pairing, then four mixed phones. Test locked idle at 1, 15, and 60 minutes, native system controls, short first utterance, rapid alternating speakers, simultaneous requests, peer/network loss, cellular and VPN present, guest-network isolation, phone calls, route changes, denied/revoked permissions, explicit leave, force-stop, and restoration behavior.

Supply actual results per device/OS/build with reproducible steps. Do not claim an eight-person capacity without eight real phones. Report battery/memory/thermal observations from a multi-hour session. Preserve failures in the report.

Each handoff includes task ID, contract version, commit ID, changed paths, exact build/test commands, observed results, known failures, and untested assumptions. Propose cross-boundary fixes to Agent A rather than changing server semantics locally.

## Coordination

Use a separate branch/worktree and small daily integration changes. Provide a fake media/platform adapter early for isolated UI tests, but label it clearly. Reuse generated contracts. Any new dependency or native library fork needs provenance, a specific reason, and a documented compatibility decision. Do not invent SDK capabilities to meet a deadline.

