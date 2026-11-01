package com.nathanielinman.broker

import org.json.JSONObject
import java.io.IOException
import java.net.ConnectException
import java.net.HttpURLConnection
import java.net.NoRouteToHostException
import java.net.SocketTimeoutException
import java.net.URL
import java.net.UnknownHostException

/** The phone page answered with an error. */
class ApiError(val status: Int, message: String) : IOException(message)

/**
 * The phone page is not there. broker only listens while it is asking for
 * something, so this is usually not a fault.
 */
class Unreachable(message: String, cause: Throwable) : IOException(message, cause)

/**
 * Talks to broker's phone page on the PC, the same API the page's own
 * script uses. Every call is blocking; run it off the main thread.
 */
class Api(private val origin: String, private val token: String) {

    fun get(path: String, readTimeoutMs: Int = 10_000): JSONObject = call("GET", path, null, readTimeoutMs)

    fun post(path: String, body: JSONObject, readTimeoutMs: Int = 30_000): JSONObject =
        call("POST", path, body, readTimeoutMs)

    private fun call(method: String, path: String, body: JSONObject?, readTimeoutMs: Int): JSONObject {
        val conn = URL(origin + path).openConnection() as HttpURLConnection
        try {
            conn.requestMethod = method
            conn.connectTimeout = 5_000
            conn.readTimeout = readTimeoutMs
            conn.useCaches = false
            conn.setRequestProperty("Authorization", "Bearer $token")
            conn.setRequestProperty("Accept", "application/json")
            if (body != null) {
                conn.doOutput = true
                conn.setRequestProperty("Content-Type", "application/json")
                conn.outputStream.use { it.write(body.toString().toByteArray()) }
            }
            val status = conn.responseCode
            val stream = if (status in 200..299) conn.inputStream else conn.errorStream
            val text = stream?.use { it.readBytes().decodeToString() }.orEmpty()
            val json = if (text.isBlank()) JSONObject() else runCatching { JSONObject(text) }.getOrElse { JSONObject() }
            if (status !in 200..299) {
                throw ApiError(status, json.optString("error").ifEmpty { "HTTP $status" })
            }
            return json
        } catch (e: UnknownHostException) {
            throw Unreachable("Can't find ${URL(origin).host}. Is this phone on the home network?", e)
        } catch (e: ConnectException) {
            throw Unreachable("broker isn't asking for anything right now.", e)
        } catch (e: NoRouteToHostException) {
            throw Unreachable("Can't reach the PC. Is this phone on the home network?", e)
        } catch (e: SocketTimeoutException) {
            if (conn.connectTimeout > 0 && e.message?.contains("connect", ignoreCase = true) == true) {
                throw Unreachable("Can't reach the PC. Is this phone on the home network?", e)
            }
            throw e
        } finally {
            conn.disconnect()
        }
    }

    companion object {
        /** The enroll link `broker phone enroll` prints: `https://HOST[:PORT]/enroll#TOKEN`. */
        fun parseEnrollLink(link: String): Pair<String, String>? {
            val trimmed = link.trim()
            val hash = trimmed.indexOf('#')
            if (hash < 0) return null
            val token = trimmed.substring(hash + 1)
            val url = runCatching { URL(trimmed.substring(0, hash)) }.getOrNull() ?: return null
            if (url.protocol != "https" || url.host.isEmpty() || url.path != "/enroll" || token.isEmpty()) return null
            val origin = "https://" + url.host + if (url.port != -1 && url.port != 443) ":" + url.port else ""
            return origin to token
        }
    }
}
