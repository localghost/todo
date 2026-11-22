package com.codeleet.toodoo.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.DateRange
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.InputChip
import androidx.compose.material3.InputChipDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.OffsetMapping
import androidx.compose.ui.text.input.TransformedText
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.PopupProperties
import java.time.DateTimeException
import java.time.LocalDate
import java.time.LocalDateTime
import java.time.LocalTime

// A due date on a new item (spec /design/android-add-due-design.md, option D3;
// canvas page "25 · Android due date on add"): the "Date" chip first in the
// row under the add field, and the date typed at the end of the text.

/**
 * A due date typed at the end of the add field: the place of the date words
 * in the text ([start], [end]), the day, and the time ("HH:MM", "" all day).
 */
data class TypedDue(val start: Int, val end: Int, val date: LocalDate, val time: String) {
    /** The date words, as the "no" to them is kept: lower case, one space between words. */
    fun words(text: String): String = text.substring(start, end).lowercase().replace(spaces, " ")
}

private val spaces = Regex("\\s+")
private val marker = Regex("(?<=^|\\s)(?:due\\s+|@\\s*)", RegexOption.IGNORE_CASE)
private val clockWord = Regex("^([01]?\\d|2[0-3]):([0-5]\\d)$")
private val dotDate = Regex("^(\\d{1,2})\\.(\\d{1,2})(?:\\.(\\d{4}))?$")
private val dayNumber = Regex("^\\d{1,2}$")
private val year = Regex("^\\d{4}$")

private val typedDays = mapOf(
    "sun" to 0, "sunday" to 0, "mon" to 1, "monday" to 1, "tue" to 2, "tuesday" to 2, "wed" to 3, "wednesday" to 3,
    "thu" to 4, "thursday" to 4, "fri" to 5, "friday" to 5, "sat" to 6, "saturday" to 6,
)
private val typedMonths = mapOf(
    "jan" to 1, "january" to 1, "feb" to 2, "february" to 2, "mar" to 3, "march" to 3, "apr" to 4, "april" to 4, "may" to 5,
    "jun" to 6, "june" to 6, "jul" to 7, "july" to 7, "aug" to 8, "august" to 8, "sep" to 9, "sept" to 9, "september" to 9,
    "oct" to 10, "october" to 10, "nov" to 11, "november" to 11, "dec" to 12, "december" to 12,
)

/**
 * Finds a due date at the end of [text]: after "due" or "@" at the start of a
 * word, with some text before it. The same rules as parseTypedDue in
 * internal/web/static/app.js, tested on the same cases
 * (internal/web/testdata/typed-due.json). [now] is the phone's local time.
 */
fun parseTypedDue(text: String, now: LocalDateTime): TypedDue? {
    for (m in marker.findAll(text)) {
        if (text.substring(0, m.range.first).isBlank()) continue
        val (date, time) = typedWhen(text.substring(m.range.last + 1), now) ?: continue
        return TypedDue(m.range.first, text.trimEnd().length, date, time)
    }
    return null
}

/** Reads the words after "due" or "@": a day word, a weekday or a date, then a time; or only a time (today). */
private fun typedWhen(rest: String, now: LocalDateTime): Pair<LocalDate, String>? {
    val words = rest.trim().split(spaces).map { it.lowercase() }.toMutableList()
    if (words[0].isEmpty()) return null
    var time = ""
    clockWord.find(words.last())?.let {
        time = it.groupValues[1].padStart(2, '0') + ":" + it.groupValues[2]
        words.removeAt(words.size - 1)
    }
    val today = now.toLocalDate()
    val day = when {
        words.isEmpty() -> if (time.isNotEmpty()) today else null
        words.size == 1 && words[0] == "today" -> today
        words.size == 1 && words[0] == "tomorrow" -> today.plusDays(1)
        words.size == 1 && words[0] in typedDays -> {
            // The next such day. Today only with a time later than now.
            var ahead = (typedDays.getValue(words[0]) - today.dayOfWeek.value % 7 + 7) % 7
            if (ahead == 0 && !(time.isNotEmpty() && LocalTime.parse(time).isAfter(now.toLocalTime().withSecond(0).withNano(0)))) ahead = 7
            today.plusDays(ahead.toLong())
        }
        else -> typedDate(words, today.year)
    } ?: return null
    return Pair(day, time)
}

/** Reads "12.11", "12.11.2027", "12 Nov", "Nov 12", each with an optional year. A day that does not exist gives null. */
private fun typedDate(words: List<String>, thisYear: Int): LocalDate? {
    var y = thisYear
    val d: Int
    val mo: Int
    val dots = if (words.size == 1) dotDate.find(words[0]) else null
    if (dots != null) {
        d = dots.groupValues[1].toInt()
        mo = dots.groupValues[2].toInt()
        if (dots.groupValues[3].isNotEmpty()) y = dots.groupValues[3].toInt()
    } else if (words.size == 2 || words.size == 3) {
        val num = { w: String -> if (dayNumber.matches(w)) w.toInt() else 0 }
        if (num(words[0]) != 0 && words[1] in typedMonths) {
            d = num(words[0])
            mo = typedMonths.getValue(words[1])
        } else if (words[0] in typedMonths && num(words[1]) != 0) {
            d = num(words[1])
            mo = typedMonths.getValue(words[0])
        } else {
            return null
        }
        if (words.size == 3) {
            if (!year.matches(words[2])) return null
            y = words[2].toInt()
        }
    } else {
        return null
    }
    return try {
        LocalDate.of(y, mo, d)
    } catch (_: DateTimeException) {
        null
    }
}

/** The due date of the new item: the day, the time ("" all day), and the typed words it comes from, if it does. */
data class NewDue(val date: LocalDate, val time: String = "", val typed: TypedDue? = null)

