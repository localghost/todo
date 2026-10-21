package dev.toodoo.reminders

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import dev.toodoo.TooDooApp
import dev.toodoo.api.ApiException
import dev.toodoo.api.OfflineException
import kotlinx.coroutines.flow.first

/** Loads the whole list in the background and sets the alarms for it. */
class SyncWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {
    override suspend fun doWork(): Result {
        val app = applicationContext as TooDooApp
        val session = app.sessions.session.first() ?: return Result.success()
        return try {
            app.reminders.sync(app.api(session).items().items)
            Result.success()
        } catch (e: OfflineException) {
            Result.retry()
        } catch (e: ApiException) {
            if (e.loggedOut) app.reminders.clear()
            Result.success()
        }
    }
}
