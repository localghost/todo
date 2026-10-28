package com.codeleet.toodoo

import android.app.Application
import com.codeleet.toodoo.api.ApiClient
import com.codeleet.toodoo.data.ListCache
import com.codeleet.toodoo.data.Session
import com.codeleet.toodoo.data.SessionStore
import com.codeleet.toodoo.reminders.Reminders
import java.util.concurrent.TimeUnit
import okhttp3.OkHttpClient

/** Holds the app's single instances. */
class TooDooApp : Application() {
    lateinit var sessions: SessionStore
        private set
    lateinit var cache: ListCache
        private set
    lateinit var reminders: Reminders
        private set
    private val http = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .callTimeout(30, TimeUnit.SECONDS)
        .build()

    override fun onCreate() {
        super.onCreate()
        sessions = SessionStore(this)
        cache = ListCache(filesDir)
        reminders = Reminders(this)
        Reminders.createChannel(this)
    }

    fun api(session: Session) = ApiClient(http, session.server, session.token)

    /** A client without a token, for the log-in. Null if [server] is not allowed. */
    fun loginApi(server: String): ApiClient? =
        ApiClient.checkServer(server)?.let { ApiClient(http, it.toString(), null) }
}
