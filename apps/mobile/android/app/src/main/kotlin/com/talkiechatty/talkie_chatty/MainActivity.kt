package com.talkiechatty.talkie_chatty

import android.content.Intent
import android.os.PowerManager
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, "talkie/platform")
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "startService" -> {
                        startForegroundService(
                            Intent(this, TalkService::class.java).setAction(TalkService.ACTION_START)
                        )
                        result.success(null)
                    }
                    "stopService" -> {
                        startService(
                            Intent(this, TalkService::class.java).setAction(TalkService.ACTION_STOP)
                        )
                        result.success(null)
                    }
                    "status" -> {
                        val pm = getSystemService(PowerManager::class.java)
                        result.success(
                            mapOf(
                                "serviceRunning" to TalkService.running,
                                "talkRequested" to TalkService.talkRequested,
                                "batteryOptimisationIgnored" to
                                    pm.isIgnoringBatteryOptimizations(packageName)
                            )
                        )
                    }
                    else -> result.notImplemented()
                }
            }
    }
}
