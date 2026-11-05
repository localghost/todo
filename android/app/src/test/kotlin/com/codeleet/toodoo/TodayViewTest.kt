package com.codeleet.toodoo

import com.codeleet.toodoo.api.Item
import com.codeleet.toodoo.api.ItemList
import com.codeleet.toodoo.api.Label
import com.codeleet.toodoo.ui.inToday
import com.codeleet.toodoo.ui.millisToNextDay
import com.codeleet.toodoo.ui.viewOf
import java.time.ZoneId
import java.time.ZonedDateTime
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The Today view on the phone, for the saved list when there is no connection.
 * The rules are the same as on the server (internal/todo/today.go).
 */
class TodayViewTest {
    private val warsaw = ZoneId.of("Europe/Warsaw")
    private val now = ZonedDateTime.of(2026, 11, 5, 10, 0, 0, 0, warsaw)
    private val work = Label(1, "work", "#2555da")

    private fun item(id: Long, due: String?, done: Boolean = false, labels: List<Label> = emptyList()) =
        Item(id, "item $id", done, dueAt = due, labels = labels)

    private val yesterday = "2026-11-04T17:00:00Z"
    private val laterToday = "2026-11-05T22:30:00Z" // 23:30 in Warsaw
    private val tomorrow = "2026-11-05T23:30:00Z" // 00:30 on 6 Nov in Warsaw

    @Test
    fun openOverdueIsIn() = assertTrue(inToday(item(1, yesterday), now))

    @Test
    fun openLaterTodayIsIn() = assertTrue(inToday(item(1, laterToday), now))

    // In UTC this is still 5 Nov, but on the phone it is tomorrow.
    @Test
    fun openTomorrowIsOut() = assertFalse(inToday(item(1, tomorrow), now))

    @Test
    fun noDateIsOut() = assertFalse(inToday(item(1, null), now))

    @Test
    fun doneTodayIsIn() = assertTrue(inToday(item(1, laterToday, done = true), now))

    @Test
    fun doneYesterdayIsOut() = assertFalse(inToday(item(1, yesterday, done = true), now))

    @Test
    fun badDateIsOut() = assertFalse(inToday(item(1, "soon"), now))

    private val saved = ItemList(
        listOf(
            item(1, yesterday),
            item(2, laterToday, labels = listOf(work)),
            item(3, null),
            item(4, tomorrow, labels = listOf(work)),
            item(5, laterToday, done = true),
            item(6, yesterday, done = true),
        ),
        open = 4, done = 2, hideDone = false,
    )

    @Test
    fun wholeListStaysTheSame() = assertSame(saved, saved.viewOf(today = false, label = null, now))

    @Test
    fun todayCountsItsItems() {
        val shown = saved.viewOf(today = true, label = null, now)
        assertEquals(listOf(1L, 2L, 5L), shown.items.map { it.id })
        assertEquals(2, shown.open)
        assertEquals(1, shown.done)
    }

    @Test
    fun todayWithLabel() {
        val shown = saved.viewOf(today = true, label = work, now)
        assertEquals(listOf(2L), shown.items.map { it.id })
        assertEquals(1, shown.open)
        assertEquals(0, shown.done)
        assertEquals(work, shown.label)
    }

    // All view, label filter: the saved list has no filter of its own.
    @Test
    fun labelOnly() {
        val shown = saved.viewOf(today = false, label = work, now)
        assertEquals(listOf(2L, 4L), shown.items.map { it.id })
    }

    @Test
    fun nextDayIsAtMidnight() {
        assertEquals(14 * 3_600_000L, millisToNextDay(now))
    }

    // On 25 Oct 2026 in Warsaw the clocks go back, so that day has 25 hours.
    @Test
    fun nextDayKnowsTheClockChange() {
        val morning = ZonedDateTime.of(2026, 10, 25, 0, 0, 0, 0, warsaw)
        assertEquals(25 * 3_600_000L, millisToNextDay(morning))
    }
}
