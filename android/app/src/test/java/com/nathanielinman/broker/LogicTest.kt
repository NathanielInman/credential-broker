package com.nathanielinman.broker

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.security.MessageDigest

class LogicTest {
    @Test
    fun parsesEnrollLinks() {
        assertEquals(
            "https://broker.twinspear.com:8443" to "abc_-123",
            Api.parseEnrollLink("https://broker.twinspear.com:8443/enroll#abc_-123"),
        )
        assertEquals("https://broker.example" to "t", Api.parseEnrollLink(" https://broker.example:443/enroll#t\n"))
        assertNull(Api.parseEnrollLink("http://broker.example/enroll#t"))
        assertNull(Api.parseEnrollLink("https://broker.example/#t"))
        assertNull(Api.parseEnrollLink("https://broker.example/enroll"))
        assertNull(Api.parseEnrollLink("https://broker.example/enroll#"))
    }

    @Test
    fun prfSaltIsWebAuthns() {
        val input = ByteArray(32) { it.toByte() }
        val want = MessageDigest.getInstance("SHA-256").digest("WebAuthn PRF".toByteArray() + byteArrayOf(0) + input)
        assertArrayEquals(want, Fido.prfSalt(input))
    }

    @Test
    fun saysDurationsLikeTheWindow() {
        assertEquals("1 hour", human(3600))
        assertEquals("8 hours", human(8 * 3600))
        assertEquals("15 min", human(900))
        assertEquals("1h 30m", human(5400))
    }

    @Test
    fun trimsTheProcessChain() {
        val r = Request(
            emptyList(), 0, emptyList(), java.time.Instant.EPOCH,
            listOf("broker", "zsh", "claude", "claude", "node", "Hyprland", "systemd"),
            "/home/nate/x", "/home/nate", emptyList(), "",
        )
        assertEquals(listOf("zsh", "claude", "node"), r.chain())
        assertEquals("~/x", r.tilde(r.cwd))
    }

    @Test
    fun quotesLikeAShell() {
        assertEquals("gh pr list 'a b' 'it'\\''s'", quote(listOf("gh", "pr", "list", "a b", "it's")))
    }
}
