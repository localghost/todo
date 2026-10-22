package dev.toodoo

import dev.toodoo.api.Item
import dev.toodoo.ui.clockText
import dev.toodoo.ui.dateButtonText
import dev.toodoo.ui.dueFields
import dev.toodoo.ui.hslToHex
import dev.toodoo.ui.moveBy
import dev.toodoo.ui.pickerStart
import dev.toodoo.ui.timeButtonText
import dev.toodoo.ui.wheelColor
import dev.toodoo.ui.wheelPick
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SheetAndWheelTest {
    private val warsaw = ZoneId.of("Europe/Warsaw")

    @Test
    fun dueFieldsInThePhonesZone() {
        assertEquals(Pair(LocalDate.of(2026, 10, 22), "14:30"), dueFields(Item(1, "a", false, dueAt = "2026-10-22T12:30:00Z"), warsaw))
        assertEquals(Pair(LocalDate.of(2026, 10, 22), ""), dueFields(Item(1, "a", false, dueAt = "2026-10-22T07:00:00Z", dueAllDay = true), warsaw))
        assertEquals(Pair(null, ""), dueFields(Item(1, "a", false), warsaw))
    }

    @Test
    fun dateButtonShowsTheDateOrDate() {
        val today = LocalDate.of(2026, 10, 21)
        assertEquals("Date", dateButtonText(null, today))
        assertEquals("Wed 21 Oct", dateButtonText(today, today))
        assertEquals("21 Oct 2027", dateButtonText(LocalDate.of(2027, 10, 21), today))
    }

    @Test
    fun timeButtonShowsTheTimeOrAllDay() {
        assertEquals("Time", timeButtonText("", hasDate = false))
        assertEquals("All day", timeButtonText("", hasDate = true))
        assertEquals("14:30", timeButtonText("14:30", hasDate = true))
    }

    @Test
    fun pickerStartsAtTheTimeOrTheNextHour() {
        assertEquals(Pair(14, 30), pickerStart("14:30", LocalTime.of(9, 10)))
        assertEquals(Pair(10, 0), pickerStart("", LocalTime.of(9, 10)))
        assertEquals(Pair(10, 0), pickerStart("", LocalTime.of(9, 0)))
        assertEquals(Pair(0, 0), pickerStart("", LocalTime.of(23, 30)))
    }

    @Test
    fun clockTextHasTwoDigits() {
        assertEquals("09:05", clockText(9, 5))
        assertEquals("00:00", clockText(0, 0))
        assertEquals("23:59", clockText(23, 59))
    }

    @Test
    fun wheelLikeTheWeb() {
        // static/wheel.js: hue 0 at the top, clockwise; the middle is gray.
        assertEquals(Pair(0.0, 1.0), wheelPick(0.0, -0.5))
        assertEquals(90.0, wheelPick(0.5, 0.0).first, 1e-9)
        assertEquals(180.0, wheelPick(0.0, 0.25).first, 1e-9)
        assertEquals(0.5, wheelPick(0.0, 0.25).second, 1e-9)
        assertEquals(1.0, wheelPick(0.5, 0.5).second, 1e-9)
        assertEquals("#808080", wheelColor(123.0, 0.0))
        assertEquals("#ff0000", wheelColor(0.0, 1.0))
        assertEquals("#2555da", hslToHex(224.0, 0.71, 0.5))
    }

    @Test
    fun moveStepsKeepOverdueOnTop() {
        val late = Item(1, "late", false, overdue = true)
        val a = Item(2, "a", false)
        val b = Item(3, "b", false)
        val shown = listOf(late, a, b)
        assertEquals(listOf(1L, 3L, 2L), moveBy(shown, a, 1))
        assertEquals(listOf(1L, 3L, 2L), moveBy(shown, b, -1))
        assertNull(moveBy(shown, a, -1)) // not above an overdue item
        assertNull(moveBy(shown, late, 1))
        assertNull(moveBy(shown, b, 1))
    }
}
