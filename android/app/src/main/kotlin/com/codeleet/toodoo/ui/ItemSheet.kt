package com.codeleet.toodoo.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.DateRange
import androidx.compose.material3.BasicAlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TimeInput
import androidx.compose.material3.TimePicker
import androidx.compose.material3.TimePickerState
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.material.icons.filled.Check
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.layout
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.codeleet.toodoo.api.Item
import com.codeleet.toodoo.api.Label
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.Locale
import kotlinx.coroutines.launch

/** The amounts an item can be postponed by, as on the web (todo.PostponeMinutes). */
val POSTPONE_MINUTES = listOf(5, 10, 15, 30)

private val dateButton = DateTimeFormatter.ofPattern("EEE d MMM", Locale.ENGLISH)
private val dateButtonYear = DateTimeFormatter.ofPattern("d MMM yyyy", Locale.ENGLISH)

/** The date button's text: "Date" with no date; the weekday only in this year, so the text stays short. */
fun dateButtonText(date: LocalDate?, today: LocalDate): String =
    date?.format(if (date.year == today.year) dateButton else dateButtonYear) ?: "Date"

/** The time button's text: the time, "All day" for a date with no time, or "Time" with no date. */
fun timeButtonText(time: String, hasDate: Boolean): String = when {
    !hasDate -> "Time"
    time.isEmpty() -> "All day"
    else -> time
}

/** The hour and minute the time picker opens at: the item's time ("HH:MM"), or the next full hour. */
fun pickerStart(time: String, now: LocalTime): Pair<Int, Int> {
    val parts = time.split(":")
    val hour = parts.getOrNull(0)?.toIntOrNull()
    val minute = parts.getOrNull(1)?.toIntOrNull()
    return if (hour != null && minute != null) Pair(hour, minute) else Pair((now.hour + 1) % 24, 0)
}

/** "HH:MM", as the API takes it. */
fun clockText(hour: Int, minute: Int): String = "%02d:%02d".format(hour, minute)

/** The due date and time of [item] in [zone], as the sheet's fields show them: the time is "" for all day. */
fun dueFields(item: Item, zone: ZoneId): Pair<LocalDate?, String> {
    val at = item.dueAt?.let { runCatching { Instant.parse(it) }.getOrNull() } ?: return Pair(null, "")
    val local = at.atZone(zone)
    return Pair(local.toLocalDate(), if (item.dueAllDay) "" else "%02d:%02d".format(local.hour, local.minute))
}

