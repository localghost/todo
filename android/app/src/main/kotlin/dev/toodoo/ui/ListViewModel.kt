package dev.toodoo.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.toodoo.TooDooApp
import dev.toodoo.api.ApiException
import dev.toodoo.api.Item
import dev.toodoo.api.ItemList
import dev.toodoo.api.OfflineException
import dev.toodoo.data.Session
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ListState(
    val list: ItemList? = null,
    val refreshing: Boolean = false,
    val offline: Boolean = false, // the list is the cached one
    val message: String? = null, // shown once in a snackbar
    val pendingDelete: Item? = null, // deleted on the phone, Undo still possible
)

/**
 * The list screen. Every change is sent to the server and the list is loaded
 * again after it, so the order (overdue first), counts and "hide done" are
 * always the server's.
 */
class ListViewModel(private val app: TooDooApp, private val session: Session) : ViewModel() {
    private val api = app.api(session)
    private val _state = MutableStateFlow(ListState())
    val state: StateFlow<ListState> = _state
    val username get() = session.username

    init {
        viewModelScope.launch {
            app.cache.load()?.let { cached -> _state.update { if (it.list == null) it.copy(list = cached) else it } }
        }
        refresh()
    }

    fun refresh() {
        _state.update { it.copy(refreshing = true) }
        viewModelScope.launch { reload() }
    }

    private suspend fun reload() {
        try {
            val list = api.items()
            val pending = _state.value.pendingDelete
            _state.update {
                it.copy(list = list.withoutItem(pending), refreshing = false, offline = false)
            }
            app.cache.save(list)
        } catch (e: Exception) {
            _state.update { it.copy(refreshing = false) }
            fail(e)
        }
    }

    /** Adds an item. Returns false if the text was refused (the field keeps it). */
    suspend fun add(text: String): Boolean {
        if (text.isBlank()) {
            _state.update { it.copy(message = "Please type some text first.") }
            return false
        }
        return try {
            api.addItem(text)
            reload()
            true
        } catch (e: Exception) {
            fail(e)
            false
        }
    }

    fun toggle(item: Item) {
        // Show the change at once; the reload brings the server's view.
        _state.update { s -> s.copy(list = s.list?.replace(item.copy(done = !item.done))) }
        change { api.setDone(item.id, !item.done) }
    }

    /** Takes the item off the list at once; the server delete waits for the snackbar. */
    fun delete(item: Item) {
        _state.value.pendingDelete?.let { commitDelete(it) }
        _state.update { it.copy(list = it.list.withoutItem(item), pendingDelete = item) }
    }

    fun undoDelete() {
        _state.update { it.copy(pendingDelete = null) }
        refresh()
    }

    /** The snackbar went away without Undo: delete on the server. */
    fun commitDelete(item: Item) {
        _state.update { if (it.pendingDelete?.id == item.id) it.copy(pendingDelete = null) else it }
        change { api.deleteItem(item.id) }
    }

    fun setHideDone(hide: Boolean) = change { api.setHideDone(hide) }

    fun clearDone() = change {
        val n = api.clearDone()
        _state.update { it.copy(message = if (n == 1) "1 done item deleted." else "$n done items deleted.") }
    }

    fun logOut() {
        viewModelScope.launch {
            runCatching { api.logout() }
            endSession()
        }
    }

    fun messageShown() = _state.update { it.copy(message = null) }

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
        app.sessions.clear()
    }
}

private fun ItemList?.withoutItem(item: Item?): ItemList? {
    if (this == null || item == null || items.none { it.id == item.id }) return this
    return copy(
        items = items.filterNot { it.id == item.id },
        open = if (item.done) open else open - 1,
        done = if (item.done) done - 1 else done,
    )
}

private fun ItemList.replace(item: Item): ItemList = copy(items = items.map { if (it.id == item.id) item else it })
