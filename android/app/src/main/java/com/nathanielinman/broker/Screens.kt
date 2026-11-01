package com.nathanielinman.broker

import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import java.time.Instant

/** What the screens can ask of the activity. */
class Actions(
    val scan: () -> Unit,
    val paste: (String) -> Unit,
    val notificationSettings: () -> Unit,
    val close: () -> Unit,
)

@Composable
fun BrokerScreen(screen: Screen, model: BrokerModel, actions: Actions, origin: String?, notifying: Boolean, stale: Boolean) {
    val p = LocalPalette.current
    Box(
        Modifier
            .fillMaxSize()
            .background(p.bg)
            .safeDrawingPadding(),
    ) {
        Column(
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 16.dp),
        ) {
            when (screen) {
                Screen.Loading -> {
                    Header("broker", "Checking…")
                    Waiting("Asking the PC…")
                }
                is Screen.Setup -> SetupScreen(screen, actions)
                is Screen.Enrolling -> EnrollingScreen(screen, model)
                is Screen.Idle -> IdleScreen(screen, model, actions, origin, notifying, stale)
                is Screen.Asking -> AskingScreen(screen, model, actions)
            }
        }
    }
}

// ------------------------------------------------------------------ parts

@Composable
private fun Header(title: String, sub: String? = null, menu: (@Composable () -> Unit)? = null) {
    val p = LocalPalette.current
    Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(bottom = 8.dp)) {
        Box(
            Modifier
                .size(44.dp)
                .clip(RoundedCornerShape(12.dp))
                .background(p.panel),
            contentAlignment = Alignment.Center,
        ) {
            Icon(painterResource(R.drawable.ic_key), null, tint = p.primary, modifier = Modifier.size(24.dp))
        }
        Column(
            Modifier
                .padding(start = 14.dp)
                .weight(1f),
        ) {
            Text(title, color = p.fg, fontSize = 20.sp, fontWeight = FontWeight.SemiBold, lineHeight = 25.sp)
            if (sub != null) Text(sub, color = p.muted, fontSize = 14.sp)
        }
        menu?.invoke()
    }
}

@Composable
private fun Label(text: String) {
    Text(
        text.uppercase(),
        color = LocalPalette.current.faint,
        fontSize = 12.sp,
        letterSpacing = 0.8.sp,
        modifier = Modifier.padding(top = 20.dp, bottom = 8.dp),
    )
}

@Composable
private fun Panel(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier
            .fillMaxWidth()
            .padding(bottom = 6.dp)
            .clip(RoundedCornerShape(10.dp))
            .background(LocalPalette.current.panel)
            .padding(horizontal = 12.dp, vertical = 10.dp),
        content = content,
    )
}

@Composable
private fun Boxed(bg: Color, line: Color, modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Column(
        modifier
            .fillMaxWidth()
            .padding(top = 14.dp)
            .clip(RoundedCornerShape(10.dp))
            .background(bg)
            .border(1.dp, line, RoundedCornerShape(10.dp))
            .padding(14.dp),
        content = content,
    )
}

@Composable
private fun ErrorBox(text: String) {
    val p = LocalPalette.current
    Boxed(p.warnBg, p.warnLine) {
        Row(verticalAlignment = Alignment.Top) {
            Icon(painterResource(R.drawable.ic_alert), null, tint = p.warnFg, modifier = Modifier.size(20.dp))
            Text(text, color = p.warnFg, fontSize = 15.sp, modifier = Modifier.padding(start = 10.dp))
        }
    }
}

@Composable
private fun Result(text: String, bad: Boolean) {
    val p = LocalPalette.current
    Boxed(if (bad) p.warnBg else p.touchBg, if (bad) p.warnLine else p.touchLine) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(
                painterResource(if (bad) R.drawable.ic_close else R.drawable.ic_check),
                null,
                tint = if (bad) p.warnFg else p.ok,
                modifier = Modifier.size(22.dp),
            )
            Text(text, color = p.fg, fontSize = 16.sp, fontWeight = FontWeight.Medium, modifier = Modifier.padding(start = 10.dp))
        }
    }
}

@Composable
private fun Waiting(text: String) {
    val p = LocalPalette.current
    Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(top = 18.dp)) {
        CircularProgressIndicator(Modifier.size(20.dp), color = p.primary, strokeWidth = 2.5.dp)
        Text(text, color = p.muted, fontSize = 15.sp, modifier = Modifier.padding(start = 12.dp))
    }
}

