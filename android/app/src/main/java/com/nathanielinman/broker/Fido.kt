package com.nathanielinman.broker

import com.yubico.yubikit.core.YubiKeyDevice
import com.yubico.yubikit.core.application.CommandState
import com.yubico.yubikit.core.fido.CtapException
import com.yubico.yubikit.core.fido.FidoConnection
import com.yubico.yubikit.core.smartcard.SmartCardConnection
import com.yubico.yubikit.fido.ctap.ClientPin
import com.yubico.yubikit.fido.ctap.Ctap2Session
import com.yubico.yubikit.fido.ctap.PinUvAuthProtocol
import com.yubico.yubikit.fido.ctap.PinUvAuthProtocolV1
import com.yubico.yubikit.fido.ctap.PinUvAuthProtocolV2
import com.yubico.yubikit.fido.webauthn.AuthenticatorData
import java.io.IOException
import java.nio.ByteBuffer
import java.security.MessageDigest
import java.security.SecureRandom

/**
 * The security key, spoken to directly (CTAP2 over USB or NFC) instead of
 * through Android's WebAuthn, which insists on the key's PIN. broker never
 * asks the key to verify the user: the touch is the approval, as on the PC.
 *
 * Enrolling makes a non-discoverable credential with the hmac-secret
 * extension; unlocking evaluates hmac-secret on the PC's salt, as WebAuthn
 * PRF would (the salt goes through the same SHA-256 with "WebAuthn PRF").
 * Without user verification the key answers from its no-UV secret, so a
 * credential enrolled through the page in Chrome (with a PIN) gives a
 * different output here, and the app enrolls its own.
 */
object Fido {
    private val random = SecureRandom()

    /** Runs [block] with a CTAP2 session on [device], over FIDO HID (USB) or the smart card interface (NFC). */
    fun <T> withSession(device: YubiKeyDevice, block: (Ctap2Session) -> T): T {
        if (device.supportsConnection(FidoConnection::class.java)) {
            device.openConnection(FidoConnection::class.java).use { return block(Ctap2Session(it)) }
        }
        device.openConnection(SmartCardConnection::class.java).use { return block(Ctap2Session(it)) }
    }

    /** Makes a credential for [rpId] and returns its ID. Takes a touch. */
    fun makeCredential(session: Ctap2Session, rpId: String, userId: ByteArray, state: CommandState): ByteArray {
        if ("hmac-secret" !in session.cachedInfo.extensions) {
            throw KeyError("This security key can't do hmac-secret, which broker needs. A YubiKey 5 can.")
        }
        val data = session.makeCredential(
            randomBytes(32),
            mapOf("id" to rpId, "name" to "broker"),
            mapOf("id" to userId, "name" to "broker", "displayName" to "broker"),
            listOf(mapOf("type" to "public-key", "alg" to -7)),
            null,
            mapOf("hmac-secret" to true),
            mapOf("rk" to false),
            null, null, null, state,
        )
        val auth = AuthenticatorData.parseFrom(ByteBuffer.wrap(data.authenticatorData))
        return auth.attestedCredentialData?.credentialId ?: throw KeyError("The key made no credential.")
    }

    /** The PRF output of [credential] on [salt]: 32 bytes. Takes a touch. */
    fun prf(session: Ctap2Session, rpId: String, credential: ByteArray, salt: ByteArray, state: CommandState): ByteArray {
        val protocol: PinUvAuthProtocol =
            if (2 in session.cachedInfo.pinUvAuthProtocols) PinUvAuthProtocolV2() else PinUvAuthProtocolV1()
        val shared = ClientPin(session, protocol).sharedSecret
        val saltEnc = protocol.encrypt(shared.second, prfSalt(salt))
        val input = mapOf(
            1 to shared.first,
            2 to saltEnc,
            3 to protocol.authenticate(shared.second, saltEnc),
            4 to protocol.version,
        )
        val assertion = session.getAssertions(
            rpId,
            randomBytes(32),
            listOf(mapOf("type" to "public-key", "id" to credential)),
            mapOf("hmac-secret" to input),
            null, null, null, state,
        ).first()
        val auth = AuthenticatorData.parseFrom(ByteBuffer.wrap(assertion.authenticatorData))
        val out = auth.extensions?.get("hmac-secret") as? ByteArray
            ?: throw KeyError("The key gave no hmac-secret output.")
        val plain = protocol.decrypt(shared.second, out)
        if (plain.size < 32) throw KeyError("The key's hmac-secret output is short.")
        return plain.copyOf(32)
    }

    /** WebAuthn PRF's salt: SHA-256("WebAuthn PRF" || 0x00 || input). */
    fun prfSalt(input: ByteArray): ByteArray =
        MessageDigest.getInstance("SHA-256").run {
            update("WebAuthn PRF".toByteArray(Charsets.US_ASCII))
            update(0)
            update(input)
            digest()
        }

    fun randomBytes(n: Int): ByteArray = ByteArray(n).also(random::nextBytes)

    /** Says what went wrong with the key in words for the human. */
    fun explain(e: Throwable): String = when (e) {
        is KeyError -> e.message.orEmpty()
        is CtapException -> when (e.ctapError) {
            CtapException.ERR_NO_CREDENTIALS -> "That's not the security key enrolled for this phone."
            CtapException.ERR_USER_ACTION_TIMEOUT, CtapException.ERR_ACTION_TIMEOUT -> "The key wasn't touched in time."
            CtapException.ERR_KEEPALIVE_CANCEL -> "Cancelled."
            CtapException.ERR_OPERATION_DENIED -> "The key refused."
            CtapException.ERR_PUAT_REQUIRED ->
                "This key wants its PIN to make credentials, and broker doesn't use PINs. Turn off \"always require user verification\" on the key."
            CtapException.ERR_UNSUPPORTED_EXTENSION -> "This security key can't do hmac-secret, which broker needs."
            else -> "The key said ${e.message}."
        }
        is IOException -> "Lost the key (${e.message}). Keep it plugged in, or hold it to the phone until it's done."
        else -> e.message ?: e.toString()
    }
}

class KeyError(message: String) : Exception(message)
