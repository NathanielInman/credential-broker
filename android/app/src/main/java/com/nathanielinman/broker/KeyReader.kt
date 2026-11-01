package com.nathanielinman.broker

import android.app.Activity
import com.yubico.yubikit.android.YubiKitManager
import com.yubico.yubikit.android.transport.nfc.NfcConfiguration
import com.yubico.yubikit.android.transport.nfc.NfcNotAvailable
import com.yubico.yubikit.android.transport.usb.UsbConfiguration
import com.yubico.yubikit.core.YubiKeyDevice
import com.yubico.yubikit.core.application.CommandState
import com.yubico.yubikit.fido.ctap.Ctap2Session
import kotlinx.coroutines.suspendCancellableCoroutine
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/**
 * Finds the security key, plugged in (USB) or held to the phone (NFC), for
 * as long as the activity waits on one.
 */
class KeyReader(private val activity: Activity) {
    private val manager = YubiKitManager(activity)
    private val worker = Executors.newSingleThreadExecutor()

    /**
     * Waits for a key and runs [block] on it with a CTAP2 session, off the
     * main thread. Cancelling cancels whatever the key is waiting for.
     */
    suspend fun <T> use(block: (Ctap2Session, CommandState) -> T): T = suspendCancellableCoroutine { cont ->
        val state = CommandState()
        val taken = AtomicBoolean(false)
        fun stop() {
            manager.stopUsbDiscovery()
            activity.runOnUiThread { manager.stopNfcDiscovery(activity) }
        }
        fun found(device: YubiKeyDevice) {
            if (taken.getAndSet(true)) return
            worker.execute {
                val result = runCatching { Fido.withSession(device) { block(it, state) } }
                stop()
                result.fold({ cont.resume(it) }, { cont.resumeWithException(it) })
            }
        }
        manager.startUsbDiscovery(UsbConfiguration()) { found(it) }
        try {
            manager.startNfcDiscovery(NfcConfiguration().timeout(30_000), activity) { found(it) }
        } catch (_: NfcNotAvailable) {
            // USB alone, then.
        }
        cont.invokeOnCancellation {
            state.cancel()
            stop()
        }
    }
}
