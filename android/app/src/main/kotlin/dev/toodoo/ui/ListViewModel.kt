package dev.toodoo.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.toodoo.TooDooApp
import dev.toodoo.api.ApiException
import dev.toodoo.api.Item
import dev.toodoo.api.ItemList
import dev.toodoo.api.Label
import dev.toodoo.api.OfflineException
import dev.toodoo.data.Session
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ListState(
    val list: ItemList? = null,
    val refreshing: Boolean = false,
    val offline: Boolean = false, // the list is the cached one
    val message: String? = null, // shown once in a snackbar
    val pending: Pending? = null, // changed on the phone, Undo still possible
    val filter: Label? = null, // only items with this label
    val labels: List<Label>? = null, // all labels, loaded when a picker opens
    val sheet: Item? = null, // the item open in the item sheet
    val gestureHint: Boolean = false, // show the hint about swipe and hold
)

/**
 * A change the list shows at once but sends when its snackbar goes away, so
 * Undo needs no request: [item] deleted, or [item] (as it was) marked done
 * or not done.
 */
data class Pending(val item: Item, val delete: Boolean)

/**
 * The list screen. Every change is sent to the server and the list is loaded
 * again after it, so the order (overdue first), counts and "hide done" are
 * always the server's.
 */
/** What the list screen shows and does; [ListViewModel] in the app, a fake in the screenshot tests. */
interface ListActions {
    val state: StateFlow<ListState>
    val username: String
    val server: String
    fun exactReminders(): Boolean
    fun messageShown()
    fun refresh()
    suspend fun add(text: String, labels: List<String>): Boolean
    fun toggle(item: Item)
    fun delete(item: Item)
    fun undo()
    fun commit(pending: Pending)
    fun gestureHintSeen()
    fun setFilter(label: Label?)
    fun loadLabels()
    fun openSheet(item: Item)
    fun closeSheet()
    suspend fun save(item: Item, text: String?, due: Pair<String, String>?): Boolean
    fun postpone(item: Item, minutes: Int)
    fun addLabel(item: Item, name: String)
    fun removeLabel(item: Item, label: Label)
    fun setLabelColor(label: Label, color: String)
    fun deleteLabel(label: Label)
    fun move(id: Long, order: List<Long>)
    fun setHideDone(hide: Boolean)
    fun clearDone()
    fun logOut()
}

class ListViewModel(private val app: TooDooApp, private val session: Session) : ViewModel(), ListActions {
    private val api = app.api(session)
    private val _state = MutableStateFlow(ListState())
    override val state: StateFlow<ListState> = _state
    override val username get() = session.username
    override val server get() = session.server

    /** Whether reminders ring on time (Android 12+ asks the user for exact alarms). */
    override fun exactReminders() = app.reminders.exact()

    init {
        viewModelScope.launch {
            app.cache.load()?.let { cached -> _state.update { if (it.list == null) it.copy(list = cached) else it } }
        }
        viewModelScope.launch {
            val seen = app.sessions.gestureHintSeen.first()
            _state.update { it.copy(gestureHint = !seen) }
        }
        refresh()
        app.reminders.startBackgroundLoads()
    }

    override fun refresh() {
        _state.update { it.copy(refreshing = true) }
        viewModelScope.launch { reload() }
    }

    private suspend fun reload() {
        try {
            val filter = _state.value.filter
            val list = api.items(filter?.id)
            val pending = _state.value.pending
            _state.update { s ->
                // A filter on a label that is gone shows the whole list again (the server ignores it).
                val f = if (s.filter?.id != filter?.id) s.filter else list.label
                s.copy(
                    list = list.with(pending), refreshing = false, offline = false, filter = f,
                    sheet = s.sheet?.let { open -> list.items.find { it.id == open.id } ?: open },
                )
            }
            if (filter == null) {
                app.cache.save(list)
                app.reminders.sync(list.items)
            } else {
                app.reminders.syncSoon()
            }
        } catch (e: Exception) {
            _state.update { it.copy(refreshing = false) }
            fail(e)
        }
    }

    /** Adds an item with the [labels] (names). Returns false if the text was refused (the field keeps it). */
    override suspend fun add(text: String, labels: List<String>): Boolean {
        if (text.isBlank()) {
            _state.update { it.copy(message = "Please type some text first.") }
            return false
        }
        return try {
            api.addItem(text, labels)
            reload()
            true
        } catch (e: Exception) {
            fail(e)
            false
        }
    }

    /** Marks the item done (or not done) on the phone; the server hears of it when the snackbar goes. */
    override fun toggle(item: Item) = hold(Pending(item, delete = false))

    /** Takes the item off the list at once; the server delete waits for the snackbar. */
    override fun delete(item: Item) = hold(Pending(item, delete = true))

    private fun hold(p: Pending) {
        // One snackbar at a time: the change before this one goes now.
        _state.value.pending?.let { commit(it) }
        _state.update { it.copy(list = it.list.with(p), pending = p) }
    }

    override fun undo() {
        _state.update { it.copy(pending = null) }
        refresh()
    }

    /** The snackbar went away without Undo: send the change. */
    override fun commit(pending: Pending) {
        val item = pending.item
        if (pending.delete && _state.value.sheet?.id == item.id) closeSheet()
        _state.update { if (it.pending == pending) it.copy(pending = null) else it }
        change { if (pending.delete) api.deleteItem(item.id) else api.setDone(item.id, !item.done) }
    }

    override fun gestureHintSeen() {
        _state.update { it.copy(gestureHint = false) }
        viewModelScope.launch { app.sessions.setGestureHintSeen() }
    }

