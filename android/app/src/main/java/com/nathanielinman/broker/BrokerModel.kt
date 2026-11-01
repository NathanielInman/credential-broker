package com.nathanielinman.broker

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.IOException
import java.util.Base64

/** How a request ended, in broker's words. */
data class Outcome(val text: String, val bad: Boolean)

/** Where an enrollment is. */
enum class EnrollStep { Connecting, Credential, Prf, Pc, Done }

/** Where a request is on this phone. */
enum class Phase { Request, Key, Unlocking, Denying, Done }

sealed interface Screen {
    data object Loading : Screen

    /** Not enrolled: scan the QR code `broker phone enroll` prints. */
    data class Setup(val error: String? = null) : Screen

    data class Enrolling(
        val step: EnrollStep,
        val error: String? = null,
        val notifications: Boolean = false,
        val note: String? = null,
    ) : Screen

    /** Nothing to approve, or the PC cannot be reached ([problem]). */
    data class Idle(val note: String? = null, val problem: Boolean = false) : Screen

    data class Asking(
        val request: Request,
        val phase: Phase = Phase.Request,
        val picked: Long = request.forSecs,
        val error: String? = null,
        val status: Outcome? = null,
        val outcome: Outcome? = null,
    ) : Screen
}

/**
 * The screen once [r] arrives. A poll from the start sees the open
 * request again, which keeps what the human has done with it so far; a
 * request after one that is over starts fresh, not with its outcome.
 */
fun Screen.showing(r: Request): Screen =
    if (this is Screen.Asking && phase != Phase.Done) copy(request = r) else Screen.Asking(r)

/** The phone's key as the PC knows it, from each poll. */
private data class KeyInfo(val rpId: String, val credential: ByteArray, val salt: ByteArray)

class BrokerModel(app: Application) : AndroidViewModel(app) {
    private val ctx get() = getApplication<Application>()

    private val _screen = MutableStateFlow<Screen>(Screen.Loading)
    val screen: StateFlow<Screen> = _screen

    /** The activity's key reader, while there is an activity. */
    var reader: KeyReader? = null

    private var api: Api? = null
    private var key: KeyInfo? = null
    private var watching: Job? = null
    private var working: Job? = null
    private var approveWhenShown = false

    init {
        viewModelScope.launch {
            Events.pushed.collect {
                val s = _screen.value
                if (s is Screen.Idle || s is Screen.Asking && s.phase == Phase.Done) refresh()
            }
        }
    }

    // ------------------------------------------------------------ requests

    /** Asks the PC what is pending; with [approve], starts approving as soon as it shows. */
    fun refresh(approve: Boolean = false) {
        if (working?.isActive == true && _screen.value is Screen.Enrolling) return
        val e = Store.enrollment(ctx)
        if (e == null) {
            if (_screen.value !is Screen.Setup) _screen.value = Screen.Setup()
            return
        }
        approveWhenShown = approve
        val a = Api(e.origin, e.token)
        api = a
        watch(a)
    }

    /** Refreshes when coming back to an app that has nothing open. */
    fun resumed() {
        when (val s = _screen.value) {
            is Screen.Idle, Screen.Loading -> refresh()
            is Screen.Asking -> if (s.phase == Phase.Done) refresh()
            else -> {}
        }
    }

    private fun watch(a: Api) {
        watching?.cancel()
        watching = viewModelScope.launch {
            if (_screen.value !is Screen.Asking) _screen.value = Screen.Loading
            var next = 0
            while (isActive) {
                val data = try {
                    withContext(Dispatchers.IO) { a.get("/api/poll?after=$next", readTimeoutMs = 35_000) }
                } catch (e: CancellationException) {
                    throw e
                } catch (e: ApiError) {
                    if (e.status == 401) {
                        _screen.value = Screen.Setup("This phone is no longer enrolled with the PC. Run broker phone enroll there and scan again.")
                    } else {
                        ended(e.message, problem = true)
                    }
                    return@launch
                } catch (e: Unreachable) {
                    ended(e.message, problem = e.cause !is java.net.ConnectException)
                    return@launch
                } catch (e: IOException) {
                    ended("Lost the PC (${e.message}).", problem = true)
                    return@launch
                }
                key = runCatching {
                    KeyInfo(data.getString("rpId"), b64d(data.getString("credential")), b64d(data.getString("salt")))
                }.getOrNull()
                data.optJSONArray("messages").objects().forEach(::receive)
                next = data.optInt("next", next)
                val s = _screen.value
                if (data.optBoolean("closed") && !(s is Screen.Asking && s.phase == Phase.Done)) {
                    if (s is Screen.Asking) {
                        _screen.value = s.copy(phase = Phase.Done, outcome = s.outcome ?: Outcome("This request is over", true))
                    } else {
                        _screen.value = Screen.Idle()
                    }
                }
                val now = _screen.value
                if (now is Screen.Asking && now.phase == Phase.Done) {
                    Notifications.clearRequest(ctx)
                    return@launch
                }
                if (approveWhenShown && now is Screen.Asking && now.phase == Phase.Request) {
                    approveWhenShown = false
                    approve()
                }
            }
        }
    }

