package dev.toodoo.reminders

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import dev.toodoo.TooDooApp

/** Sets the alarms again after a reboot, an app update, or a change of the exact-alarm permission. */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            Intent.ACTION_BOOT_COMPLETED, Intent.ACTION_MY_PACKAGE_REPLACED,
            "android.app.action.SCHEDULE_EXACT_ALARM_PERMISSION_STATE_CHANGED" -> {
                val reminders = (context.applicationContext as TooDooApp).reminders
                reminders.restore()
                reminders.syncSoon()
            }
        }
    }
}
