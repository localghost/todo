package com.codeleet.toodoo

import com.codeleet.toodoo.ui.NewDue
import com.codeleet.toodoo.ui.newDue
import com.codeleet.toodoo.ui.newDuePassed
import com.codeleet.toodoo.ui.newDueText
import com.codeleet.toodoo.ui.notInTodayMessage
import com.codeleet.toodoo.ui.parseTypedDue
import java.io.File
import java.time.LocalDate
import java.time.LocalDateTime
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

// A due date on a new item (spec android-add-due-design.md, option D3).
class AddDueTest {
    // The web's own cases (internal/web/typed_due_test.go runs app.js on
    // them), so the phone reads the typed words as the web does.
    @Test
    fun typedDueAsOnTheWeb() {
        val data = Json.parseToJsonElement(File("../../internal/web/testdata/typed-due.json").readText()).jsonObject
        val now = LocalDateTime.parse(data.getValue("now").jsonPrimitive.content)
        val cases = data.getValue("cases").jsonArray
        assertTrue(cases.size > 20)
        val wrong = cases.mapNotNull { c ->
            val o = c.jsonObject
            val text = o.getValue("text").jsonPrimitive.content
            val m = parseTypedDue(text, now)
            val got = m?.let { text.substring(it.start, it.end) + " → " + it.date + " " + it.time } ?: "no date"
            val match = o.getValue("match")
            val want = if (match is JsonNull) "no date" else
                match.jsonPrimitive.content + " → " + o.getValue("date").jsonPrimitive.content + " " + o.getValue("time").jsonPrimitive.content
            if (got != want) "\"$text\": got $got, want $want" else null
        }
        assertEquals("", wrong.joinToString("\n"))
    }

    private val now = LocalDateTime.of(2026, 11, 19, 10, 0)
    private val today = now.toLocalDate()

    @Test
    fun typedDateWinsUntilRefused() {
        val text = "Call the dentist due tomorrow 10:00"
        val picked = NewDue(LocalDate.of(2026, 11, 24))
        val typed = newDue(text, picked, refused = null, now)!!
        assertEquals(NewDue(today.plusDays(1), "10:00", typed.typed), typed)
        assertEquals("Call the dentist", text.substring(0, typed.typed!!.start).trimEnd())
        // The × (or a picked date) says no to these words: the picked date counts.
        assertEquals(picked, newDue(text, picked, refused = typed.typed!!.words(text), now))
        assertNull(newDue(text, null, refused = "due  tomorrow 10:00".replace("  ", " "), now))
        // Other words are new words.
        assertEquals(today.plusDays(1), newDue("Call the dentist due tomorrow", null, refused = "due tomorrow 10:00", now)?.date)
    }

    @Test
    fun chipText() {
        assertEquals("Today", newDueText(NewDue(today), today))
        assertEquals("Tomorrow, 10:00", newDueText(NewDue(today.plusDays(1), "10:00"), today))
        assertEquals("Yesterday", newDueText(NewDue(today.minusDays(1)), today))
        assertEquals("24 Nov", newDueText(NewDue(LocalDate.of(2026, 11, 24)), today))
        assertEquals("12 Jan 2027", newDueText(NewDue(LocalDate.of(2027, 1, 12)), today))
    }

    @Test
    fun passed() {
        assertTrue(newDuePassed(NewDue(today.minusDays(1)), now))
        assertFalse(newDuePassed(NewDue(today), now))
        assertTrue(newDuePassed(NewDue(today, "09:30"), now))
        assertFalse(newDuePassed(NewDue(today, "10:30"), now))
    }

    @Test
    fun notInToday() {
        assertNull(notInTodayMessage(today, today))
        assertNull(notInTodayMessage(today.minusDays(2), today))
        assertEquals("Added. It is due tomorrow, so it is not in Today.", notInTodayMessage(today.plusDays(1), today))
        assertEquals("Added. It is due 24 Nov, so it is not in Today.", notInTodayMessage(LocalDate.of(2026, 11, 24), today))
    }
}
