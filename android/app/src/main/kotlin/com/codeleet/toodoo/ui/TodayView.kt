package com.codeleet.toodoo.ui

import com.codeleet.toodoo.api.Item
import com.codeleet.toodoo.api.ItemList
import com.codeleet.toodoo.api.Label
import java.time.Duration
import java.time.Instant
import java.time.ZonedDateTime

// The Today view (issue #15, canvas page "22 · Today", option A1). The server
// makes the view; these rules (the same as internal/todo/today.go) are for the
// saved list when there is no connection.

/** Whether the item is in the Today view at [now]: open and due today or earlier, or done and due today. */
fun inToday(item: Item, now: ZonedDateTime): Boolean {
    val at = item.dueAt ?: return false
    val due = runCatching { Instant.parse(at) }.getOrNull()?.atZone(now.zone)?.toLocalDate() ?: return false
    val today = now.toLocalDate()
    return if (item.done) due == today else !due.isAfter(today)
}

/**
 * The saved whole list in a view: only the items of the Today view if [today],
 * and only those with [label] if given. With done items hidden the saved list
 * has none, so the done counter of a view is then 0.
 */
fun ItemList.viewOf(today: Boolean, label: Label?, now: ZonedDateTime): ItemList {
    if (!today && label == null) return this
    val shown = items.filter { item ->
        (!today || inToday(item, now)) && (label == null || item.labels.any { it.id == label.id })
    }
    return copy(items = shown, open = shown.count { !it.done }, done = shown.count { it.done }, label = label)
}

/** How long until the next day starts at [now]'s place. */
fun millisToNextDay(now: ZonedDateTime): Long {
    val next = now.toLocalDate().plusDays(1).atStartOfDay(now.zone)
    return Duration.between(now, next).toMillis()
}
