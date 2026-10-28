package com.codeleet.toodoo

import com.codeleet.toodoo.api.Item
import com.codeleet.toodoo.api.ItemList
import com.codeleet.toodoo.ui.Pending
import com.codeleet.toodoo.ui.with
import org.junit.Assert.assertEquals
import org.junit.Assert.assertSame
import org.junit.Test

/** How the list looks while a swipe waits for its snackbar. */
class PendingTest {
    private val rent = Item(1, "Pay rent", false)
    private val bank = Item(2, "Call the bank", true)
    private val list = ItemList(listOf(rent, bank), open = 1, done = 1, hideDone = false)

    @Test
    fun markDone() {
        val shown = list.with(Pending(rent, delete = false))!!
        assertEquals(listOf(true, true), shown.items.map { it.done })
        assertEquals(0, shown.open)
        assertEquals(2, shown.done)
    }

    @Test
    fun markNotDone() {
        val shown = list.with(Pending(bank, delete = false))!!
        assertEquals(listOf(false, false), shown.items.map { it.done })
        assertEquals(2, shown.open)
        assertEquals(0, shown.done)
    }

    // With done items hidden, a row marked done leaves the list.
    @Test
    fun markDoneWhileHidden() {
        val hidden = ItemList(listOf(rent), open = 1, done = 1, hideDone = true)
        val shown = hidden.with(Pending(rent, delete = false))!!
        assertEquals(emptyList<Item>(), shown.items)
        assertEquals(0, shown.open)
        assertEquals(2, shown.done)
    }

    // A reload after the change reached the server must not flip it back.
    @Test
    fun alreadyOnServer() {
        val reloaded = ItemList(listOf(rent.copy(done = true), bank), open = 0, done = 2, hideDone = false)
        assertSame(reloaded, reloaded.with(Pending(rent, delete = false)))
    }

    @Test
    fun delete() {
        val shown = list.with(Pending(rent, delete = true))!!
        assertEquals(listOf(bank), shown.items)
        assertEquals(0, shown.open)
        assertEquals(1, shown.done)
    }
}
