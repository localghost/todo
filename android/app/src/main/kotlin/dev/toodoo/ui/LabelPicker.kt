package dev.toodoo.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.toodoo.api.Label

/** What a tap on a label in the picker does. */
sealed interface PickerMode {
    /** Puts the label on the item, or takes it off; new names can be made. */
    data class ForItem(val itemText: String, val selected: Set<Long>) : PickerMode

    /** Shows only the items with the label; new names can be made too, with no items. */
    data class Filter(val current: Label?) : PickerMode
}

/**
 * The searchable list of all labels (mockup "label picker"): a color dot that
 * opens the color wheel, the name with its item count, and a × that deletes
 * the label after asking. [labels] is null while it loads.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LabelPicker(
    labels: List<Label>?,
    mode: PickerMode,
    onPick: (Label) -> Unit,
    onCreate: (String) -> Unit,
    onClearFilter: () -> Unit,
    onColor: (Label, String) -> Unit,
    onDelete: (Label) -> Unit,
    onDismiss: () -> Unit,
) {
    var query by rememberSaveable { mutableStateOf("") }
    var coloring by remember { mutableStateOf<Label?>(null) }
    var deleting by remember { mutableStateOf<Label?>(null) }
    val q = query.trim()
    val canCreate = q.isNotEmpty() && labels != null && labels.none { it.name.equals(q, ignoreCase = true) }
    val create = {
        onCreate(q)
        query = ""
    }
    val colors = MaterialTheme.colorScheme

    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = colors.surface) {
        LabelPickerBody(labels, mode, query, { query = it }, onPick, create, canCreate, onClearFilter, { coloring = it }, { deleting = it }, onDismiss)
    }

    coloring?.let { label ->
        ColorWheelDialog(label, onPick = {
            coloring = null
            if (!it.equals(label.color, ignoreCase = true)) onColor(label, it)
        }, onDismiss = { coloring = null })
    }
    deleting?.let { label ->
        val n = label.items ?: 0
        AlertDialog(
            onDismissRequest = { deleting = null },
            title = { Text("Delete the label “${label.name}”?") },
            text = { Text(if (n == 0) "No item has it." else if (n == 1) "It is taken off 1 item. The item stays." else "It is taken off $n items. The items stay.") },
            confirmButton = {
                TextButton(onClick = {
                    deleting = null
                    onDelete(label)
                }) { Text("Delete", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton(onClick = { deleting = null }) { Text("Cancel") } },
        )
    }
}

/** The inside of the label picker. */
@Composable
fun LabelPickerBody(
    labels: List<Label>?,
    mode: PickerMode,
    query: String,
    onQuery: (String) -> Unit,
    onPick: (Label) -> Unit,
    create: () -> Unit,
    canCreate: Boolean,
    onClearFilter: () -> Unit,
    onColor: (Label) -> Unit,
    onDelete: (Label) -> Unit,
    onDismiss: () -> Unit,
) {
    val q = query.trim()
    val shown = labels.orEmpty().filter { it.name.contains(q, ignoreCase = true) }
    val colors = MaterialTheme.colorScheme
    Column(Modifier.fillMaxWidth().imePadding().padding(start = 12.dp, end = 12.dp, bottom = 16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Row(Modifier.padding(horizontal = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            Text("Labels", fontSize = 20.sp, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
            TextButton(onClick = onDismiss) { Text("Done", fontWeight = FontWeight.SemiBold) }
        }
        OutlinedTextField(
            value = query,
            onValueChange = onQuery,
            placeholder = { Text("Search or create…") },
            singleLine = true,
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
            keyboardActions = KeyboardActions(onDone = { if (canCreate) create() }),
            modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp),
        )
        if (mode is PickerMode.Filter) {
            Text("Tap a label to show only its items.", fontSize = 14.sp, color = colors.onSurfaceVariant, modifier = Modifier.padding(horizontal = 16.dp))
        }
        LazyColumn(Modifier.fillMaxWidth().heightIn(max = 420.dp)) {
            if (mode is PickerMode.Filter && mode.current != null) {
                item(key = "all") {
                    Text(
                        "All items",
                        fontSize = 17.sp,
                        color = colors.primary,
                        fontWeight = FontWeight.SemiBold,
                        modifier = Modifier.fillMaxWidth().clickable(onClick = onClearFilter).padding(horizontal = 16.dp, vertical = 16.dp),
                    )
                }
            }
            if (labels != null && labels.isEmpty()) {
                item(key = "none") {
                    Text(
                        "No labels yet. Type a name to make one.",
                        color = colors.onSurfaceVariant,
                        modifier = Modifier.padding(16.dp),
                    )
                }
            }
            items(shown, key = { it.id }) { label ->
                val on = when (mode) {
                    is PickerMode.ForItem -> label.id in mode.selected
                    is PickerMode.Filter -> label.id == mode.current?.id
                }
                LabelOption(label, on, onPick = { onPick(label) }, onColor = { onColor(label) }, onDelete = { onDelete(label) })
            }
            if (canCreate) {
                item(key = "create") {
                    HorizontalDivider(Modifier.padding(top = 4.dp), color = colors.outlineVariant)
                    Row(
                        Modifier.fillMaxWidth().heightIn(min = 56.dp).clickable(onClick = create).padding(horizontal = 16.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        Icon(Icons.Filled.Add, contentDescription = null, tint = colors.primary, modifier = Modifier.size(18.dp))
                        Text("Create “$q”", fontSize = 17.sp, color = colors.primary, fontWeight = FontWeight.SemiBold)
                    }
                }
            }
        }
    }
}

@Composable
private fun LabelOption(label: Label, on: Boolean, onPick: () -> Unit, onColor: () -> Unit, onDelete: () -> Unit) {
    val colors = MaterialTheme.colorScheme
    Row(
        Modifier.fillMaxWidth().heightIn(min = 56.dp)
            .background(if (on) colors.primaryContainer else Color.Transparent, RoundedCornerShape(8.dp)),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        IconButton(onClick = onColor, modifier = Modifier.semantics { contentDescription = "Color of “${label.name}”" }) {
            Box(Modifier.size(18.dp).background(labelDot(label.color), CircleShape))
        }
        Row(
            Modifier.weight(1f).heightIn(min = 56.dp)
                .semantics { selected = on }
                .clickable(role = Role.Checkbox, onClick = onPick)
                .padding(horizontal = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(label.name, fontSize = 17.sp, color = colors.onSurface, modifier = Modifier.weight(1f))
            label.items?.let { Text(if (it == 1) "1 item" else "$it items", fontSize = 14.sp, color = colors.onSurfaceVariant) }
            Box(Modifier.width(28.dp), contentAlignment = Alignment.Center) {
                if (on) Icon(Icons.Filled.Check, contentDescription = null, tint = colors.primary, modifier = Modifier.size(20.dp))
            }
        }
        IconButton(onClick = onDelete) {
            Icon(Icons.Filled.Close, contentDescription = "Delete the label “${label.name}”", tint = colors.onSurfaceVariant, modifier = Modifier.size(18.dp))
        }
    }
    Spacer(Modifier.size(2.dp))
}

/** The color dot of a label; gray for a color the app cannot read. */
private fun labelDot(hex: String): Color = runCatching { hexColor(hex) }.getOrDefault(Color.Gray)
