package com.codeleet.toodoo.data

import android.content.Context
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.codeleet.toodoo.api.ApiClient
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

/** A logged-in user: the server, the bearer token, and the name to show. */
data class Session(val server: String, val token: String, val username: String)

private val Context.sessionData by preferencesDataStore("session")

private val SERVER = stringPreferencesKey("server")
private val TOKEN = stringPreferencesKey("token")
private val USERNAME = stringPreferencesKey("username")
private val GESTURE_HINT_SEEN = booleanPreferencesKey("gesture_hint_seen")

/** Keeps the session in the app's private storage (never in backups). */
class SessionStore(private val context: Context) {
    /** The session, or null when nobody is logged in. */
    val session: Flow<Session?> = context.sessionData.data.map { p ->
        p[TOKEN]?.let { Session(p[SERVER] ?: ApiClient.DEFAULT_SERVER, it, p[USERNAME].orEmpty()) }
    }

    /** The server of the last log-in, for the log-in screen. */
    val lastServer: Flow<String> = context.sessionData.data.map { it[SERVER] ?: ApiClient.DEFAULT_SERVER }

    /** The username of the last log-in, for the log-in screen. */
    val lastUsername: Flow<String> = context.sessionData.data.map { it[USERNAME].orEmpty() }

    /** Whether the user closed the hint about swipe and hold on the list. */
    val gestureHintSeen: Flow<Boolean> = context.sessionData.data.map { it[GESTURE_HINT_SEEN] == true }

    suspend fun setGestureHintSeen() {
        context.sessionData.edit { it[GESTURE_HINT_SEEN] = true }
    }

    suspend fun save(s: Session) {
        context.sessionData.edit {
            it[SERVER] = s.server
            it[TOKEN] = s.token
            it[USERNAME] = s.username
        }
    }

    /** Logs out: the token goes, the server and username stay for the next log-in. */
    suspend fun clear() {
        context.sessionData.edit { it.remove(TOKEN) }
    }
}