/** "Touch your YubiKey", with a pulsing dot. */
@Composable
private fun Touch(title: String, hint: String, onCancel: (() -> Unit)? = null) {
    val p = LocalPalette.current
    val pulse by rememberInfiniteTransition(label = "touch").animateFloat(
        initialValue = 0.35f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(tween(700), RepeatMode.Reverse),
        label = "dot",
    )
    Boxed(p.touchBg, p.touchLine) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier
                    .size(12.dp)
                    .alpha(pulse)
                    .clip(CircleShape)
                    .background(p.ok),
            )
            Column(
                Modifier
                    .padding(start = 14.dp)
                    .weight(1f),
            ) {
                Text(title, color = p.fg, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
                Text(hint, color = p.muted, fontSize = 14.sp)
            }
            if (onCancel != null) TextButton(onClick = onCancel) { Text("Cancel") }
        }
    }
}

@Composable
private fun PrimaryButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier) {
    Button(
        onClick = onClick,
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp),
        shape = RoundedCornerShape(12.dp),
    ) { Text(text, fontSize = 16.sp, fontWeight = FontWeight.SemiBold) }
}

@Composable
private fun SecondaryButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    Button(
        onClick = onClick,
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp),
        shape = RoundedCornerShape(12.dp),
        colors = ButtonDefaults.buttonColors(containerColor = p.button, contentColor = p.fg),
    ) { Text(text, fontSize = 16.sp) }
}

@Composable
private fun Mono(text: String, color: Color = LocalPalette.current.fg) {
    Text(text, color = color, fontFamily = FontFamily.Monospace, fontSize = 14.sp, lineHeight = 20.sp)
}

/** Seconds until [deadline] as m:ss, ticking. */
@Composable
private fun countdown(deadline: Instant): String {
    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(deadline) {
        while (true) {
            now = System.currentTimeMillis()
            delay(1000)
        }
    }
    val ms = deadline.toEpochMilli() - now
    if (ms <= 0) return "0:00"
    val s = (ms + 999) / 1000
    return "${s / 60}:${(s % 60).toString().padStart(2, '0')}"
}

@Composable
private fun Menu(items: List<Pair<String, () -> Unit>>) {
    var open by remember { mutableStateOf(false) }
    Box {
        IconButton(onClick = { open = true }) {
            Icon(painterResource(R.drawable.ic_more), "More", tint = LocalPalette.current.muted)
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            items.forEach { (label, action) ->
                DropdownMenuItem(text = { Text(label) }, onClick = {
                    open = false
                    action()
                })
            }
        }
    }
}

// ---------------------------------------------------------------- screens

