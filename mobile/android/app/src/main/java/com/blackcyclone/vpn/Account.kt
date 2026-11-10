package com.blackcyclone.vpn

import org.json.JSONObject

/**
 * A registered WARP device, kept as the JSON the Go core reads and writes (same layout
 * as settings.Warp of the desktop app, so accounts can be copied between them).
 */
class Account private constructor(val json: String, private val o: JSONObject) {
    val endpoint: String get() = o.optString("endpoint")
    val port: Int get() = o.optInt("endpoint_port", 2408)
    val accountType: String get() = o.optString("account_type")
    val license: String get() = o.optString("license")
    val addresses: List<String>
        get() = o.optJSONArray("addresses")?.let { a -> (0 until a.length()).map { a.getString(it) } }.orEmpty()

    val endpointText: String get() = if (endpoint.contains(':')) "[$endpoint]:$port" else "$endpoint:$port"

    val typeLabel: String
        get() = when (accountType) {
            "", "free" -> tr("WARP miễn phí", "Free WARP")
            "unlimited", "limited" -> "WARP+"
            else -> accountType
        }

    fun withEndpoint(ip: String, port: Int): Account =
        parse(JSONObject(json).put("endpoint", ip).put("endpoint_port", port).toString())!!

    companion object {
        /** Parses account JSON; null if it is not a usable WARP account. */
        fun parse(json: String): Account? = runCatching {
            val o = JSONObject(json.trim())
            if (o.optString("private_key").isEmpty() || o.optString("peer_public_key").isEmpty()) null
            else Account(json.trim(), o)
        }.getOrNull()
    }
}

/** Parses "ip:port", "[v6]:port" or a bare IP (port 2408). */
fun parseEndpoint(text: String): Pair<String, Int>? {
    val s = text.trim()
    if (s.isEmpty()) return null
    if (s.startsWith("[")) {
        val end = s.indexOf(']')
        if (end < 0) return null
        val host = s.substring(1, end)
        val port = s.substring(end + 1).removePrefix(":").ifEmpty { "2408" }.toIntOrNull() ?: return null
        return (host to port).takeIf { port in 1..65535 }
    }
    val parts = s.split(':')
    return when (parts.size) {
        1 -> s to 2408
        2 -> parts[1].toIntOrNull()?.takeIf { it in 1..65535 }?.let { parts[0] to it }
        else -> s to 2408 // bare IPv6
    }
}
