package com.nathanielinman.broker

import org.json.JSONArray
import org.json.JSONObject
import java.time.Instant
import java.time.OffsetDateTime

/**
 * One secret a request asks for. [changed] means it is still granted, but its value changed in the
 * vault since, so the grant holds the old one and the request is for the new value.
 */
data class Secret(val name: String, val description: String, val changed: Boolean = false)

/** A grant request, as broker shows it in the desktop window. */
data class Request(
    val secrets: List<Secret>,
    /** The duration asked for, and the ones the human may pick instead, in seconds. */
    val forSecs: Long,
    val choices: List<Long>,
    val deadline: Instant,
    /** The requesting process chain, nearest first, read by broker from /proc. */
    val ancestry: List<String>,
    val cwd: String,
    val home: String,
    val command: List<String>,
    /** The agent's own word, unverified. */
    val reason: String,
) {
    /** The process chain worth showing: no broker, no repeats, nothing from the session up. */
    fun chain(): List<String> {
        val own = ancestry.drop(if (ancestry.firstOrNull() == "broker") 1 else 0)
        val full = own.filterIndexed { i, c -> i == 0 || c != own[i - 1] }
        val root = full.indexOfFirst { it in sessionRoots }
        val chain = if (root > 0) full.take(root) else full
        return if (chain.size > 6) chain.take(6) + "…" else chain
    }

    /** Why a request asks again for secrets that are still granted, as the window says it, or null. */
    fun changedNote(): String? {
        val names = secrets.filter { it.changed }.map { it.name }
        if (names.isEmpty()) return null
        val one = names.size == 1
        val list = if (names.size < 3) names.joinToString(" and ") else names.dropLast(1).joinToString(", ") + ", and " + names.last()
        return "Asking again: $list ${if (one) "is" else "are"} still granted, but the value changed in the vault since, " +
            "so the grant holds the old one. Approving grants the new ${if (one) "value" else "values"}."
    }

    /** [path] with the home directory shortened to ~. */
    fun tilde(path: String): String = when {
        home.isEmpty() || path.isEmpty() -> path
        path == home -> "~"
        else -> path.replace("$home/", "~/")
    }

    companion object {
        fun parse(o: JSONObject): Request = Request(
            secrets = o.optJSONArray("secrets").objects().map {
                Secret(it.optString("name"), it.optString("description"), it.optBoolean("changed"))
            },
            forSecs = o.optLong("for"),
            choices = o.optJSONArray("choices").longs(),
            deadline = runCatching { OffsetDateTime.parse(o.optString("deadline")).toInstant() }
                .getOrDefault(Instant.now().plusSeconds(60)),
            ancestry = o.optJSONArray("ancestry").strings(),
            cwd = o.optString("cwd"),
            home = o.optString("home"),
            command = o.optJSONArray("command").strings(),
            reason = o.optString("reason"),
        )

        /** Where a process chain stops being the agent's and becomes the desktop's. */
        private val sessionRoots = setOf(
            "systemd", "init", "login", "agetty", "launchd", "loginwindow",
            "Hyprland", "start-hyprland", "sway", "niri", "river", "labwc", "wayfire", "hikari", "dwl", "Xorg", "Xwayland",
            "gnome-shell", "gnome-session-b", "gdm-wayland-ses", "gdm-x-session", "gdm-session-wor",
            "kwin_wayland", "kwin_x11", "plasmashell", "startplasma-way", "startplasma-x11", "ksmserver", "sddm-helper",
            "lightdm", "xfce4-session", "cinnamon-sessio", "mate-session", "i3", "bspwm", "awesome", "openbox",
        )
    }
}

fun JSONArray?.objects(): List<JSONObject> =
    if (this == null) emptyList() else (0 until length()).mapNotNull { optJSONObject(it) }

fun JSONArray?.strings(): List<String> =
    if (this == null) emptyList() else (0 until length()).map { optString(it) }

fun JSONArray?.longs(): List<Long> =
    if (this == null) emptyList() else (0 until length()).map { optLong(it) }

/** A duration in seconds the way the desktop window says it. */
fun human(secs: Long): String {
    if (secs % 3600 == 0L) {
        val h = secs / 3600
        return "$h ${if (h == 1L) "hour" else "hours"}"
    }
    if (secs < 3600 && secs % 60 == 0L) return "${secs / 60} min"
    val h = secs / 3600
    val m = Math.round((secs % 3600) / 60.0)
    return (if (h > 0) "${h}h " else "") + "${m}m"
}

/** argv as a shell would need it typed. */
fun quote(argv: List<String>): String = argv.joinToString(" ") { a ->
    if (Regex("^[\\w@%+=:,./-]+$").matches(a)) a else "'" + a.replace("'", "'\\''") + "'"
}
