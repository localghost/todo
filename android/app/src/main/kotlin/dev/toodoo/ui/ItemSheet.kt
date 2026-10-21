package dev.toodoo.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
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
import androidx.compose.material3.Button
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.toodoo.api.Item
import dev.toodoo.api.Label
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.Locale
import kotlinx.coroutines.launch

/** The amounts an item can be postponed by, as on the web (todo.PostponeMinutes). */
val POSTPONE_MINUTES = listOf(5, 10, 15, 30)

private val dateButton = DateTimeFormatter.ofPattern("EEE d MMM", Locale.ENGLISH)
private val dateButtonYear = DateTimeFormatter.ofPattern("EEE d MMM yyyy", Locale.ENGLISH)

/** The due date and time of [item] in [zone], as the sheet's fields show them: the time is "" for all day. */
fun dueFields(item: Item, zone: ZoneId): Pair<LocalDate?, String> {
    val at = item.dueAt?.let { runCatching { Instant.parse(it) }.getOrNull() } ?: return Pair(null, "")
    val local = at.atZone(zone)
    return Pair(local.toLocalDate(), if (item.dueAllDay) "" else "%02d:%02d".format(local.hour, local.minute))
}

/**
 * The item sheet (mockup "item sheet"): text, due date and time, postpone,
 * labels and delete. Text and due date are sent with Save; postpone and
 * labels at once.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ItemSheet(
    item: Item,
    onSave: suspend (text: String?, due: Pair<String, String>?) -> Boolean,
    onPostpone: (Int) -> Unit,
    onRemoveLabel: (Label) -> Unit,
    onAddLabel: () -> Unit,
    onDelete: () -> Unit,
    onDismiss: () -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet, containerColor = MaterialTheme.colorScheme.surface) {
        ItemSheetBody(item, onSave, onPostpone, onRemoveLabel, onAddLabel, onDelete, onDismiss)
    }
}

/** The inside of the item sheet. */
@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun ItemSheetBody(
    item: Item,
    onSave: suspend (text: String?, due: Pair<String, String>?) -> Boolean,
    onPostpone: (Int) -> Unit,
    onRemoveLabel: (Label) -> Unit,
    onAddLabel: () -> Unit,
    onDelete: () -> Unit,
    onDismiss: () -> Unit,
) {
    val zone = phoneZone()
    val (startDate, startTime) = remember(item.id, item.dueAt, item.dueAllDay) { dueFields(item, zone) }
    var text by rememberSaveable(item.id) { mutableStateOf(item.text) }
    var date by remember(item.id, startDate) { mutableStateOf(startDate) }
    var time by rememberSaveable(item.id, startTime) { mutableStateOf(startTime) }
    var picking by remember { mutableStateOf(false) }
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
                OutlinedButton(
                    onClick = { picking = true },
                    shape = RoundedCornerShape(8.dp),
                    modifier = Modifier.weight(1f).height(52.dp),
                ) {
                    Icon(Icons.Filled.DateRange, contentDescription = null, modifier = Modifier.size(18.dp))
                    Spacer(Modifier.width(8.dp))
                    Text(date?.let { it.format(if (it.year == LocalDate.now(zone).year) dateButton else dateButtonYear) } ?: "Pick a date", maxLines = 1, fontSize = 16.sp, color = colors.onSurface, modifier = Modifier.weight(1f))
                }
                OutlinedTextField(
                    value = time,
                    onValueChange = { time = it.take(5) },
                    placeholder = { Text("All day", fontSize = 15.sp) },
                    singleLine = true,
                    enabled = date != null,
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    modifier = Modifier.width(108.dp).semantics { contentDescription = "Time" },
                )
                IconButton(onClick = {
                    date = null
                    time = ""
                }, enabled = date != null) {
                    Icon(Icons.Filled.Close, contentDescription = "Clear the due date", modifier = Modifier.size(18.dp))
                }
            }
            Text("Leave the time empty for all day. Type 1430 for 14:30.", fontSize = 13.sp, color = colors.onSurfaceVariant)
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

        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            label("Labels")
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                item.labels.forEach { l -> RemovableChip(l) { onRemoveLabel(l) } }
                Surface(
                    onClick = onAddLabel,
                    shape = RoundedCornerShape(18.dp),
                    border = BorderStroke(1.dp, colors.outline),
                    color = colors.surface,
                    modifier = Modifier.height(36.dp),
                ) {
                    Row(Modifier.padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        Icon(Icons.Filled.Add, contentDescription = null, tint = colors.primary, modifier = Modifier.size(16.dp))
                        Text("Add label", fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = colors.primary)
                    }
                }
            }
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
}

/** A label chip with a × that takes the label off the item. */
@Composable
private fun RemovableChip(label: Label, onRemove: () -> Unit) {
    val shades = labelShades(label.color, androidx.compose.foundation.isSystemInDarkTheme())
    val colors = MaterialTheme.colorScheme
    val fill = shades?.let { hexColor("#%06x".format(it.fill)) } ?: colors.surfaceVariant
    val border = shades?.let { hexColor("#%06x".format(it.border)) } ?: colors.outlineVariant
    val ink = shades?.let { hexColor("#%06x".format(it.text)) } ?: colors.onSurfaceVariant
    Surface(shape = RoundedCornerShape(18.dp), color = fill, border = BorderStroke(1.dp, border), modifier = Modifier.height(36.dp)) {
        Row(Modifier.padding(start = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Text(label.name, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = ink)
            IconButton(onClick = onRemove, modifier = Modifier.size(34.dp)) {
                Icon(Icons.Filled.Close, contentDescription = "Remove ${label.name}", tint = ink, modifier = Modifier.size(14.dp))
            }
        }
    }
}
