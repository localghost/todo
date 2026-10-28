package com.codeleet.toodoo.reminders

import android.app.AlarmManager
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import com.codeleet.toodoo.api.Item
import com.codeleet.toodoo.api.json
import java.io.File
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.concurrent.TimeUnit
import kotlinx.serialization.Serializable

/** One reminder: the item, its text, and when it is due (epoch milliseconds). */
@Serializable
data class Alarm(val id: Long, val text: String, val at: Long, val allDay: Boolean)

/** At most this many alarms are set, the soonest ones (Android allows 500 per app). */
const val MAX_ALARMS = 100

/** Returns the alarms for [items]: open items due after [now], soonest first. */
fun alarmsFor(items: List<Item>, now: Instant): List<Alarm> = items.mapNotNull { item ->
    val at = item.dueAt?.let { runCatching { Instant.parse(it) }.getOrNull() }
    if (item.done || at == null || !at.isAfter(now)) null else Alarm(item.id, item.text, at.toEpochMilli(), item.dueAllDay)
}.sortedBy { it.at }.take(MAX_ALARMS)

private val clock = DateTimeFormatter.ofPattern("HH:mm")

/** The title of a reminder, as in the mockup: "Due now: 14:30", or "Due today" for an all-day item (at 09:00). */
fun reminderTitle(alarm: Alarm, zone: ZoneId): String =
    if (alarm.allDay) "Due today" else "Due now: " + Instant.ofEpochMilli(alarm.at).atZone(zone).format(clock)

/**
 * Sets an alarm for each open item with a due time in the future (spec
 * section 4.2). The phone notifies by itself, without asking the server.
 * The alarms that are set are kept in a file, to cancel them when items
 * change and to set them again after a reboot.
 */
class Reminders(private val context: Context) {
    private val file = File(context.filesDir, "alarms.json")
    private val alarms = context.getSystemService(AlarmManager::class.java)
    private val lock = Any()

    /** Sets the alarms for the whole list [items]; alarms of other items are cancelled. */
    fun sync(items: List<Item>) = synchronized(lock) {
        val wanted = alarmsFor(items, Instant.now())
        val keep = wanted.toSet()
        load().filterNot { it in keep }.forEach { cancel(it.id) }
        wanted.forEach { set(it) }
        save(wanted)
    }

    /** Sets the alarm of one changed item (from a notification button), keeping the others. */
    fun update(item: Item) = synchronized(lock) {
        cancel(item.id)
        val others = load().filterNot { it.id == item.id }
        val mine = alarmsFor(listOf(item), Instant.now())
        mine.forEach { set(it) }
        save((others + mine).sortedBy { it.at })
    }

    /** Sets the kept alarms again, after a reboot or an update of the app. */
    fun restore() = synchronized(lock) {
        val now = System.currentTimeMillis()
        val future = load().filter { it.at > now }
        future.forEach { set(it) }
        save(future)
    }

    /** Cancels all alarms and the background loads (log-out). */
    fun clear() = synchronized(lock) {
        load().forEach { cancel(it.id) }
        file.delete()
        WorkManager.getInstance(context).cancelUniqueWork(PERIODIC)
        WorkManager.getInstance(context).cancelUniqueWork(ONCE)
    }

    /** Whether the alarms ring on time; otherwise Android may delay them by a few minutes. */
    fun exact(): Boolean = Build.VERSION.SDK_INT < 31 || alarms.canScheduleExactAlarms()

    /** Loads the list every 30 minutes in the background, so due dates set on the web also remind. */
    fun startBackgroundLoads() {
        val request = PeriodicWorkRequestBuilder<SyncWorker>(30, TimeUnit.MINUTES).setConstraints(online).build()
        WorkManager.getInstance(context).enqueueUniquePeriodicWork(PERIODIC, ExistingPeriodicWorkPolicy.KEEP, request)
    }

    /** Loads the whole list soon (the screen shows a filtered one) and sets the alarms. */
    fun syncSoon() {
        val request = OneTimeWorkRequestBuilder<SyncWorker>().setConstraints(online).build()
        WorkManager.getInstance(context).enqueueUniqueWork(ONCE, ExistingWorkPolicy.REPLACE, request)
    }

    private fun set(alarm: Alarm) {
        val intent = alarmIntent(alarm.id) ?: return
        if (exact()) {
            alarms.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, alarm.at, intent)
        } else {
            alarms.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, alarm.at, intent)
        }
    }

    private fun cancel(id: Long) {
        alarmIntent(id, create = false)?.let { alarms.cancel(it) }
    }

    private fun alarmIntent(id: Long, create: Boolean = true): PendingIntent? {
        val intent = Intent(context, ReminderReceiver::class.java)
            .setAction(ReminderReceiver.SHOW)
            .setData(itemUri(id))
        val flags = PendingIntent.FLAG_IMMUTABLE or if (create) PendingIntent.FLAG_UPDATE_CURRENT else PendingIntent.FLAG_NO_CREATE
        return PendingIntent.getBroadcast(context, 0, intent, flags)
    }

    /** The alarm of item [id], or null if none is set. */
    fun alarm(id: Long): Alarm? = synchronized(lock) { load().find { it.id == id } }

    private fun load(): List<Alarm> =
        runCatching { json.decodeFromString<List<Alarm>>(file.readText()) }.getOrDefault(emptyList())

    private fun save(list: List<Alarm>) {
        val tmp = File(file.parentFile, file.name + ".tmp")
        tmp.writeText(json.encodeToString(kotlinx.serialization.builtins.ListSerializer(Alarm.serializer()), list))
        tmp.renameTo(file)
    }

    companion object {
        const val CHANNEL = "reminders"
        private const val PERIODIC = "load-list"
        private const val ONCE = "load-list-now"
        private val online = Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build()

        /** Each item's alarm and notification are told apart by this address. */
        fun itemUri(id: Long): Uri = Uri.parse("toodoo://item/$id")

        /** Makes the notification channel (Android 8+ needs one). */
        fun createChannel(context: Context) {
            val channel = NotificationChannel(CHANNEL, "Reminders", NotificationManager.IMPORTANCE_HIGH).apply {
                description = "When an item is due"
            }
            context.getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
        }
    }
}
