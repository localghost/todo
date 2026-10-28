package com.codeleet.toodoo

import com.codeleet.toodoo.api.Item
import com.codeleet.toodoo.ui.dueLabel
import java.time.ZoneId
import java.time.ZonedDateTime
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class DueTextTest {
    private val warsaw = ZoneId.of("Europe/Warsaw")
    private val now = ZonedDateTime.of(2026, 10, 21, 12, 0, 0, 0, warsaw)

    private fun item(dueAt: String?, allDay: Boolean = false, overdue: Boolean = false, done: Boolean = false) =
        Item(id = 1, text = "x", done = done, dueAt = dueAt, dueAllDay = allDay, overdue = overdue)

    @Test
    fun timedToday() {
        // 12:30 UTC is 14:30 in Warsaw in October (UTC+2).
        assertEquals("Due today, 14:30", dueLabel(item("2026-10-21T12:30:00Z"), now)!!.text)
    }

    @Test
    fun allDayTomorrowAndLater() {
        assertEquals("Due tomorrow", dueLabel(item("2026-10-22T07:00:00Z", allDay = true), now)!!.text)
        assertEquals("Due 24 Oct", dueLabel(item("2026-10-24T07:00:00Z", allDay = true), now)!!.text)
        assertEquals("Due 5 Jan 2027", dueLabel(item("2027-01-05T08:00:00Z", allDay = true), now)!!.text)
    }

    @Test
    fun overdueComesFromTheServer() {
        val l = dueLabel(item("2026-10-20T16:00:00Z", overdue = true), now)!!
        assertEquals("Overdue since yesterday, 18:00", l.text)
        assertEquals(true, l.overdue)
    }

    @Test
    fun noDueDate() {
        assertNull(dueLabel(item(null), now))
    }
}
