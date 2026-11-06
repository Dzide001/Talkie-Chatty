package com.talkiechatty.talkie_chatty

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.SystemClock

/**
 * Phase-zero spike: visible ongoing foreground service while the user is in a room.
 * It does NOT capture audio and does NOT grant the floor. Notification actions only
 * record a local request; the server grant (contract v0.1) is not wired yet.
 */
class TalkService : Service() {
    companion object {
        const val ACTION_START = "com.talkiechatty.START"
        const val ACTION_STOP = "com.talkiechatty.STOP"
        const val ACTION_TALK = "com.talkiechatty.TALK_REQUEST"
        const val ACTION_TALK_STOP = "com.talkiechatty.TALK_STOP"
        private const val CHANNEL = "talkie_room"
        private const val NOTIF_ID = 1001

        @Volatile var running = false
        @Volatile var talkRequested = false

        // Idle-test evidence (spike): heartbeat every 10 s, gaps measured on elapsedRealtime
        // so deep-sleep delays show up as a large gap instead of being hidden.
        const val BEAT_MS = 10_000L
        @Volatile var beatCount = 0L
        @Volatile var startedWallMs = 0L
        @Volatile var lastBeatWallMs = 0L
        @Volatile var maxGapMs = 0L
        @Volatile private var lastBeatElapsed = 0L
    }

    private val handler = Handler(Looper.getMainLooper())
    private val beat = object : Runnable {
        override fun run() {
            val now = SystemClock.elapsedRealtime()
            if (lastBeatElapsed != 0L) {
                val gap = now - lastBeatElapsed
                if (gap > maxGapMs) maxGapMs = gap
            }
            lastBeatElapsed = now
            lastBeatWallMs = System.currentTimeMillis()
            beatCount++
            handler.postDelayed(this, BEAT_MS)
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> { stopSelfClean(); return START_NOT_STICKY }
            ACTION_TALK -> talkRequested = true
            ACTION_TALK_STOP -> talkRequested = false
            else -> if (!running) {
                try {
                    enterForeground()
                    running = true
                    beatCount = 0; maxGapMs = 0; lastBeatElapsed = 0
                    startedWallMs = System.currentTimeMillis()
                    handler.post(beat)
                } catch (e: Exception) {
                    // e.g. microphone permission missing or start not allowed: fail visibly
                    running = false
                    stopSelf()
                    return START_NOT_STICKY
                }
            }
        }
        if (running) notifyUpdate()
        return START_NOT_STICKY
    }

    private fun channel() {
        val nm = getSystemService(NotificationManager::class.java)
        if (nm.getNotificationChannel(CHANNEL) == null) {
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL, "Room", NotificationManager.IMPORTANCE_LOW)
            )
        }
    }

    private fun action(label: String, act: String, code: Int): Notification.Action {
        val pi = PendingIntent.getService(
            this, code, Intent(this, TalkService::class.java).setAction(act),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )
        return Notification.Action.Builder(null, label, pi).build()
    }

    private fun build(): Notification {
        channel()
        val text = if (talkRequested) "Talk requested (not yet granted)" else "In room, listening"
        return Notification.Builder(this, CHANNEL)
            .setContentTitle("Talkie Chatty")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_btn_speak_now)
            .setOngoing(true)
            .addAction(action("Talk", ACTION_TALK, 1))
            .addAction(action("Stop talk", ACTION_TALK_STOP, 2))
            .addAction(action("Leave", ACTION_STOP, 3))
            .build()
    }

    private fun enterForeground() {
        val n = build()
        if (Build.VERSION.SDK_INT >= 29) {
            startForeground(NOTIF_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE)
        } else {
            startForeground(NOTIF_ID, n)
        }
    }

    private fun notifyUpdate() {
        getSystemService(NotificationManager::class.java).notify(NOTIF_ID, build())
    }

    private fun stopSelfClean() {
        handler.removeCallbacks(beat)
        running = false
        talkRequested = false
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onDestroy() {
        handler.removeCallbacks(beat)
        running = false
        talkRequested = false
        super.onDestroy()
    }
}
