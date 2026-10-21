package dev.toodoo.ui

import dev.toodoo.api.Item
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.util.Locale

/** The due line of a row: its text, and whether it shows as overdue. */
data class DueLabel(val text: String, val overdue: Boolean)

private val dayMonth = DateTimeFormatter.ofPattern("d MMM", Locale.ENGLISH)
private val dayMonthYear = DateTimeFormatter.ofPattern("d MMM yyyy", Locale.ENGLISH)
private val clock = DateTimeFormatter.ofPattern("HH:mm", Locale.ENGLISH)

/**
 * Returns the due label of an item in the phone's zone, worded like the web
 * (internal/web/due.go): "Due today, 14:30", "Overdue since yesterday", or
 * null without a due date. "Overdue" comes from the server.
 */
fun dueLabel(item: Item, now: ZonedDateTime = ZonedDateTime.now()): DueLabel? {
    val at = item.dueAt ?: return null
    val due = runCatching { Instant.parse(at) }.getOrNull()?.atZone(now.zone) ?: return null
    var whenText = dayName(due.toLocalDate(), now.toLocalDate())
    if (!item.dueAllDay) whenText += ", " + due.format(clock)
    return if (item.overdue && !item.done) DueLabel("Overdue since $whenText", true) else DueLabel("Due $whenText", false)
}

/** Names a day relative to today: today, tomorrow, yesterday, or a date. */
fun dayName(d: LocalDate, today: LocalDate): String = when {
    d == today -> "today"
    d == today.plusDays(1) -> "tomorrow"
    d == today.minusDays(1) -> "yesterday"
    d.year == today.year -> d.format(dayMonth)
    else -> d.format(dayMonthYear)
}

/** The phone's zone. */
fun phoneZone(): ZoneId = ZoneId.systemDefault()
