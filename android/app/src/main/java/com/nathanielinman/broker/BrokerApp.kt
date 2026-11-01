package com.nathanielinman.broker

import android.app.Application
import android.util.Log

class BrokerApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Notifications.createChannels(this)
        // Before any FCM message is handed over, which can start this
        // process with no activity at all.
        runCatching { Push.start(this) }.onFailure { Log.w("broker", "Firebase: ${it.message}") }
    }
}
