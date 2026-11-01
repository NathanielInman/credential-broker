package com.nathanielinman.broker

import android.content.Context
import android.content.SharedPreferences
import androidx.core.content.edit

/** Where this phone enrolled: the phone page's origin and the token it shows there. */
data class Enrollment(val origin: String, val token: String)

/**
 * The public half of the Firebase project broker sends notifications
 * through, which the PC hands over at enrollment, so the APK carries no
 * project of its own.
 */
data class FcmSettings(val projectId: String, val senderId: String, val appId: String, val apiKey: String)

/** What the app keeps between runs, in its private preferences. */
object Store {
    private const val ORIGIN = "origin"
    private const val TOKEN = "token"
    private const val FCM_PROJECT = "fcm_project"
    private const val FCM_SENDER = "fcm_sender"
    private const val FCM_APP = "fcm_app"
    private const val FCM_KEY = "fcm_key"
    private const val FCM_TOKEN = "fcm_token"
    private const val FCM_STALE = "fcm_stale"
    private const val DONE = "done"

    private fun prefs(ctx: Context): SharedPreferences =
        ctx.applicationContext.getSharedPreferences("broker", Context.MODE_PRIVATE)

    fun enrollment(ctx: Context): Enrollment? {
        val p = prefs(ctx)
        val origin = p.getString(ORIGIN, null) ?: return null
        val token = p.getString(TOKEN, null) ?: return null
        return Enrollment(origin, token)
    }

    fun fcm(ctx: Context): FcmSettings? {
        val p = prefs(ctx)
        return FcmSettings(
            p.getString(FCM_PROJECT, null) ?: return null,
            p.getString(FCM_SENDER, null) ?: return null,
            p.getString(FCM_APP, null) ?: return null,
            p.getString(FCM_KEY, null) ?: return null,
        )
    }

    /** Keeps an enrollment the PC has finished, with what notifications came through. */
    fun enrolled(ctx: Context, e: Enrollment, fcm: FcmSettings?, fcmToken: String?) {
        prefs(ctx).edit {
            clear()
            putString(ORIGIN, e.origin)
            putString(TOKEN, e.token)
            if (fcm != null && fcmToken != null) {
                putString(FCM_PROJECT, fcm.projectId)
                putString(FCM_SENDER, fcm.senderId)
                putString(FCM_APP, fcm.appId)
                putString(FCM_KEY, fcm.apiKey)
                putString(FCM_TOKEN, fcmToken)
            }
        }
    }

    fun forget(ctx: Context) = prefs(ctx).edit { clear() }

    /** Whether notifications reach this phone: enrolled with a token FCM has not replaced. */
    fun notifying(ctx: Context): Boolean {
        val p = prefs(ctx)
        return p.getString(FCM_TOKEN, null) != null && !p.getBoolean(FCM_STALE, false)
    }

    /**
     * FCM replaced the token the PC has. The PC only learns a token at
     * enrollment, so notifications stop until the phone enrolls again.
     */
    fun newFcmToken(ctx: Context, token: String) {
        val p = prefs(ctx)
        val known = p.getString(FCM_TOKEN, null) ?: return
        if (known != token) p.edit { putBoolean(FCM_STALE, true) }
    }

    fun fcmStale(ctx: Context): Boolean = prefs(ctx).getBoolean(FCM_STALE, false)

    /** Remembers that request [id] is over, in case its "request" message arrives after. */
    fun markDone(ctx: Context, id: String) {
        val p = prefs(ctx)
        val now = System.currentTimeMillis()
        val kept = (p.getStringSet(DONE, emptySet()) ?: emptySet())
            .filter { it.substringAfter(' ').toLongOrNull()?.let { t -> now - t < 60 * 60 * 1000 } == true }
            .toMutableSet()
        kept += "$id $now"
        p.edit { putStringSet(DONE, kept) }
    }

    fun isDone(ctx: Context, id: String): Boolean =
        (prefs(ctx).getStringSet(DONE, emptySet()) ?: emptySet()).any { it.substringBefore(' ') == id }
}
