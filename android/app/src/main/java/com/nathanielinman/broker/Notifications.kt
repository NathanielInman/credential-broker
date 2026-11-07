package com.nathanielinman.broker

import android.Manifest
import android.annotation.SuppressLint
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import kotlinx.coroutines.flow.MutableSharedFlow
import java.time.Instant

/** News from FCM for a running activity: something about a request changed. */
object Events {
    val pushed = MutableSharedFlow<Unit>(extraBufferCapacity = 1)
}

object Notifications {
    private const val REQUESTS = "requests"
    private const val INFO = "info"
    private const val REQUEST_ID = 1
    private const val INFO_ID = 2

    fun createChannels(ctx: Context) {
        val nm = ctx.getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(REQUESTS, ctx.getString(R.string.channel_requests), NotificationManager.IMPORTANCE_HIGH).apply {
                description = ctx.getString(R.string.channel_requests_desc)
                // The secret names are on the private version only.
                lockscreenVisibility = NotificationCompat.VISIBILITY_PRIVATE
            },
        )
        nm.createNotificationChannel(
            NotificationChannel(INFO, ctx.getString(R.string.channel_info), NotificationManager.IMPORTANCE_DEFAULT).apply {
                description = ctx.getString(R.string.channel_info_desc)
            },
        )
    }

    fun enabled(ctx: Context): Boolean = allowed(ctx)

    private fun allowed(ctx: Context): Boolean =
        (Build.VERSION.SDK_INT < 33 ||
            ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) &&
            NotificationManagerCompat.from(ctx).areNotificationsEnabled()

    /** Opens the app, which waits on the key for an open request. */
    private fun open(ctx: Context): PendingIntent {
        val intent = Intent(ctx, MainActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        return PendingIntent.getActivity(ctx, 0, intent, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
    }

    /**
     * Shows request [id], for [count] secrets until [deadline]. With the
     * request's [details] (fetched from the PC, never from FCM) the
     * unlocked phone shows what is asked; the lock screen only ever says
     * how many.
     */
    @SuppressLint("MissingPermission")
    fun showRequest(ctx: Context, id: String, count: Int, deadline: Instant, details: Request?) {
        if (!allowed(ctx)) return
        val n = details?.secrets?.size ?: count
        val generic = if (n > 1) "An agent is asking for $n secrets." else "An agent is asking for a secret."
        val left = deadline.toEpochMilli() - System.currentTimeMillis()
        if (left <= 0) return
        val public = NotificationCompat.Builder(ctx, REQUESTS)
            .setSmallIcon(R.drawable.ic_key)
            .setColor(ctx.getColor(R.color.brand))
            .setContentTitle("broker")
            .setContentText(generic)
            .build()
        val b = NotificationCompat.Builder(ctx, REQUESTS)
            .setSmallIcon(R.drawable.ic_key)
            .setColor(ctx.getColor(R.color.brand))
            .setCategory(NotificationCompat.CATEGORY_REMINDER)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
            .setPublicVersion(public)
            .setContentIntent(open(ctx))
            .setAutoCancel(true)
            .setOnlyAlertOnce(false)
            .setWhen(deadline.toEpochMilli())
            .setUsesChronometer(true)
            .setChronometerCountDown(true)
            .setTimeoutAfter(left)
            .addAction(R.drawable.ic_key, "Approve", open(ctx))
            .addAction(R.drawable.ic_close, "Deny", DenyReceiver.intent(ctx, id))
        if (details == null) {
            b.setContentTitle("broker").setContentText(generic)
        } else {
            val names = details.secrets.joinToString(", ") { it.name }
            val title = if (n == 1) "Agent wants ${details.secrets[0].name}" else "Agent wants $n secrets"
            val lines = buildList {
                if (n > 1) add(names)
                add("For ${human(details.forSecs)}")
                if (details.reason.isNotEmpty()) add("“${details.reason}”")
                details.chain().takeIf { it.isNotEmpty() }?.let { add("Asked by " + it.joinToString(" < ")) }
            }
            b.setContentTitle(title)
                .setContentText(lines.first())
                .setStyle(NotificationCompat.BigTextStyle().bigText(lines.joinToString("\n")))
        }
        NotificationManagerCompat.from(ctx).notify(id, REQUEST_ID, b.build())
    }

    /** Drops request [id]'s notification, or every request's without one. */
    fun clearRequest(ctx: Context, id: String? = null) {
        val nm = ctx.getSystemService(NotificationManager::class.java)
        if (id != null) {
            nm.cancel(id, REQUEST_ID)
            return
        }
        nm.activeNotifications.filter { it.id == REQUEST_ID }.forEach { nm.cancel(it.tag, it.id) }
    }

    @SuppressLint("MissingPermission")
    fun showInfo(ctx: Context, title: String, text: String) {
        if (!allowed(ctx)) return
        val b = NotificationCompat.Builder(ctx, INFO)
            .setSmallIcon(R.drawable.ic_key)
            .setColor(ctx.getColor(R.color.brand))
            .setContentTitle(title)
            .setContentText(text)
            .setContentIntent(open(ctx))
            .setAutoCancel(true)
        NotificationManagerCompat.from(ctx).notify(INFO_ID, b.build())
    }
}
