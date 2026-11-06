import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:permission_handler/permission_handler.dart';

// Phase-zero spike UI. Not the product UI. No backend, no floor grant, no audio yet.
void main() => runApp(const SpikeApp());

class SpikeApp extends StatelessWidget {
  const SpikeApp({super.key});
  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'Talkie Chatty (spike)',
        theme: ThemeData(colorSchemeSeed: Colors.teal, useMaterial3: true),
        home: const SpikePage(),
      );
}

class SpikePage extends StatefulWidget {
  const SpikePage({super.key});
  @override
  State<SpikePage> createState() => _SpikePageState();
}

class _SpikePageState extends State<SpikePage> with WidgetsBindingObserver {
  static const _ch = MethodChannel('talkie/platform');
  Map<String, dynamic> _status = {};
  String _message = 'Not in a room.';
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _poll = Timer.periodic(const Duration(seconds: 1), (_) => _refresh());
    _refresh();
  }

  @override
  void dispose() {
    _poll?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  String _ago(Object? ms) {
    final v = (ms as num?)?.toInt() ?? 0;
    if (v == 0) return '-';
    final d = DateTime.now().millisecondsSinceEpoch - v;
    return '${(d / 1000).round()} s ago';
  }

  String _since() {
    final v = (_status['startedWallMs'] as num?)?.toInt() ?? 0;
    if (v == 0 || _status['serviceRunning'] != true) return '-';
    final s = (DateTime.now().millisecondsSinceEpoch - v) ~/ 1000;
    return '${s ~/ 60} min ${s % 60} s';
  }

  Future<void> _refresh() async {
    try {
      final s = await _ch.invokeMapMethod<String, dynamic>('status');
      if (mounted) setState(() => _status = s ?? {});
    } on PlatformException catch (_) {}
  }

  Future<void> _join() async {
    final mic = await Permission.microphone.request();
    if (!mic.isGranted) {
      setState(() => _message = 'MIC_DENIED: microphone permission is required.');
      return;
    }
    await Permission.notification.request();
    await _ch.invokeMethod('startService');
    await Future<void>.delayed(const Duration(milliseconds: 500));
    await _refresh();
    setState(() => _message = _status['serviceRunning'] == true
        ? 'Foreground service running (local only, not connected to any room).'
        : 'Service failed to start.');
  }

  Future<void> _leave() async {
    await _ch.invokeMethod('stopService');
    await Future<void>.delayed(const Duration(milliseconds: 300));
    await _refresh();
    setState(() => _message = 'Left.');
  }

  @override
  Widget build(BuildContext context) {
    final running = _status['serviceRunning'] == true;
    return Scaffold(
      appBar: AppBar(title: const Text('Talkie Chatty (phase-0 spike)')),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text('Service: ${running ? "RUNNING" : "stopped"}'),
          Text('Talk requested via notification: ${_status['talkRequested'] == true}'),
          Text('Battery optimisation ignored: ${_status['batteryOptimisationIgnored']}'),
          const SizedBox(height: 8),
          Text('Heartbeats (every 10 s): ${_status['beatCount'] ?? 0}'),
          Text('Running for: ${_since()}'),
          Text('Last beat: ${_ago(_status['lastBeatWallMs'])}'),
          Text('Largest gap between beats: ${_status['maxGapMs'] ?? 0} ms'),
          const SizedBox(height: 12),
          Text(_message),
          const Spacer(),
          FilledButton(onPressed: running ? null : _join, child: const Text('Start foreground service')),
          const SizedBox(height: 8),
          OutlinedButton(onPressed: running ? _leave : null, child: const Text('Stop')),
        ]),
      ),
    );
  }
}
