package dev.toodoo.ui

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.provider.Settings
import android.view.HapticFeedbackConstants
import android.view.View
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGesturesAfterLongPress
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.filled.Close
import androidx.compose.material3.InputChip
import androidx.compose.material3.InputChipDefaults
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.zIndex
import androidx.core.content.ContextCompat
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.isImeVisible
import androidx.compose.foundation.layout.navigationBarsPadding
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
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarDuration
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.SnackbarResult
import androidx.compose.material3.Surface
import androidx.compose.material3.SwipeToDismissBox
import androidx.compose.material3.SwipeToDismissBoxValue
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.rememberSwipeToDismissBoxState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.toodoo.api.Item
import dev.toodoo.api.Label
import java.time.ZonedDateTime
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ListScreen(vm: ListActions) {
    val state by vm.state.collectAsStateWithLifecycle()
    val snackbar = remember { SnackbarHostState() }
    var menuOpen by remember { mutableStateOf(false) }
    var confirmClear by remember { mutableStateOf(false) }
    var picker by remember { mutableStateOf<PickerFor?>(null) }
    val list = state.list
    val context = LocalContext.current
    val listState = rememberLazyListState()
    val reorder = remember(listState) { Reorder(listState) }
    val askNotify = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {}
    val openWeb = { path: String -> context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(vm.server.trimEnd('/') + path))) }

    LaunchedEffect(state.message) {
        state.message?.let {
            vm.messageShown()
            snackbar.showSnackbar(it)
        }
    }
    state.pending?.let { p ->
        LaunchedEffect(p) {
            val what = when {
                p.delete -> "deleted"
                p.item.done -> "not done"
                else -> "done"
            }
            val result = snackbar.showSnackbar("“${p.item.text}” $what", actionLabel = "Undo", duration = SnackbarDuration.Short)
            if (result == SnackbarResult.ActionPerformed) vm.undo() else vm.commit(p)
        }
    }
    val scope = rememberCoroutineScope()
    val view = LocalView.current
    val overdueHeld = {
        haptic(view, rejected = true)
        scope.launch { snackbar.showSnackbar("Overdue items stay at the top.") }
        Unit
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text("TooDoo", fontWeight = FontWeight.Bold)
                        if (list != null) {
                            Text("${list.open} open · ${list.done} done", fontSize = 14.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                },
                actions = {
                    IconButton(onClick = {
                        vm.loadLabels()
                        picker = PickerFor.Filter
                    }) {
                        Box(Modifier.semantics { contentDescription = "Filter by label" }) { TagIcon(MaterialTheme.colorScheme.onSurface) }
                    }
                    Box {
                        IconButton(onClick = { menuOpen = true }) { Icon(Icons.Filled.MoreVert, contentDescription = "More") }
                        DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                            val hide = list?.hideDone == true
                            DropdownMenuItem(text = { Text(if (hide) "Show done items" else "Hide done items") }, onClick = {
                                menuOpen = false
                                vm.setHideDone(!hide)
                            })
                            DropdownMenuItem(text = { Text("Clear done items") }, enabled = (list?.done ?: 0) > 0, onClick = {
                                menuOpen = false
                                confirmClear = true
                            })
                            DropdownMenuItem(text = { Text("Labels") }, onClick = {
                                menuOpen = false
                                vm.loadLabels()
                                picker = PickerFor.Filter
                            })
                            if (!vm.exactReminders()) {
                                DropdownMenuItem(text = { Text("Allow exact reminders") }, onClick = {
                                    menuOpen = false
                                    // Without it, Android may deliver reminders a few minutes late.
                                    context.startActivity(Intent(Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM, Uri.parse("package:" + context.packageName)))
                                })
                            }
                            DropdownMenuItem(text = { Text("Account (web)") }, onClick = {
                                menuOpen = false
                                openWeb("/account")
                            })
                            HorizontalDivider()
                            DropdownMenuItem(text = { Text("Log out ${vm.username}") }, onClick = {
                                menuOpen = false
                                vm.logOut()
                            })
                        }
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
            )
        },
        bottomBar = { AddBar(labels = state.labels, filter = state.filter, onOpen = vm::loadLabels, onAdd = vm::add) },
        snackbarHost = { SnackbarHost(snackbar) },
        containerColor = MaterialTheme.colorScheme.background,
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = state.refreshing,
            onRefresh = vm::refresh,
            modifier = Modifier.fillMaxSize().padding(padding),
        ) {
            LazyColumn(
                state = listState,
                contentPadding = PaddingValues(start = 12.dp, end = 12.dp, bottom = 16.dp),
                modifier = Modifier.fillMaxSize(),
            ) {
                state.filter?.let { f ->
                    item(key = "filter") {
                        InputChip(
                            selected = true,
                            onClick = { vm.setFilter(null) },
                            label = { Text("Label: ${f.name}") },
                            trailingIcon = { Icon(Icons.Filled.Close, contentDescription = "Show all items", modifier = Modifier.size(InputChipDefaults.IconSize)) },
                            colors = InputChipDefaults.inputChipColors(
                                selectedContainerColor = MaterialTheme.colorScheme.primaryContainer,
                                selectedLabelColor = MaterialTheme.colorScheme.onPrimaryContainer,
                                selectedTrailingIconColor = MaterialTheme.colorScheme.onPrimaryContainer,
                            ),
                            modifier = Modifier.padding(start = 4.dp, bottom = 8.dp),
                        )
                    }
                }
                if (state.gestureHint) {
                    item(key = "hint") { GestureHint(onClose = vm::gestureHintSeen) }
                }
                if (state.offline) {
                    item(key = "offline") {
                        Text(
                            "No connection. This is the list from your last visit.",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 8.dp),
                        )
                    }
                }
                when {
                    list == null -> {}
                    list.items.isEmpty() -> item(key = "empty") {
                        Text(
                            if (list.open + list.done == 0) "Nothing to do. Add your first item below." else "No items to show.",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(horizontal = 8.dp, vertical = 24.dp),
                        )
                    }
                    else -> {
                        val now = ZonedDateTime.now(phoneZone())
                        val byId = list.items.associateBy { it.id }
                        val shown = if (reorder.dragging != null) reorder.order.mapNotNull { byId[it] } else list.items
                        reorder.movable = { byId[it]?.overdue == false }
                        reorder.current = list.items.map { it.id }
                        items(shown, key = { it.id }) { item ->
                            val first = item.id == shown.first().id
                            val last = item.id == shown.last().id
                            val dragged = reorder.dragging == item.id
                            ItemRow(
                                item, now, first, last, dragged,
                                onToggle = { vm.toggle(item) },
                                onDelete = { vm.delete(item) },
                                onOpen = { vm.openSheet(item) },
                                onMoveBy = { step -> moveBy(shown, item, step)?.let { vm.move(item.id, it) } },
                                // Hold a row to lift it. Overdue rows stay at the top, as on the web.
                                hold = Modifier.pointerInput(item.id, item.overdue) {
                                    if (item.overdue) {
                                        detectDragGesturesAfterLongPress(onDragStart = { overdueHeld() }, onDrag = { change, _ -> change.consume() })
                                    } else {
                                        detectDragGesturesAfterLongPress(
                                            onDragStart = {
                                                haptic(view)
                                                reorder.start(item.id)
                                            },
                                            onDrag = { change, amount ->
                                                change.consume()
                                                reorder.drag(amount.y)
                                            },
                                            onDragEnd = { reorder.end()?.let { (id, order) -> vm.move(id, order) } },
                                            onDragCancel = { reorder.end()?.let { (id, order) -> vm.move(id, order) } },
                                        )
                                    }
                                },
                                modifier = if (dragged) {
                                    Modifier.zIndex(1f).graphicsLayer {
                                        translationY = reorder.offset
                                        scaleX = 1.03f
                                        shadowElevation = 10.dp.toPx()
                                    }
                                } else {
                                    Modifier.animateItem()
                                },
                            )
                        }
                    }
                }
            }
        }
    }

    if (confirmClear && list != null) {
        AlertDialog(
            onDismissRequest = { confirmClear = false },
            title = { Text("Clear done items?") },
            text = { Text(if (list.done == 1) "1 done item will be deleted." else "${list.done} done items will be deleted.") },
            confirmButton = {
                TextButton(onClick = {
                    confirmClear = false
                    vm.clearDone()
                }) { Text("Delete", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton(onClick = { confirmClear = false }) { Text("Cancel") } },
        )
    }

    state.sheet?.let { item ->
        ItemSheet(
            item = item,
            onSave = { text, due ->
                val ok = vm.save(item, text, due)
                // Android 13+ asks for notifications with the first due date, not at start-up.
                if (ok && due?.first?.isNotEmpty() == true && Build.VERSION.SDK_INT >= 33 &&
                    ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
                ) {
                    askNotify.launch(Manifest.permission.POST_NOTIFICATIONS)
                }
                ok
            },
            onPostpone = { vm.postpone(item, it) },
            onRemoveLabel = { vm.removeLabel(item, it) },
            onAddLabel = {
                vm.loadLabels()
                picker = PickerFor.Item
            },
            onToggleDone = {
                vm.closeSheet()
                vm.toggle(item)
            },
            onDelete = {
                vm.closeSheet()
                vm.delete(item)
            },
            onDismiss = vm::closeSheet,
        )
    }

    picker?.let { p ->
        val sheetItem = state.sheet
        val mode = when {
            p == PickerFor.Item && sheetItem != null -> PickerMode.ForItem(sheetItem.text, sheetItem.labels.map { it.id }.toSet())
            else -> PickerMode.Filter(state.filter)
        }
        LabelPicker(
            labels = state.labels,
            mode = mode,
            onPick = { label ->
                if (mode is PickerMode.ForItem && sheetItem != null) {
                    if (label.id in mode.selected) vm.removeLabel(sheetItem, label) else vm.addLabel(sheetItem, label.name)
                } else {
                    picker = null
                    vm.setFilter(label)
                }
            },
            onCreate = { name -> sheetItem?.let { vm.addLabel(it, name) } },
            onClearFilter = {
                picker = null
                vm.setFilter(null)
            },
            onColor = vm::setLabelColor,
            onDelete = vm::deleteLabel,
            onDismiss = { picker = null },
        )
    }
}

/** Which label picker is open. */
private enum class PickerFor { Item, Filter }

/**
 * The new order of [shown] after moving [item] one place up (-1) or down
 * (+1), or null if it cannot move there: overdue items stay at the top.
 */
internal fun moveBy(shown: List<Item>, item: Item, step: Int): List<Long>? {
    val ids = shown.map { it.id }.toMutableList()
    val from = ids.indexOf(item.id)
    val to = from + step
    if (item.overdue || from < 0 || to !in ids.indices || shown[to].overdue) return null
    ids.removeAt(from)
    ids.add(to, item.id)
    return ids
}

/**
 * Drag to reorder: the dragged row follows the finger, and the rows swap
 * places when its middle crosses another row. Rows that are not [movable]
 * (overdue) do not swap.
 */
private class Reorder(private val listState: LazyListState) {
    var dragging by mutableStateOf<Long?>(null)
        private set
    var offset by mutableFloatStateOf(0f)
        private set
    val order = mutableStateListOf<Long>()
    var movable: (Long) -> Boolean = { true }

    /** The list's ids as the server sent them; the screen keeps it up to date. */
    var current: List<Long> = emptyList()
    private var before: List<Long> = emptyList()

    fun start(id: Long) {
        before = current
        order.clear()
        order.addAll(current)
        offset = 0f
        dragging = id
    }

    fun drag(dy: Float) {
        val id = dragging ?: return
        offset += dy
        val rows = listState.layoutInfo.visibleItemsInfo
        val me = rows.find { it.key == id } ?: return
        val middle = me.offset + offset + me.size / 2f
        val target = rows.find {
            val key = it.key
            key is Long && key != id && movable(key) && middle >= it.offset && middle <= it.offset + it.size
        } ?: return
        val from = order.indexOf(id)
        val to = order.indexOf(target.key as Long)
        if (from < 0 || to < 0) return
        order.removeAt(from)
        order.add(to, id)
        // The row now sits where the target was; keep it under the finger.
        val newTop = if (to > from) target.offset + target.size - me.size else target.offset
        offset -= newTop - me.offset
    }

    /** Ends the drag; returns the item and the new order, or null if nothing moved. */
    fun end(): Pair<Long, List<Long>>? {
        val id = dragging ?: return null
        val moved = order.toList()
        dragging = null
        offset = 0f
        return if (moved == before) null else Pair(id, moved)
    }
}

/**
 * One row. Swipe right to mark it done (or not done), left to delete it;
 * [hold] lifts it for dragging. TalkBack gets the same as actions.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ItemRow(
    item: Item,
    now: ZonedDateTime,
    first: Boolean,
    last: Boolean,
    dragged: Boolean,
    onToggle: () -> Unit,
    onDelete: () -> Unit,
    onOpen: () -> Unit,
    onMoveBy: (Int) -> Unit,
    hold: Modifier,
    modifier: Modifier = Modifier,
) {
    val view = LocalView.current
    val dismiss = rememberSwipeToDismissBoxState(
        confirmValueChange = {
            when (it) {
                SwipeToDismissBoxValue.EndToStart -> onDelete()
                SwipeToDismissBoxValue.StartToEnd -> onToggle()
                SwipeToDismissBoxValue.Settled -> {}
            }
            // A row marked done springs back into place; a deleted one leaves.
            it != SwipeToDismissBoxValue.StartToEnd
        },
        positionalThreshold = { it * 0.4f },
    )
    // Past the line, letting go changes the item. One tick each time the finger crosses it.
    val armed = dismiss.targetValue != SwipeToDismissBoxValue.Settled
    var wasArmed by remember { mutableStateOf(false) }
    LaunchedEffect(armed) {
        if (armed != wasArmed) haptic(view, light = true)
        wasArmed = armed
    }
    val shape = RoundedCornerShape(
        topStart = if (first) 12.dp else 0.dp, topEnd = if (first) 12.dp else 0.dp,
        bottomStart = if (last) 12.dp else 0.dp, bottomEnd = if (last) 12.dp else 0.dp,
    )
    val colors = MaterialTheme.colorScheme
    SwipeToDismissBox(
        state = dismiss,
        backgroundContent = { SwipeBackground(item, dismiss.dismissDirection, armed, shape) },
        modifier = modifier,
    ) {
        Surface(
            color = colors.surface,
            shape = shape,
            border = if (dragged) BorderStroke(2.dp, colors.primary) else BorderStroke(1.dp, colors.outlineVariant),
        ) {
            Column(
                Modifier.fillMaxWidth().heightIn(min = 64.dp)
                    .clickable(onClickLabel = "Edit", onClick = onOpen)
                    // After clickable, so it sees the touch first: a hold that lifts the row is not also a tap.
                    .then(hold)
                    .semantics {
                        stateDescription = if (item.done) "Done" else "Not done"
                        customActions = buildList {
                            add(CustomAccessibilityAction(if (item.done) "Mark as not done" else "Mark as done") { onToggle(); true })
                            if (!item.overdue) {
                                add(CustomAccessibilityAction("Move up") { onMoveBy(-1); true })
                                add(CustomAccessibilityAction("Move down") { onMoveBy(1); true })
                            }
                            add(CustomAccessibilityAction("Delete") { onDelete(); true })
                        }
                    }
                    .padding(horizontal = 16.dp, vertical = 12.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Text(
                    item.text,
                    fontSize = 17.sp,
                    color = if (item.done) colors.onSurfaceVariant else colors.onSurface,
                    textDecoration = if (item.done) TextDecoration.LineThrough else null,
                )
                MetaLine(item, now)
            }
        }
    }
}

/** What shows under a row while it is swiped: blue "Done", gray "Not done", or red "Delete". */
@Composable
private fun SwipeBackground(item: Item, direction: SwipeToDismissBoxValue, armed: Boolean, shape: RoundedCornerShape) {
    val colors = MaterialTheme.colorScheme
    when (direction) {
        SwipeToDismissBoxValue.Settled -> {}
        SwipeToDismissBoxValue.EndToStart -> Row(
            Modifier.fillMaxSize().background(colors.error, shape).padding(end = 20.dp),
            horizontalArrangement = Arrangement.End,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(Icons.Filled.Delete, contentDescription = null, tint = colors.onError)
            Spacer(Modifier.width(8.dp))
            Text("Delete", color = colors.onError, fontWeight = FontWeight.SemiBold)
        }
        SwipeToDismissBoxValue.StartToEnd -> {
            // Before the line a light shade, past it the full color.
            val (fill, ink) = when {
                item.done && armed -> colors.onSurfaceVariant to colors.surface
                item.done -> colors.surfaceVariant to colors.onSurfaceVariant
                armed -> colors.primary to colors.onPrimary
                else -> colors.primaryContainer to colors.onPrimaryContainer
            }
            Row(
                Modifier.fillMaxSize().background(fill, shape).padding(start = 20.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                val size = if (armed) 28.dp else 20.dp
                if (item.done) UndoIcon(ink, size = size) else Icon(Icons.Filled.Check, contentDescription = null, tint = ink, modifier = Modifier.size(size))
                Spacer(Modifier.width(8.dp))
                Text(if (item.done) "Not done" else "Done", color = ink, fontWeight = FontWeight.Bold)
            }
        }
    }
}

/** Shown above the list until the user closes it. */
@Composable
private fun GestureHint(onClose: () -> Unit) {
    val colors = MaterialTheme.colorScheme
    Surface(
        color = colors.primaryContainer,
        contentColor = colors.onPrimaryContainer,
        shape = RoundedCornerShape(12.dp),
        modifier = Modifier.fillMaxWidth().padding(bottom = 12.dp),
    ) {
        Row(Modifier.padding(start = 14.dp, top = 12.dp, bottom = 12.dp, end = 4.dp), verticalAlignment = Alignment.Top) {
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Text("Swipe and hold", fontSize = 15.sp, fontWeight = FontWeight.Bold)
                Text("Swipe a row right to mark it done, left to delete it. Hold a row to move it.", fontSize = 15.sp)
            }
            TextButton(onClick = onClose) { Text("Got it", color = colors.onPrimaryContainer, fontWeight = FontWeight.Bold) }
        }
    }
}

/** A short buzz: [light] for the swipe line, [rejected] for a row that cannot move. */
private fun haptic(view: View, light: Boolean = false, rejected: Boolean = false) {
    val kind = when {
        light -> HapticFeedbackConstants.CLOCK_TICK
        rejected && Build.VERSION.SDK_INT >= 30 -> HapticFeedbackConstants.REJECT
        else -> HapticFeedbackConstants.LONG_PRESS
    }
    view.performHapticFeedback(kind)
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun MetaLine(item: Item, now: ZonedDateTime) {
    val due = dueLabel(item, now)
    if (due == null && item.labels.isEmpty()) return
    FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        if (due != null) {
            Text(
                due.text,
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
                color = if (due.overdue) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.align(Alignment.CenterVertically),
            )
        }
        item.labels.forEach { LabelChip(it) }
    }
}

@Composable
fun LabelChip(label: Label) {
    val shades = labelShades(label.color, isSystemInDarkTheme())
    val fill = shades?.let { Color(0xFF000000 or it.fill.toLong()) } ?: MaterialTheme.colorScheme.surfaceVariant
    val border = shades?.let { Color(0xFF000000 or it.border.toLong()) } ?: MaterialTheme.colorScheme.outlineVariant
    val text = shades?.let { Color(0xFF000000 or it.text.toLong()) } ?: MaterialTheme.colorScheme.onSurfaceVariant
    Surface(shape = RoundedCornerShape(50), color = fill, border = BorderStroke(1.dp, border)) {
        Text(label.name, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = text, modifier = Modifier.padding(horizontal = 9.dp, vertical = 3.dp))
    }
}

/**
 * The add field. While the keyboard is open, a row of [labels] shows under it;
 * the labels picked there go with the new item. With a [filter], its label is
 * picked at the start, so the new item stays in the filtered list.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun AddBar(labels: List<Label>?, filter: Label?, onOpen: () -> Unit, onAdd: suspend (String, List<String>) -> Boolean) {
    var text by rememberSaveable { mutableStateOf("") }
    var picked by rememberSaveable(filter?.id) { mutableStateOf(listOfNotNull(filter?.id)) }
    var focused by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val shown = remember(labels, filter) { labelsByUse(labels.orEmpty(), filter) }
    val submit = {
        if (!busy) {
            busy = true
            val sent = text
            // The filter's label can be picked before the label list has loaded.
            val names = (shown + listOfNotNull(filter)).distinctBy { it.id }.filter { it.id in picked }.map { it.name }
            scope.launch {
                // Text typed while the request ran stays in the field, and so do the picked labels.
                if (onAdd(sent, names) && text == sent) {
                    text = ""
                    picked = listOfNotNull(filter?.id)
                }
                busy = false
            }
        }
    }
    // The back key closes the keyboard but leaves the focus in the field, so check both.
    val withLabels = focused && WindowInsets.isImeVisible && shown.isNotEmpty()
    Surface(color = MaterialTheme.colorScheme.background) {
        Column(Modifier.navigationBarsPadding().imePadding()) {
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Row(
                Modifier.fillMaxWidth().padding(start = 12.dp, end = 12.dp, top = 12.dp, bottom = if (withLabels) 8.dp else 16.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                OutlinedTextField(
                    value = text,
                    onValueChange = { text = it },
                    placeholder = { Text("Add an item…") },
                    singleLine = true,
                    shape = RoundedCornerShape(26.dp),
                    keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences, imeAction = ImeAction.Send),
                    keyboardActions = KeyboardActions(onSend = { submit() }),
                    modifier = Modifier.weight(1f).semantics { contentDescription = "New item" }.onFocusChanged {
                        // The label counts change, so load them each time the field gets the focus.
                        if (it.isFocused && !focused) onOpen()
                        focused = it.isFocused
                    },
                )
                FilledIconButton(onClick = { submit() }, enabled = !busy, modifier = Modifier.size(52.dp)) {
                    Icon(Icons.Filled.Add, contentDescription = "Add")
                }
            }
            if (withLabels) {
                AddLabelRow(shown, picked) { label ->
                    picked = if (label.id in picked) picked - label.id else picked + label.id
                }
            }
        }
    }
}