/**
 * The item sheet (mockup "item sheet"): text, due date and time, postpone,
 * labels, mark as done and delete. Text and due date are sent with Save;
 * postpone and labels at once. [labels] (all labels) is null while it loads.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ItemSheet(
    item: Item,
    onSave: suspend (text: String?, due: Pair<String, String>?) -> Boolean,
    onPostpone: (Int) -> Unit,
    labels: List<Label>?,
    onToggleLabel: (Label) -> Unit,
    onAllLabels: () -> Unit,
    onToggleDone: () -> Unit,
    onDelete: () -> Unit,
    onDismiss: () -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet, containerColor = MaterialTheme.colorScheme.surface) {
        ItemSheetBody(item, onSave, onPostpone, labels, onToggleLabel, onAllLabels, onToggleDone, onDelete, onDismiss)
    }
}

/** The inside of the item sheet. */
@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun ItemSheetBody(
    item: Item,
    onSave: suspend (text: String?, due: Pair<String, String>?) -> Boolean,
    onPostpone: (Int) -> Unit,
    labels: List<Label>?,
    onToggleLabel: (Label) -> Unit,
    onAllLabels: () -> Unit,
    onToggleDone: () -> Unit,
    onDelete: () -> Unit,
    onDismiss: () -> Unit,
) {
    val zone = phoneZone()
    val (startDate, startTime) = remember(item.id, item.dueAt, item.dueAllDay) { dueFields(item, zone) }
    var text by rememberSaveable(item.id) { mutableStateOf(item.text) }
    var date by remember(item.id, startDate) { mutableStateOf(startDate) }
    var time by rememberSaveable(item.id, startTime) { mutableStateOf(startTime) }
    var picking by remember { mutableStateOf(false) }
    var pickingTime by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val colors = MaterialTheme.colorScheme
    val label = @Composable { s: String -> Text(s, fontSize = 14.sp, fontWeight = FontWeight.SemiBold, color = colors.onSurfaceVariant) }

    val save = {
        val newText = text.takeIf { it != item.text }
        val newDue = if (date != startDate || time.trim() != startTime) Pair(date?.toString() ?: "", if (date == null) "" else time.trim()) else null
        if (newText == null && newDue == null) {
            onDismiss()
        } else if (!busy) {
            busy = true
            scope.launch {
                if (onSave(newText, newDue)) onDismiss()
                busy = false
            }
        }
    }

    Column(
        Modifier.fillMaxWidth().imePadding().verticalScroll(rememberScrollState()).padding(start = 20.dp, end = 20.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(18.dp),
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            label("Text")
            OutlinedTextField(
                value = text,
                onValueChange = { text = it },
                minLines = 2,
                textStyle = MaterialTheme.typography.bodyLarge.copy(fontSize = 18.sp),
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences),
                modifier = Modifier.fillMaxWidth().semantics { contentDescription = "Text" },
            )
        }

        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            label("Due")
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                // 12 dp side padding (not the default 24), so a date fits on a 360 dp phone.
                OutlinedButton(
                    onClick = { picking = true },
                    shape = RoundedCornerShape(8.dp),
                    contentPadding = PaddingValues(horizontal = 12.dp),
                    modifier = Modifier.weight(1f).height(52.dp),
                ) {
                    Icon(Icons.Filled.DateRange, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(8.dp))
                    Text(dateButtonText(date, LocalDate.now(zone)), maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 16.sp, color = colors.onSurface, modifier = Modifier.weight(1f))
                }
                val timeText = timeButtonText(time, hasDate = date != null)
                OutlinedButton(
                    onClick = { pickingTime = true },
                    enabled = date != null,
                    shape = RoundedCornerShape(8.dp),
                    contentPadding = PaddingValues(horizontal = 12.dp),
                    modifier = Modifier.width(112.dp).height(52.dp).semantics { contentDescription = "Time: $timeText" },
                ) {
                    ClockIcon(LocalContentColor.current)
                    Spacer(Modifier.width(8.dp))
                    Text(timeText, maxLines = 1, fontSize = 16.sp, color = if (date != null) colors.onSurface else LocalContentColor.current)
                }
                IconButton(onClick = {
                    date = null
                    time = ""
                }, enabled = date != null) {
                    Icon(Icons.Filled.Close, contentDescription = "Clear the due date", modifier = Modifier.size(18.dp))
                }
            }
            Text("Tap the time to pick one. No time means all day.", fontSize = 13.sp, color = colors.onSurfaceVariant)
        }

        if (item.dueAt != null && !item.done) {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                label("Postpone")
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    POSTPONE_MINUTES.forEach { m ->
                        OutlinedButton(onClick = { onPostpone(m) }, contentPadding = PaddingValues(horizontal = 4.dp), modifier = Modifier.weight(1f)) { Text("$m min", fontSize = 15.sp, maxLines = 1) }
                    }
                }
            }
        }

        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            label("Labels")
            // The item's labels when the sheet opened come first, and the order
            // stays while it is open. Before all labels load, the item's own show.
            val first = remember(item.id) { item.labels.map { it.id } }
            LabelChipRow(
                labels = labelsForItem(labels ?: item.labels, first),
                picked = item.labels.map { it.id },
                description = "Labels of the item",
                modifier = Modifier.padding(top = 4.dp).bleed(20.dp),
                sidePadding = 20.dp,
                onToggle = onToggleLabel,
            )
            TextButton(onClick = onAllLabels, contentPadding = PaddingValues(horizontal = 8.dp), modifier = Modifier.offset(x = (-8).dp)) {
                TagIcon(colors.primary, Modifier.size(16.dp))
                Spacer(Modifier.width(6.dp))
                Text("All labels", fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
            }
        }

        // The same as a swipe right on the row, for those who do not swipe.
        OutlinedButton(onClick = onToggleDone, border = BorderStroke(1.dp, colors.primary), modifier = Modifier.fillMaxWidth().height(48.dp)) {
            if (item.done) UndoIcon(colors.primary, size = 18.dp) else Icon(Icons.Filled.Check, contentDescription = null, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
            Text(if (item.done) "Mark as not done" else "Mark as done", fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
        }

        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            TextButton(onClick = onDelete) { Text("Delete", fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = colors.error) }
            Spacer(Modifier.weight(1f))
            TextButton(onClick = onDismiss) { Text("Cancel", fontSize = 16.sp, fontWeight = FontWeight.SemiBold) }
            Button(onClick = { save() }, enabled = !busy) { Text("Save", fontSize = 16.sp, fontWeight = FontWeight.SemiBold) }
        }
    }

    if (picking) {
        val state = rememberDatePickerState(
            initialSelectedDateMillis = (date ?: LocalDate.now(zone)).atStartOfDay(ZoneOffset.UTC).toInstant().toEpochMilli(),
        )
        DatePickerDialog(
            onDismissRequest = { picking = false },
            confirmButton = {
                TextButton(onClick = {
                    picking = false
                    // The picker's days are UTC midnights.
                    state.selectedDateMillis?.let { date = Instant.ofEpochMilli(it).atZone(ZoneOffset.UTC).toLocalDate() }
                }) { Text("OK") }
            },
            dismissButton = { TextButton(onClick = { picking = false }) { Text("Cancel") } },
        ) { DatePicker(state) }
    }

    if (pickingTime) {
        TimePickerDialog(time, zone, onPick = {
            time = it
            pickingTime = false
        }, onDismiss = { pickingTime = false })
    }
}

