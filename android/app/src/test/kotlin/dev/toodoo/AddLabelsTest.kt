package dev.toodoo

import dev.toodoo.api.Label
import dev.toodoo.ui.labelsByUse
import org.junit.Assert.assertEquals
import org.junit.Test

/** The order of the label chips under the add field. */
class AddLabelsTest {
    private val work = Label(1, "work", "#1A3C99", 6)
    private val home = Label(2, "home", "#1E6B32", 5)
    private val family = Label(3, "family", "#5B2D91", 4)
    private val urgent = Label(4, "urgent", "#9C2A22", 1)
    private val health = Label(5, "Health", "#145C58", 1)
    private val unused = Label(6, "travel", "#8A4410", 0)

    @Test
    fun mostItemsFirst() {
        assertEquals(listOf(work, home, family), labelsByUse(listOf(family, work, home), filter = null))
    }

    // The same count sorts by name, also when the case differs.
    @Test
    fun sameCountByName() {
        assertEquals(listOf(health, urgent), labelsByUse(listOf(urgent, health), filter = null))
    }

    @Test
    fun unusedLabelsLast() {
        assertEquals(listOf(home, unused), labelsByUse(listOf(unused, home), filter = null))
    }

    @Test
    fun filterLabelFirst() {
        assertEquals(listOf(family, work, home), labelsByUse(listOf(work, home, family), filter = family))
    }

    // The filter is a copy of the label (from the list), so it matches by id.
    @Test
    fun filterMatchesById() {
        val filter = family.copy(items = null)
        assertEquals(listOf(family, work), labelsByUse(listOf(work, family), filter = filter))
    }
}