    /** Shows only the items with [label], or all with null. */
    override fun setFilter(label: Label?) {
        _state.update { it.copy(filter = label, list = if (it.filter?.id == label?.id) it.list else null) }
        refresh()
    }

    /** Loads all labels, for the label picker and the filter. */
    override fun loadLabels() {
        viewModelScope.launch {
            try {
                val labels = api.labels()
                _state.update { it.copy(labels = labels) }
            } catch (e: Exception) {
                fail(e)
            }
        }
    }

    override fun openSheet(item: Item) = _state.update { it.copy(sheet = item) }

    override fun closeSheet() = _state.update { it.copy(sheet = null) }

    /** Saves a new text. Returns false if the server refused it. */
    suspend fun setText(item: Item, text: String): Boolean = itemChange { api.setText(item.id, text) }

    /** Sets the due date and time ("" for all day); both "" clear it. */
    suspend fun setDue(item: Item, date: String, time: String): Boolean = itemChange { api.setDue(item.id, date, time) }

    /** Saves the item sheet: a new text and a new due date (date, time), each null if unchanged. */
    override suspend fun save(item: Item, text: String?, due: Pair<String, String>?): Boolean {
        if (text != null && !setText(item, text)) return false
        if (due != null && !setDue(item, due.first, due.second)) return false
        return true
    }

    override fun postpone(item: Item, minutes: Int) = launchItemChange { api.postpone(item.id, minutes) }

    override fun addLabel(item: Item, name: String) = launchItemChange { api.addLabel(item.id, name) }

    override fun removeLabel(item: Item, label: Label) = launchItemChange { api.removeLabel(item.id, label.id) }

    override fun setLabelColor(label: Label, color: String) = change {
        api.setLabelColor(label.id, color)
        val labels = api.labels()
        _state.update { s ->
            s.copy(labels = labels, filter = s.filter?.let { f -> labels.find { it.id == f.id } ?: f })
        }
    }

    override fun deleteLabel(label: Label) = change {
        api.deleteLabel(label.id)
        val labels = api.labels()
        _state.update { s ->
            s.copy(
                labels = labels,
                filter = s.filter?.takeIf { it.id != label.id },
                sheet = s.sheet?.let { it.copy(labels = it.labels.filterNot { l -> l.id == label.id }) },
            )
        }
    }

    /**
     * Moves an item to a new place: [order] is the list's ids in their new
     * order (the screen shows it at once). The item goes right after the
     * nearest item above it that is not overdue.
     */
    override fun move(id: Long, order: List<Long>) {
        val list = _state.value.list ?: return
        val byId = list.items.associateBy { it.id }
        val at = order.indexOf(id)
        val after = order.subList(0, at.coerceAtLeast(0)).lastOrNull { byId[it]?.overdue == false } ?: 0L
        _state.update { s -> s.copy(list = s.list?.copy(items = order.mapNotNull { byId[it] })) }
        change { api.move(id, after) }
    }

    override fun setHideDone(hide: Boolean) = change { api.setHideDone(hide) }

    override fun clearDone() = change {
        val n = api.clearDone(_state.value.filter?.id)
        _state.update { it.copy(message = if (n == 1) "1 done item deleted." else "$n done items deleted.") }
    }

    override fun logOut() {
        viewModelScope.launch {
            runCatching { api.logout() }
            endSession()
        }
    }

    override fun messageShown() = _state.update { it.copy(message = null) }

    /** Sends a change that returns the item, shows it in the sheet, and reloads. */
    private suspend fun itemChange(block: suspend () -> Item): Boolean {
        return try {
            val item = block()
            _state.update { s -> s.copy(sheet = if (s.sheet?.id == item.id) item else s.sheet) }
            reload()
            true
        } catch (e: Exception) {
            fail(e)
            false
        }
    }

    private fun launchItemChange(block: suspend () -> Item) {
        viewModelScope.launch { itemChange(block) }
    }

    private fun change(block: suspend () -> Unit) {
        viewModelScope.launch {
            try {
                block()
            } catch (e: Exception) {
                fail(e)
            }
            reload()
        }
    }

    private suspend fun fail(e: Exception) {
        when {
            e is CancellationException -> throw e
            e is ApiException && e.loggedOut -> endSession()
            e is ApiException -> _state.update { it.copy(message = e.message) }
            e is OfflineException -> _state.update { it.copy(offline = true, message = "No connection. Changes need the server.") }
            else -> _state.update { it.copy(message = "Something went wrong. Please try again.") }
        }
    }

    /** Logs out on the phone; the screen changes to the log-in. */
    private suspend fun endSession() {
        app.cache.clear()
        app.reminders.clear()
        app.sessions.clear()
    }
}

/** The list as it looks with [p] done; a done item goes when done items are hidden. */
internal fun ItemList?.with(p: Pending?): ItemList? {
    if (this == null || p == null) return this
    val item = items.find { it.id == p.item.id } ?: return this
    if (p.delete) {
        return copy(
            items = items.filterNot { it.id == item.id },
            open = if (item.done) open else open - 1,
            done = if (item.done) done - 1 else done,
        )
    }
    if (item.done != p.item.done) return this // the server has it already
    val toggled = item.copy(done = !item.done)
    return copy(
        items = if (toggled.done && hideDone) items.filterNot { it.id == item.id } else items.map { if (it.id == item.id) toggled else it },
        open = if (toggled.done) open - 1 else open + 1,
        done = if (toggled.done) done + 1 else done - 1,
    )
}