/**
 * The time picker (mockup "Date button and time picker", page 17): the
 * 24-hour dial, as in Google Calendar. [onPick] gets "HH:MM", or "" for
 * "All day".
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TimePickerDialog(time: String, zone: ZoneId, onPick: (String) -> Unit, onDismiss: () -> Unit) {
    val (hour, minute) = remember(time) { pickerStart(time, LocalTime.now(zone)) }
    val state = rememberTimePickerState(initialHour = hour, initialMinute = minute, is24Hour = true)
    BasicAlertDialog(onDismissRequest = onDismiss) {
        TimePickerBody(state, onOk = { onPick(clockText(state.hour, state.minute)) }, onAllDay = { onPick("") }, onDismiss = onDismiss)
    }
}

/** The inside of the time picker. The keyboard icon switches to two number fields and back. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TimePickerBody(state: TimePickerState, onOk: () -> Unit, onAllDay: () -> Unit, onDismiss: () -> Unit, startTyping: Boolean = false) {
    var typing by rememberSaveable { mutableStateOf(startTyping) }
    val colors = MaterialTheme.colorScheme
    Surface(shape = RoundedCornerShape(28.dp), color = colors.surface, tonalElevation = 6.dp) {
        Column(Modifier.padding(24.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Text(if (typing) "Enter time" else "Select time", fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = colors.onSurfaceVariant)
            Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                if (typing) TimeInput(state) else TimePicker(state)
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                IconButton(
                    onClick = { typing = !typing },
                    modifier = Modifier.semantics { contentDescription = if (typing) "Use the clock dial" else "Type the time" },
                ) {
                    if (typing) ClockIcon(colors.onSurfaceVariant, size = 24.dp) else KeyboardIcon(colors.onSurfaceVariant)
                }
                Spacer(Modifier.weight(1f))
                TextButton(onClick = onAllDay) { Text("All day") }
                TextButton(onClick = onDismiss) { Text("Cancel") }
                TextButton(onClick = onOk) { Text("OK") }
            }
        }
    }
}

/** Lets a row reach [side] past the sheet's padding on both sides, so chips scroll to the edge. */
private fun Modifier.bleed(side: Dp) = layout { measurable, constraints ->
    val extra = (side * 2).roundToPx()
    val placeable = measurable.measure(constraints.copy(minWidth = constraints.maxWidth + extra, maxWidth = constraints.maxWidth + extra))
    layout(constraints.maxWidth, placeable.height) { placeable.place(-side.roundToPx(), 0) }
}
