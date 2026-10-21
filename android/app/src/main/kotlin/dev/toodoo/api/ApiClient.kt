package dev.toodoo.api

import java.io.IOException
import java.time.ZoneId
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.add
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonArray
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

/** The server refused a request. [message] is the text to show the user. */
class ApiException(val status: Int, val code: String, override val message: String) : Exception(message) {
    val loggedOut get() = status == 401 && code == "logged_out"
}

/** The server could not be reached. */
class OfflineException(cause: Throwable) : Exception("No connection to the server.", cause)

val json = Json { ignoreUnknownKeys = true }

private val jsonType = "application/json".toMediaType()

/**
 * Calls the TooDoo API on [server] (for example https://toodoo.fly.dev) with
 * the log-in [token] (null before the log-in).
 */
class ApiClient(private val http: OkHttpClient, server: String, private val token: String?) {
    private val base: HttpUrl = checkServer(server) ?: throw IllegalArgumentException("bad server address")

    suspend fun login(username: String, password: String): LoginResult =
        json.decodeFromString(call("POST", "login", buildJsonObject {
            put("username", username)
            put("password", password)
        })!!)

    suspend fun logout() {
        call("POST", "logout")
    }

    suspend fun items(labelId: Long? = null): ItemList =
        json.decodeFromString(call("GET", "items", query = labelId?.let { mapOf("label" to it.toString()) })!!)

    suspend fun addItem(text: String, labels: List<String> = emptyList()): Item =
        item("POST", "items", buildJsonObject {
            put("text", text)
            putJsonArray("labels") { labels.forEach { add(it) } }
        })

    suspend fun setDone(id: Long, done: Boolean): Item =
        item("PATCH", "items/$id", buildJsonObject { put("done", done) })

    suspend fun setText(id: Long, text: String): Item =
        item("PATCH", "items/$id", buildJsonObject { put("text", text) })

    suspend fun deleteItem(id: Long) {
        call("DELETE", "items/$id")
    }

    suspend fun setHideDone(hide: Boolean): User =
        json.decodeFromString(call("PUT", "settings", buildJsonObject { put("hide_done", hide) })!!)

    suspend fun clearDone(labelId: Long? = null): Int {
        val body = call("POST", "items/done/delete", query = labelId?.let { mapOf("label" to it.toString()) })!!
        return json.decodeFromString<Map<String, Int>>(body)["deleted"] ?: 0
    }

    suspend fun item(id: Long): Item = json.decodeFromString(call("GET", "items/$id")!!)

    /** Sets the due date ("2026-10-22") and time ("14:30", "1430", or "" for all day); both "" clear it. */
    suspend fun setDue(id: Long, date: String, time: String): Item =
        item("PUT", "items/$id/due", buildJsonObject {
            put("date", date)
            put("time", time)
        })

    suspend fun postpone(id: Long, minutes: Int): Item =
        item("POST", "items/$id/postpone", buildJsonObject { put("minutes", minutes) })

    /** Puts the item right after item [after] (0: first). */
    suspend fun move(id: Long, after: Long) {
        call("POST", "items/$id/move", buildJsonObject { put("after", after) })
    }

    /** Adds the label [name] to the item, making the label if needed. */
    suspend fun addLabel(id: Long, name: String): Item =
        item("POST", "items/$id/labels", buildJsonObject { put("name", name) })

    suspend fun removeLabel(id: Long, labelId: Long): Item =
        json.decodeFromString(call("DELETE", "items/$id/labels/$labelId")!!)

    suspend fun labels(): List<Label> = json.decodeFromString(call("GET", "labels")!!)

    suspend fun setLabelColor(labelId: Long, color: String) {
        call("PUT", "labels/$labelId/color", buildJsonObject { put("color", color) })
    }

    suspend fun deleteLabel(labelId: Long) {
        call("DELETE", "labels/$labelId")
    }

    private suspend fun item(method: String, path: String, body: JsonObject): Item =
        json.decodeFromString(call(method, path, body)!!)

    /** Sends one request and returns the body, or null for 204. */
    private suspend fun call(
        method: String,
        path: String,
        body: JsonObject? = null,
        query: Map<String, String>? = null,
    ): String? = withContext(Dispatchers.IO) {
        val url = base.newBuilder().addPathSegments("api/v1/$path").apply {
            query?.forEach { (k, v) -> addQueryParameter(k, v) }
        }.build()
        val request = Request.Builder().url(url)
            .header("Accept", "application/json")
            .header("Time-Zone", ZoneId.systemDefault().id)
            .apply { if (token != null) header("Authorization", "Bearer $token") }
            .method(method, body?.toString()?.toRequestBody(jsonType) ?: if (method == "GET") null else ByteArray(0).toRequestBody(jsonType))
            .build()
        val response = try {
            http.newCall(request).execute()
        } catch (e: IOException) {
            throw OfflineException(e)
        }
        response.use {
            val text = try {
                it.body?.string().orEmpty()
            } catch (e: IOException) {
                throw OfflineException(e)
            }
            if (!it.isSuccessful) {
                val err = runCatching { json.decodeFromString<ErrorBody>(text) }.getOrNull()
                throw ApiException(it.code, err?.error.orEmpty(), err?.message?.ifEmpty { null } ?: "The server answered ${it.code}.")
            }
            if (it.code == 204) null else text
        }
    }

    companion object {
        const val DEFAULT_SERVER = "https://toodoo.fly.dev"

        /**
         * Returns the server address as a URL, or null if it is not allowed:
         * only https, except plain http to the emulator's host and this phone
         * (res/xml/network_security_config.xml). No scheme means https.
         */
        fun checkServer(server: String): HttpUrl? {
            var s = server.trim().trimEnd('/')
            if ("://" !in s) s = "https://$s"
            val url = s.toHttpUrlOrNull() ?: return null
            val local = url.host in setOf("10.0.2.2", "127.0.0.1", "localhost")
            if (!url.isHttps && !local) return null
            return url
        }
    }
}
