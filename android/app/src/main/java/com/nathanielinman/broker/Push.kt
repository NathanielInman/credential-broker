package com.nathanielinman.broker

import android.content.Context
import com.google.firebase.FirebaseApp
import com.google.firebase.FirebaseOptions
import com.google.firebase.messaging.FirebaseMessaging
import kotlinx.coroutines.tasks.await

/**
 * Firebase, set up at run time from the settings the PC gave at
 * enrollment rather than from a google-services.json built in.
 */
object Push {
    /** Starts Firebase with [settings], if it is not running yet. */
    fun start(ctx: Context, settings: FcmSettings? = Store.fcm(ctx)): Boolean {
        if (settings == null) return false
        val running = FirebaseApp.getApps(ctx).firstOrNull()
        if (running != null) {
            if (running.options.projectId != settings.projectId) {
                throw IllegalStateException("broker is using another Firebase project. Close broker (swipe it away) and scan again.")
            }
            return true
        }
        FirebaseApp.initializeApp(
            ctx,
            FirebaseOptions.Builder()
                .setProjectId(settings.projectId)
                .setGcmSenderId(settings.senderId)
                .setApplicationId(settings.appId)
                .setApiKey(settings.apiKey)
                .build(),
        )
        return true
    }

    /** This install's FCM token in [settings]'s project. */
    suspend fun token(ctx: Context, settings: FcmSettings): String {
        start(ctx, settings)
        return FirebaseMessaging.getInstance().token.await()
    }
}