    /** The PC stopped answering: a request it had is over, or there was none. */
    private fun ended(note: String?, problem: Boolean) {
        val s = _screen.value
        _screen.value = when {
            s is Screen.Asking && s.phase != Phase.Done ->
                s.copy(phase = Phase.Done, outcome = s.outcome ?: Outcome("This request is over", true))
            s is Screen.Asking -> s
            problem -> Screen.Idle(note, problem = true)
            else -> Screen.Idle()
        }
    }

    private fun receive(m: JSONObject) {
        when (m.optString("type")) {
            "request" -> {
                val r = Request.parse(m.optJSONObject("request") ?: return)
                _screen.update { s -> s.showing(r) }
            }
            // The window's own steps (touch the key on the PC) mean nothing
            // here; only what follows this phone's answer is shown.
            "status" -> _screen.update { s ->
                if (s is Screen.Asking && s.phase == Phase.Unlocking) s.copy(status = Outcome(m.optString("text"), m.optBoolean("error"))) else s
            }
            "done" -> _screen.update { s ->
                val o = Outcome(m.optString("text"), m.optBoolean("error"))
                if (s is Screen.Asking) s.copy(phase = Phase.Done, outcome = o) else Screen.Idle(o.text, o.bad)
            }
        }
    }

    fun pick(secs: Long) {
        _screen.update { s -> if (s is Screen.Asking) s.copy(picked = secs) else s }
        reply(JSONObject().put("type", "for").put("for", secs))
    }

    fun deny() {
        _screen.update { s -> if (s is Screen.Asking) s.copy(phase = Phase.Denying, error = null) else s }
        reply(JSONObject().put("type", "deny"))
    }

    private fun reply(body: JSONObject) {
        val a = api ?: return
        viewModelScope.launch(Dispatchers.IO) { runCatching { a.post("/api/reply", body) } }
    }

    /** Asks the key for its PRF output (a touch) and sends it to the PC. */
    fun approve() {
        val s = _screen.value as? Screen.Asking ?: return
        val k = key ?: return
        val a = api ?: return
        val r = reader ?: return
        _screen.value = s.copy(phase = Phase.Key, error = null)
        working = viewModelScope.launch {
            try {
                val prf = r.use { session, state -> Fido.prf(session, k.rpId, k.credential, k.salt, state) }
                _screen.update { cur -> if (cur is Screen.Asking && cur.phase == Phase.Key) cur.copy(phase = Phase.Unlocking, status = null) else cur }
                val picked = (_screen.value as? Screen.Asking)?.picked ?: s.picked
                try {
                    withContext(Dispatchers.IO) {
                        a.post("/api/unlock", JSONObject().put("prf", b64e(prf)).put("for", picked))
                    }
                } finally {
                    prf.fill(0)
                }
            } catch (e: CancellationException) {
                _screen.update { cur -> if (cur is Screen.Asking && cur.phase == Phase.Key) cur.copy(phase = Phase.Request) else cur }
                throw e
            } catch (e: Throwable) {
                val msg = if (e is ApiError || e is Unreachable) e.message else Fido.explain(e)
                _screen.update { cur -> if (cur is Screen.Asking && cur.phase != Phase.Done) cur.copy(phase = Phase.Request, error = msg) else cur }
            }
        }
    }

    /** Stops waiting for the key. */
    fun cancelKey() {
        working?.cancel()
    }