/**
 * The due date that the new item gets: the date typed in [text], unless the
 * user said no to these words ([refused]), else the [picked] one.
 */
fun newDue(text: String, picked: NewDue?, refused: String?, now: LocalDateTime): NewDue? {
    val typed = parseTypedDue(text, now)
    if (typed != null && typed.words(text) != refused) return NewDue(typed.date, typed.time, typed)
    return picked
}

/** The chip's text, in the words of the rows: "Tomorrow", "23 Nov", "12 Jan 2027", with ", 10:00" for a time. */
fun newDueText(due: NewDue, today: LocalDate): String =
    dayName(due.date, today).replaceFirstChar { it.uppercase() } + if (due.time.isEmpty()) "" else ", " + due.time

/** Whether the due date has passed (a past day, or a time earlier than [now]): the chip is red then. */
fun newDuePassed(due: NewDue, now: LocalDateTime): Boolean =
    if (due.time.isEmpty()) due.date.isBefore(now.toLocalDate()) else LocalDateTime.of(due.date, LocalTime.parse(due.time)).isBefore(now)

/** The snackbar after an item added in the Today view for a later day, which does not show in the view. */
fun notInTodayMessage(dueDate: LocalDate, today: LocalDate): String? =
    if (dueDate.isAfter(today)) "Added. It is due ${dayName(dueDate, today)}, so it is not in Today." else null

/** Draws the typed date words of the add field in blue, underlined, as the web marks them. */
data class TypedDueMark(val start: Int, val end: Int, val color: Color) : VisualTransformation {
    override fun filter(text: AnnotatedString): TransformedText {
        val to = end.coerceAtMost(text.length)
        val from = start.coerceAtMost(to)
        val marked = AnnotatedString.Builder(text).apply {
            addStyle(SpanStyle(color = color, textDecoration = TextDecoration.Underline), from, to)
        }.toAnnotatedString()
        return TransformedText(marked, OffsetMapping.Identity)
    }
}

/**
 * The first chip of the row under the add field. Without a [due] date it is
 * an outlined "Date"; with one it is blue (red if [passed]) with a ×. A tap
 * opens the quick choices: Today, Tomorrow, Pick a date…, and with a date
 * also Set a time…. The × calls [onClear].
 */
@Composable
fun NewDueChip(
    due: NewDue?,
    today: LocalDate,
    passed: Boolean,
    onDay: (LocalDate) -> Unit,
    onPickDate: () -> Unit,
    onPickTime: () -> Unit,
    onClear: () -> Unit,
) {
    var open by remember { mutableStateOf(false) }
    val colors = MaterialTheme.colorScheme
    Box {
        if (due == null) {
            InputChip(
                selected = false,
                onClick = { open = true },
                label = { Text("Date", fontSize = 15.sp, fontWeight = FontWeight.SemiBold) },
                leadingIcon = { Icon(Icons.Filled.DateRange, contentDescription = null, modifier = Modifier.size(InputChipDefaults.IconSize)) },
                shape = RoundedCornerShape(50),
                colors = InputChipDefaults.inputChipColors(containerColor = colors.surface, labelColor = colors.onSurface, leadingIconColor = colors.onSurfaceVariant),
                modifier = Modifier.heightIn(min = 40.dp).semantics { contentDescription = "Due date: none. Double tap to pick" },
            )
        } else {
            val text = newDueText(due, today)
            val fill = if (passed) colors.error else colors.primary
            val ink = if (passed) colors.onError else colors.onPrimary
            InputChip(
                selected = true,
                onClick = { open = true },
                label = { Text(text, fontSize = 15.sp, fontWeight = FontWeight.SemiBold) },
                trailingIcon = {
                    // Its own tap target: the × removes the date, the rest of the chip opens the menu.
                    Box(Modifier.size(28.dp).clip(CircleShape).clickable(onClick = onClear), contentAlignment = Alignment.Center) {
                        Icon(Icons.Filled.Close, contentDescription = null, modifier = Modifier.size(InputChipDefaults.IconSize))
                    }
                },
                colors = InputChipDefaults.inputChipColors(selectedContainerColor = fill, selectedLabelColor = ink, selectedTrailingIconColor = ink),
                shape = RoundedCornerShape(50),
                modifier = Modifier.heightIn(min = 40.dp).semantics {
                    contentDescription = "Due date: ${text.lowercase()}. Double tap to change"
                    customActions = listOf(CustomAccessibilityAction("Remove the due date from the new item") { onClear(); true })
                },
            )
        }
        // Not focusable, so the keyboard (and with it this row) stays open.
        DropdownMenu(expanded = open, onDismissRequest = { open = false }, properties = PopupProperties(focusable = false)) {
            val pick = { action: () -> Unit -> { open = false; action() } }
            DropdownMenuItem(text = { Text("Today") }, trailingIcon = { Text(dayLine(today), color = colors.onSurfaceVariant) }, onClick = pick { onDay(today) })
            DropdownMenuItem(text = { Text("Tomorrow") }, trailingIcon = { Text(dayLine(today.plusDays(1)), color = colors.onSurfaceVariant) }, onClick = pick { onDay(today.plusDays(1)) })
            DropdownMenuItem(text = { Text("Pick a date…") }, onClick = pick(onPickDate))
            if (due != null) {
                HorizontalDivider()
                DropdownMenuItem(text = { Text("Set a time…") }, onClick = pick(onPickTime))
            }
        }
    }
}

private val dayLineFormat = java.time.format.DateTimeFormatter.ofPattern("EEE d MMM", java.util.Locale.ENGLISH)

private fun dayLine(d: LocalDate): String = d.format(dayLineFormat)
