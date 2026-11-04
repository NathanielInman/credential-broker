package com.nathanielinman.broker

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
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
    fun aNewRequestDropsTheLastOutcome() {
        fun req(secs: Long) = Request(
            emptyList(), secs, listOf(secs), java.time.Instant.EPOCH,
            emptyList(), "", "", emptyList(), "",
        )
        val granted = Screen.Asking(req(300), phase = Phase.Done, outcome = Outcome("Granted for 5m", false))
        assertEquals(Screen.Asking(req(3600)), granted.showing(req(3600)))

        val picking = Screen.Asking(req(3600), phase = Phase.Key, picked = 900)
        assertEquals(picking, picking.showing(req(3600)))
        assertEquals(Screen.Asking(req(3600)), Screen.Idle().showing(req(3600)))
    }

    @Test
    fun closesItselfOnlyOnItsOwnGrant() {
        val r = Request(
            emptyList(), 300, listOf(300), java.time.Instant.EPOCH,
            emptyList(), "", "", emptyList(), "",
        )
        val granted = Outcome("Granted for 5m", false)
        val mine = Screen.Asking(r, phase = Phase.Unlocking).finished(granted)
        assertEquals(Screen.Asking(r, phase = Phase.Done, outcome = granted, approved = true), mine)
        assertTrue(mine.closesItself(true))
        assertFalse(mine.closesItself(false))

        // Granted on the PC while the phone only looked, or a failed unlock.
        assertFalse(Screen.Asking(r).finished(granted).closesItself(true))
        assertFalse(Screen.Asking(r, phase = Phase.Unlocking).finished(Outcome("Wrong key", true)).closesItself(true))
        assertFalse(Screen.Asking(r, phase = Phase.Denying).finished(Outcome("Denied", true)).closesItself(true))
        assertEquals(Screen.Idle("Granted for 5m", false), Screen.Idle().finished(granted))
        assertFalse(Screen.Idle().finished(granted).closesItself(true))
    }

    @Test
    fun explainsChangedSecrets() {
        fun req(vararg secrets: Secret) = Request(
            secrets.toList(), 0, emptyList(), java.time.Instant.EPOCH,
            emptyList(), "", "", emptyList(), "",
        )
        assertNull(req(Secret("A", "")).changedNote())
        assertEquals(
            "Asking again: A is still granted, but the value changed in the vault since, so the grant holds the old one. " +
                "Approving grants the new value.",
            req(Secret("A", "", changed = true), Secret("B", "")).changedNote(),
        )
        val three = req(Secret("A", "", true), Secret("B", "", true), Secret("C", "", true)).changedNote()!!
        assertEquals("Asking again: A, B, and C are", three.substringBefore(" still"))
    }

    @Test
    fun quotesLikeAShell() {
        assertEquals("gh pr list 'a b' 'it'\\''s'", quote(listOf("gh", "pr", "list", "a b", "it's")))
    }
}