    // ---------------------------------------------------------- enrollment

    fun setupError(message: String?) {
        _screen.value = Screen.Setup(message)
    }

    /** Enrolls with the link from `broker phone enroll`'s QR code. */
    fun enroll(link: String) {
        val parsed = Api.parseEnrollLink(link)
        if (parsed == null) {
            _screen.value = Screen.Setup("That isn't a broker enroll link. On the PC, run broker phone enroll and scan the QR code it prints.")
            return
        }
        val r = reader ?: return
        val (origin, token) = parsed
        watching?.cancel()
        working?.cancel()
        working = viewModelScope.launch {
            var step = EnrollStep.Connecting
            fun at(s: EnrollStep) {
                step = s
                _screen.value = Screen.Enrolling(s)
            }
            try {
                at(EnrollStep.Connecting)
                val enroll = Api(origin, token)
                val opts = withContext(Dispatchers.IO) { enroll.get("/api/enroll") }
                val rpId = opts.getString("rpId")
                val userId = b64d(opts.getString("userId"))
                val salt = b64d(opts.getString("salt"))
                val fcm = opts.optJSONObject("fcm")?.let {
                    FcmSettings(it.getString("project_id"), it.getString("sender_id"), it.getString("app_id"), it.getString("api_key"))
                }
                var note: String? = null
                val fcmToken = if (fcm == null) {
                    note = "The PC has no Firebase project set up (broker phone fcm), so requests won't notify this phone."
                    null
                } else {
                    runCatching { Push.token(ctx, fcm) }.getOrElse {
                        note = "Couldn't set up notifications: ${it.message}"
                        null
                    }
                }

                at(EnrollStep.Credential)
                val (credential, prf) = r.use { session, state ->
                    val c = Fido.makeCredential(session, rpId, userId, state)
                    step = EnrollStep.Prf
                    _screen.value = Screen.Enrolling(EnrollStep.Prf)
                    c to Fido.prf(session, rpId, c, salt, state)
                }

                at(EnrollStep.Pc)
                val body = JSONObject()
                    .put("credential", b64e(credential))
                    .put("prf", b64e(prf))
                    .put("app", true)
                if (fcmToken != null) body.put("fcm", fcmToken)
                val res = try {
                    withContext(Dispatchers.IO) { enroll.post("/api/enroll", body) }
                } finally {
                    prf.fill(0)
                }
                val phoneToken = res.getString("token")
                // The phone's part is half of enrolling: the PC still has
                // to unlock the vault and wrap it to this key. Only then is
                // the token worth keeping.
                val pc = Api(origin, phoneToken)
                while (isActive) {
                    val result = withContext(Dispatchers.IO) { pc.get("/api/enroll/result", readTimeoutMs = 35_000) }
                    if (!result.optBoolean("done")) continue
                    val err = result.optString("error")
                    if (err.isNotEmpty()) throw ApiError(0, err)
                    break
                }
                Store.enrolled(ctx, Enrollment(origin, phoneToken), fcm, fcmToken)
                _screen.value = Screen.Enrolling(EnrollStep.Done, notifications = fcmToken != null, note = note)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Throwable) {
                val msg = when (e) {
                    is ApiError, is Unreachable -> e.message
                    is IOException -> if (step == EnrollStep.Credential || step == EnrollStep.Prf) Fido.explain(e) else "Lost the PC (${e.message})."
                    else -> Fido.explain(e)
                }
                _screen.value = Screen.Enrolling(step, error = msg)
            }
        }
    }

    /** Leaves a finished or failed enrollment. */
    fun enrollDone() {
        working?.cancel()
        if (Store.enrollment(ctx) == null) _screen.value = Screen.Setup() else refresh()
    }

    /** Forgets the PC; enrolling again starts over. */
    fun forget() {
        watching?.cancel()
        working?.cancel()
        Store.forget(ctx)
        Notifications.clearRequest(ctx)
        _screen.value = Screen.Setup()
    }

    companion object {
        private val enc = Base64.getUrlEncoder().withoutPadding()
        private val dec = Base64.getUrlDecoder()
        fun b64e(b: ByteArray): String = enc.encodeToString(b)
        fun b64d(s: String): ByteArray = dec.decode(s)
    }
}
