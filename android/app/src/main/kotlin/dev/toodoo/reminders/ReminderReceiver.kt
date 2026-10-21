package dev.toodoo.reminders

import android.Manifest
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import dev.toodoo.MainActivity
import dev.toodoo.R
import dev.toodoo.TooDooApp
import dev.toodoo.api.ApiException
import java.time.ZoneId
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

/**
 * Shows a reminder when an alarm goes off, and runs its "Done" and
 * "Postpone 10 min" buttons. The buttons call the server without opening
 * the app.
 */
class ReminderReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val id = intent.data?.lastPathSegment?.toLongOrNull() ?: return
        val app = context.applicationContext as TooDooApp
        when (intent.action) {
            SHOW -> app.reminders.alarm(id)?.let { show(context, it) }
            DONE, POSTPONE -> {
                NotificationManagerCompat.from(context).cancel(id.toInt())
                val pending = goAsync()
                scope.launch {
                    try {
                        val session = app.sessions.session.first() ?: return@launch
                        val api = app.api(session)
                        val item = if (intent.action == DONE) api.setDone(id, true) else api.postpone(id, 10)
                        app.reminders.update(item)
                    } catch (e: ApiException) {
                        if (e.loggedOut) app.reminders.clear()
                    } catch (e: Exception) {
                        // No connection: the reminder is gone, the item stays as it was.
                    } finally {
                        pending.finish()
                    }
                }
            }
        }
    }

    private fun show(context: Context, alarm: Alarm) {
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED &&
            android.os.Build.VERSION.SDK_INT >= 33
        ) return
        val open = PendingIntent.getActivity(
            context, 0, Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP),
            PendingIntent.FLAG_IMMUTABLE,
        )
        val note = NotificationCompat.Builder(context, Reminders.CHANNEL)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(reminderTitle(alarm, ZoneId.systemDefault()))
            .setContentText(alarm.text)
            .setStyle(NotificationCompat.BigTextStyle().bigText(alarm.text))
            .setWhen(alarm.at)
            .setShowWhen(true)
            .setCategory(NotificationCompat.CATEGORY_REMINDER)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setContentIntent(open)
            .setAutoCancel(true)
            .addAction(0, "Done", button(context, DONE, alarm.id))
            .addAction(0, "Postpone 10 min", button(context, POSTPONE, alarm.id))
            .build()
        context.getSystemService(NotificationManager::class.java).notify(alarm.id.toInt(), note)
    }

    private fun button(context: Context, action: String, id: Long): PendingIntent =
        PendingIntent.getBroadcast(
            context, 0,
            Intent(context, ReminderReceiver::class.java).setAction(action).setData(Reminders.itemUri(id)),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )

    companion object {
        const val SHOW = "dev.toodoo.SHOW_REMINDER"
        const val DONE = "dev.toodoo.REMINDER_DONE"
        const val POSTPONE = "dev.toodoo.REMINDER_POSTPONE"
        private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    }
}
