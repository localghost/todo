package dev.toodoo.ui

import androidx.compose.foundation.BorderStroke
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
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
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
fun ListScreen(vm: ListViewModel) {
    val state by vm.state.collectAsStateWithLifecycle()
    val snackbar = remember { SnackbarHostState() }
    var menuOpen by remember { mutableStateOf(false) }
    var confirmClear by remember { mutableStateOf(false) }
    val list = state.list

    LaunchedEffect(state.message) {
        state.message?.let {
            vm.messageShown()
            snackbar.showSnackbar(it)
        }
    }
    state.pendingDelete?.let { item ->
        LaunchedEffect(item.id) {
            val result = snackbar.showSnackbar("“${item.text}” deleted", actionLabel = "Undo", duration = SnackbarDuration.Short)
            if (result == SnackbarResult.ActionPerformed) vm.undoDelete() else vm.commitDelete(item)
        }
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
        bottomBar = { AddBar(onAdd = vm::add) },
        snackbarHost = { SnackbarHost(snackbar) },
        containerColor = MaterialTheme.colorScheme.background,
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = state.refreshing,
            onRefresh = vm::refresh,
            modifier = Modifier.fillMaxSize().padding(padding),
        ) {
            LazyColumn(
                contentPadding = PaddingValues(start = 12.dp, end = 12.dp, bottom = 16.dp),
                modifier = Modifier.fillMaxSize(),
            ) {
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
                        items(list.items, key = { it.id }) { item ->
                            val first = item.id == list.items.first().id
                            val last = item.id == list.items.last().id
                            ItemRow(item, now, first, last, onToggle = { vm.toggle(item) }, onDelete = { vm.delete(item) })
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
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ItemRow(item: Item, now: ZonedDateTime, first: Boolean, last: Boolean, onToggle: () -> Unit, onDelete: () -> Unit) {
    val dismiss = rememberSwipeToDismissBoxState(confirmValueChange = {
        if (it == SwipeToDismissBoxValue.EndToStart) onDelete()
        it == SwipeToDismissBoxValue.EndToStart
    })
    val shape = RoundedCornerShape(
        topStart = if (first) 12.dp else 0.dp, topEnd = if (first) 12.dp else 0.dp,
        bottomStart = if (last) 12.dp else 0.dp, bottomEnd = if (last) 12.dp else 0.dp,
    )
    val colors = MaterialTheme.colorScheme
    SwipeToDismissBox(
        state = dismiss,
        enableDismissFromStartToEnd = false,
        backgroundContent = {
            Row(
                Modifier.fillMaxSize().background(colors.error, shape).padding(end = 20.dp),
                horizontalArrangement = Arrangement.End,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(Icons.Filled.Delete, contentDescription = null, tint = colors.onError)
                Spacer(Modifier.width(8.dp))
                Text("Delete", color = colors.onError, fontWeight = FontWeight.SemiBold)
            }
        },
        modifier = Modifier.semantics {
            customActions = listOf(CustomAccessibilityAction("Delete") { onDelete(); true })
        },
    ) {
        Surface(color = colors.surface, shape = shape, border = BorderStroke(1.dp, colors.outlineVariant)) {
            Row(Modifier.fillMaxWidth().heightIn(min = 64.dp).padding(start = 4.dp), verticalAlignment = Alignment.Top) {
                CheckCircle(item, onToggle)
                Column(Modifier.weight(1f).padding(top = 12.dp, bottom = 12.dp, end = 12.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
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
}

@Composable
private fun CheckCircle(item: Item, onToggle: () -> Unit) {
    val colors = MaterialTheme.colorScheme
    val ring = if (item.overdue && !item.done) colors.error else colors.onSurfaceVariant
    IconButton(
        onClick = onToggle,
        modifier = Modifier.size(48.dp).semantics {
            stateDescription = if (item.done) "Done" else "Not done"
        },
    ) {
        Box(
            Modifier.size(24.dp)
                .background(if (item.done) colors.onSurfaceVariant else Color.Transparent, CircleShape)
                .border(2.dp, ring, CircleShape),
            contentAlignment = Alignment.Center,
        ) {
            if (item.done) Icon(Icons.Filled.Check, contentDescription = null, tint = colors.surface, modifier = Modifier.size(16.dp))
        }
    }
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

@Composable
private fun AddBar(onAdd: suspend (String) -> Boolean) {
    var text by rememberSaveable { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val submit = {
        if (!busy) {
            busy = true
            val sent = text
            scope.launch {
                // Text typed while the request ran stays in the field.
                if (onAdd(sent) && text == sent) text = ""
                busy = false
            }
        }
    }
    Surface(color = MaterialTheme.colorScheme.background) {
        Column {
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Row(
                Modifier.fillMaxWidth().navigationBarsPadding().imePadding().padding(start = 12.dp, end = 12.dp, top = 12.dp, bottom = 16.dp),
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
                    modifier = Modifier.weight(1f).semantics { contentDescription = "New item" },
                )
                FilledIconButton(onClick = { submit() }, enabled = !busy, modifier = Modifier.size(52.dp)) {
                    Icon(Icons.Filled.Add, contentDescription = "Add")
                }
            }
        }
    }
}
