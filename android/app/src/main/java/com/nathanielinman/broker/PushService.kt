package com.nathanielinman.broker

import android.util.Log
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import java.time.Instant

/**
 * FCM's messages from broker. They only say that something happened and
 * how many secrets a request wants (Google can read them); the details
 * come from the PC itself.
 */
class PushService : FirebaseMessagingService() {

    override fun onMessageReceived(message: RemoteMessage) {
        val d = message.data
        val id = d["id"].orEmpty()
        when (d["type"]) {
            "request" -> {
                if (id.isNotEmpty() && Store.isDone(this, id)) return
                val deadline = runCatching { Instant.parse(d["deadline"]) }.getOrDefault(Instant.now().plusSeconds(60))
                Notifications.showRequest(this, id, d["count"]?.toIntOrNull() ?: 1, deadline, details())
                Events.pushed.tryEmit(Unit)
            }
            "done" -> {
                if (id.isNotEmpty()) {
                    Store.markDone(this, id)
                    Notifications.clearRequest(this, id)
                }
                Events.pushed.tryEmit(Unit)
            }
            "enrolled" -> Notifications.showInfo(this, "Phone enrolled", "broker's requests will show up here.")
        }
    }

    /** The waiting request from the PC, if it answers quickly. */
    private fun details(): Request? {
        val e = Store.enrollment(this) ?: return null
        return runCatching {
            val poll = Api(e.origin, e.token).get("/api/poll?after=0", readTimeoutMs = 5_000)
            poll.optJSONArray("messages").objects().firstOrNull { it.optString("type") == "request" }
                ?.optJSONObject("request")?.let(Request::parse)
        }.onFailure { Log.i("broker", "request details: ${it.message}") }.getOrNull()
    }

    override fun onNewToken(token: String) {
        Store.newFcmToken(this, token)
    }
}
