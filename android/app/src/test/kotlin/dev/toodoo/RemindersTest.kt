package dev.toodoo

import dev.toodoo.api.Item
import dev.toodoo.reminders.Alarm
import dev.toodoo.reminders.alarmsFor
import dev.toodoo.reminders.reminderTitle
import java.time.Instant
import java.time.ZoneId
import org.junit.Assert.assertEquals
import org.junit.Test

class RemindersTest {
    private val now = Instant.parse("2026-10-21T12:00:00Z")

    @Test
    fun onlyOpenItemsDueLaterGetAnAlarm() {
        val items = listOf(
            Item(1, "later", false, dueAt = "2026-10-22T08:00:00Z"),
            Item(2, "soon", false, dueAt = "2026-10-21T12:30:00Z"),
            Item(3, "done", true, dueAt = "2026-10-22T08:00:00Z"),
            Item(4, "past", false, dueAt = "2026-10-21T11:00:00Z", overdue = true),
            Item(5, "no date", false),
            Item(6, "all day", false, dueAt = "2026-10-23T07:00:00Z", dueAllDay = true),
        )
        assertEquals(listOf(2L, 1L, 6L), alarmsFor(items, now).map { it.id })
        assertEquals(true, alarmsFor(items, now).last().allDay)
    }

    @Test
    fun titles() {
        val warsaw = ZoneId.of("Europe/Warsaw")
        val at = Instant.parse("2026-10-21T12:30:00Z").toEpochMilli()
        assertEquals("Due now: 14:30", reminderTitle(Alarm(1, "x", at, false), warsaw))
        assertEquals("Due today", reminderTitle(Alarm(1, "x", at, true), warsaw))
    }
}