@Composable
private fun SetupScreen(s: Screen.Setup, actions: Actions) {
    val p = LocalPalette.current
    var pasting by remember { mutableStateOf(false) }
    Header("broker", "Approve agents' requests from this phone")
    Label("Set up")
    Text("On the PC, run:", color = p.fg, fontSize = 16.sp)
    Panel(Modifier.padding(top = 8.dp)) { SelectionContainer { Mono("broker phone enroll --name phone") } }
    Text(
        "Then scan the QR code it prints. Keep the YubiKey for this phone plugged in, or hold it to the back of the phone when asked. " +
            "It never needs its PIN here: a touch approves, like on the PC.",
        color = p.muted,
        fontSize = 15.sp,
        lineHeight = 21.sp,
        modifier = Modifier.padding(top = 10.dp),
    )
    if (s.error != null) ErrorBox(s.error)
    Spacer(Modifier.height(22.dp))
    PrimaryButton("Scan QR code", actions.scan)
    Spacer(Modifier.height(10.dp))
    SecondaryButton("Paste link", { pasting = true })
    if (pasting) {
        var link by remember { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { pasting = false },
            title = { Text("Enroll link") },
            text = {
                OutlinedTextField(
                    value = link,
                    onValueChange = { link = it },
                    placeholder = { Text("https://…/enroll#…") },
                    singleLine = true,
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    pasting = false
                    actions.paste(link)
                }) { Text("Enroll") }
            },
            dismissButton = { TextButton(onClick = { pasting = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun EnrollingScreen(s: Screen.Enrolling, model: BrokerModel) {
    val p = LocalPalette.current
    if (s.step == EnrollStep.Done) {
        Header("Enrolled", "This phone can approve broker's requests")
        Result("The PC added this phone's key", bad = false)
        Text(
            if (s.notifications) "Requests will notify this phone. Open one, check what's asked, and touch the key to approve."
            else "Requests won't notify this phone. Open broker when an agent asks.",
            color = p.muted,
            fontSize = 15.sp,
            lineHeight = 21.sp,
            modifier = Modifier.padding(top = 14.dp),
        )
        if (s.note != null) ErrorBox(s.note)
        Spacer(Modifier.height(22.dp))
        PrimaryButton("Done", model::enrollDone)
        return
    }
    Header("Enrolling", "Adding this phone's key to broker")
    val steps = listOf(
        EnrollStep.Connecting to "Reach the PC",
        EnrollStep.Credential to "Make a credential on the YubiKey",
        EnrollStep.Prf to "Get the key's unlock",
        EnrollStep.Pc to "The PC adds the key",
    )
    Spacer(Modifier.height(8.dp))
    steps.forEach { (step, label) ->
        val state = when {
            step.ordinal < s.step.ordinal -> 1
            step == s.step -> if (s.error != null) -1 else 0
            else -> 2
        }
        Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 9.dp)) {
            Box(Modifier.size(24.dp), contentAlignment = Alignment.Center) {
                when (state) {
                    1 -> Icon(painterResource(R.drawable.ic_check), null, tint = p.ok, modifier = Modifier.size(20.dp))
                    0 -> CircularProgressIndicator(Modifier.size(18.dp), color = p.primary, strokeWidth = 2.dp)
                    -1 -> Icon(painterResource(R.drawable.ic_close), null, tint = p.warnFg, modifier = Modifier.size(20.dp))
                    else -> Box(
                        Modifier
                            .size(8.dp)
                            .clip(CircleShape)
                            .background(p.line),
                    )
                }
            }
            Text(
                label,
                color = if (state == 2) p.faint else p.fg,
                fontSize = 16.sp,
                modifier = Modifier.padding(start = 12.dp),
            )
        }
    }
    when {
        s.error != null -> {
            ErrorBox(s.error)
            Spacer(Modifier.height(22.dp))
            SecondaryButton("Start over", model::enrollDone)
        }
        s.step == EnrollStep.Credential -> Touch("Touch your YubiKey", "It makes a credential for broker. Plugged into this phone, or held to its back.")
        s.step == EnrollStep.Prf -> Touch("Touch it again", "This time it gives the unlock the PC wraps the vault to.")
        s.step == EnrollStep.Pc -> Boxed(p.panel, p.line) {
            Text("Finish on the PC", color = p.fg, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
            Text(
                "In the terminal, press Enter, then touch the YubiKey that unlocks the vault there.",
                color = p.muted,
                fontSize = 15.sp,
                lineHeight = 21.sp,
                modifier = Modifier.padding(top = 4.dp),
            )
        }
    }
}

@Composable
private fun IdleScreen(s: Screen.Idle, model: BrokerModel, actions: Actions, origin: String?, notifying: Boolean, stale: Boolean) {
    val p = LocalPalette.current
    var forgetting by remember { mutableStateOf(false) }
    Header("broker", menu = {
        Menu(
            listOf(
                "Check again" to { model.refresh() },
                "Notification settings" to actions.notificationSettings,
                "Forget this PC" to { forgetting = true },
            ),
        )
    })
    if (s.problem && s.note != null) {
        ErrorBox(s.note)
    } else {
        Boxed(p.panel, p.line) {
            Text(s.note ?: "Nothing to approve", color = p.fg, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
            Text(
                "broker isn't asking for anything right now. A notification brings you here when an agent asks.",
                color = p.muted,
                fontSize = 15.sp,
                lineHeight = 21.sp,
                modifier = Modifier.padding(top = 4.dp),
            )
        }
    }
    if (stale) {
        ErrorBox("Notifications stopped: Firebase gave this phone a new token, which the PC doesn't know. Run broker phone rm and enroll again to get them back.")
    }
    Spacer(Modifier.height(22.dp))
    SecondaryButton("Check again", { model.refresh() })
    Spacer(Modifier.height(18.dp))
    Text(
        listOfNotNull(origin?.removePrefix("https://"), if (notifying) "notifications on" else null).joinToString(" · "),
        color = p.faint,
        fontSize = 13.sp,
    )
    if (!notifying && !stale) {
        Text(
            "Notifications are off",
            color = p.primary,
            fontSize = 13.sp,
            modifier = Modifier
                .padding(top = 4.dp)
                .clickable(onClick = actions.notificationSettings),
        )
    }
    if (forgetting) {
        AlertDialog(
            onDismissRequest = { forgetting = false },
            title = { Text("Forget this PC?") },
            text = { Text("This phone stops seeing requests. The PC still has its key until you run broker phone rm there.") },
            confirmButton = {
                TextButton(onClick = {
                    forgetting = false
                    model.forget()
                }) { Text("Forget") }
            },
            dismissButton = { TextButton(onClick = { forgetting = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun AskingScreen(s: Screen.Asking, model: BrokerModel, actions: Actions) {
    val p = LocalPalette.current
    val r = s.request
    val n = r.secrets.size
    val open = s.phase == Phase.Request
    val left = countdown(r.deadline)
    Header(
        if (n == 1) "An agent wants to use a secret" else "An agent wants to use $n secrets",
        if (s.phase == Phase.Done) "broker request" else "broker request · expires in $left",
    )

    Label(if (n == 1) "Secret" else "Secrets")
    r.secrets.forEach { secret ->
        Panel {
            Mono(secret.name)
            if (secret.description.isNotEmpty()) {
                Text(secret.description, color = p.muted, fontSize = 14.sp, modifier = Modifier.padding(top = 2.dp))
            }
        }
    }

    Label("Request")
    Field("For") {
        if (open && r.choices.size > 1) DurationPicker(s.picked, r, model::pick)
        else Text(human(s.picked), color = p.fg, fontSize = 15.sp)
    }
    Field("Asked by") { Mono(r.chain().joinToString(" < ").ifEmpty { "unknown" }) }
    if (r.cwd.isNotEmpty()) Field("Directory") { Mono(r.tilde(r.cwd)) }
    if (r.command.isNotEmpty()) Field("Will run") { Mono(quote(r.command)) }

    if (r.reason.isNotEmpty()) {
        Boxed(p.claimBg, p.claimLine) {
            Text("The agent says (unverified)", color = p.claimFg, fontSize = 12.sp, fontWeight = FontWeight.Medium)
            Text(r.reason, color = p.fg, fontSize = 15.sp, lineHeight = 21.sp, modifier = Modifier.padding(top = 4.dp))
        }
    }
    Text(
        "While granted, any agent on the PC can use ${if (n == 1) "it" else "them"}, not just this one.",
        color = p.faint,
        fontSize = 13.sp,
        modifier = Modifier.padding(top = 12.dp),
    )

    when (s.phase) {
        Phase.Done -> {
            val o = s.outcome ?: Outcome("This request is over", true)
            Result(o.text, o.bad)
            Spacer(Modifier.height(22.dp))
            SecondaryButton("Close", actions.close)
        }
        Phase.Key -> Touch("Touch your YubiKey", "Plugged into this phone, or held to its back.", onCancel = model::cancelKey)
        Phase.Denying -> Waiting("Denying…")
        Phase.Unlocking -> {
            val st = s.status
            if (st != null && st.bad) ErrorBox(st.text) else Waiting(st?.text ?: "Unlocking on the PC…")
        }
        Phase.Request -> {
            if (s.error != null) ErrorBox(s.error)
            Spacer(Modifier.height(22.dp))
            PrimaryButton("Approve with security key", model::approve)
            Spacer(Modifier.height(10.dp))
            SecondaryButton("Deny", model::deny)
        }
    }
}

@Composable
private fun Field(key: String, value: @Composable () -> Unit) {
    val p = LocalPalette.current
    Row(Modifier.padding(vertical = 5.dp), verticalAlignment = Alignment.Top) {
        Text(key, color = p.muted, fontSize = 15.sp, modifier = Modifier.width(96.dp))
        Box(Modifier.weight(1f)) { value() }
    }
}

@Composable
private fun DurationPicker(picked: Long, r: Request, onPick: (Long) -> Unit) {
    val p = LocalPalette.current
    var open by remember { mutableStateOf(false) }
    Box {
        OutlinedButton(
            onClick = { open = true },
            shape = RoundedCornerShape(8.dp),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 12.dp, vertical = 4.dp),
        ) {
            Text(human(picked) + if (picked == r.forSecs) " (asked)" else "", color = p.fg, fontSize = 15.sp)
            Text(" ▾", color = p.muted, fontSize = 15.sp)
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            r.choices.sortedDescending().forEach { c ->
                DropdownMenuItem(
                    text = { Text(human(c) + if (c == r.forSecs) " (asked)" else "", fontWeight = if (c == picked) FontWeight.SemiBold else null) },
                    onClick = {
                        open = false
                        onPick(c)
                    },
                )
            }
        }
    }
}
