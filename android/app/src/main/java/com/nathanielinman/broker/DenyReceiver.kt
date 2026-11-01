package com.nathanielinman.broker

import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import org.json.JSONObject
import kotlin.concurrent.thread

/** The Deny button on a request's notification. */
class DenyReceiver : BroadcastReceiver() {

    override fun onReceive(ctx: Context, intent: Intent) {
        val id = intent.getStringExtra(ID).orEmpty()
        val e = Store.enrollment(ctx) ?: return
        val pending = goAsync()
        thread {
            try {
                Api(e.origin, e.token).post("/api/reply", JSONObject().put("type", "deny"), readTimeoutMs = 8_000)
                Notifications.clearRequest(ctx, id)
            } catch (err: Exception) {
                Notifications.clearRequest(ctx, id)
                Notifications.showInfo(ctx, "Couldn't deny", err.message ?: "The PC didn't answer.")
            } finally {
                pending.finish()
            }
        }
    }

    companion object {
        private const val ID = "id"

        fun intent(ctx: Context, id: String): PendingIntent = PendingIntent.getBroadcast(
            ctx,
            id.hashCode(),
            Intent(ctx, DenyReceiver::class.java).putExtra(ID, id),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
    }
}
