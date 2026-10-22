package dev.toodoo

import androidx.activity.compose.LocalActivityResultRegistryOwner
import androidx.activity.result.ActivityResultRegistry
import androidx.activity.result.ActivityResultRegistryOwner
import androidx.activity.result.contract.ActivityResultContract
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.core.app.ActivityOptionsCompat
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import app.cash.paparazzi.DeviceConfig
import app.cash.paparazzi.Paparazzi
import com.android.resources.NightMode
import dev.toodoo.api.Item
import dev.toodoo.api.ItemList
import dev.toodoo.api.Label
import dev.toodoo.ui.ItemSheetBody
import dev.toodoo.ui.LabelPickerBody
import dev.toodoo.ui.ListActions
import dev.toodoo.ui.ListScreen
import dev.toodoo.ui.ListState
import dev.toodoo.ui.PickerMode
import dev.toodoo.ui.TimePickerBody
import dev.toodoo.ui.TooDooTheme
import java.time.LocalDate
import java.time.ZoneId
import java.time.ZonedDateTime
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Rule
import org.junit.Test

/**
 * Renders the screens on the JVM (Paparazzi), like the mockups on page
 * "17 · Android app". `./gradlew recordPaparazziDebug` writes the images to
 * app/src/test/snapshots/.
 */
class ScreenshotTest {
    @get:Rule
    val paparazzi = Paparazzi(deviceConfig = DeviceConfig.PIXEL_5, maxPercentDifference = 0.1)

    private val home = Label(1, "home", "#38c75d", 6)
    private val work = Label(2, "work", "#2555da", 4)
    private val money = Label(3, "money", "#e9bd16", 2)
    private val errands = Label(4, "errands", "#e07a1f", 1)
    private val health = Label(5, "health", "#2ed1c8", 3)

    private fun at(daysFromToday: Long, hour: Int, minute: Int = 0): String {
        val zone = ZoneId.systemDefault()
        return ZonedDateTime.of(LocalDate.now(zone).plusDays(daysFromToday).atTime(hour, minute), zone).toInstant().toString()
    }

    private val items = listOf(
        Item(10, "Pay rent", false, dueAt = at(-1, 18), overdue = true, labels = listOf(home)),
        Item(11, "Send invoice to Kowalski", false, dueAt = at(0, 14, 30), labels = listOf(money, work)),
        Item(12, "Buy batteries and light bulbs", false, labels = listOf(errands, home)),
        Item(13, "Book dentist appointment", false, dueAt = at(3, 9), dueAllDay = true, labels = listOf(health)),
        Item(14, "Water the plants", false),
        Item(15, "Call the bank about the card", true, labels = listOf(money)),
    )

    private fun screen(state: ListState) = object : ListActions {
        override val state = MutableStateFlow(state)
        override val username = "zbyszek"
        override val server = "https://toodoo.fly.dev"
        override fun exactReminders() = true
        override fun messageShown() {}
        override fun refresh() {}
        override suspend fun add(text: String) = true
        override fun toggle(item: Item) {}
        override fun delete(item: Item) {}
        override fun undoDelete() {}
        override fun commitDelete(item: Item) {}
        override fun setFilter(label: Label?) {}
        override fun loadLabels() {}
        override fun openSheet(item: Item) {}
        override fun closeSheet() {}
        override suspend fun save(item: Item, text: String?, due: Pair<String, String>?) = true
        override fun postpone(item: Item, minutes: Int) {}
        override fun addLabel(item: Item, name: String) {}
        override fun removeLabel(item: Item, label: Label) {}
        override fun setLabelColor(label: Label, color: String) {}
        override fun deleteLabel(label: Label) {}
        override fun move(id: Long, order: List<Long>) {}
        override fun setHideDone(hide: Boolean) {}
        override fun clearDone() {}
        override fun logOut() {}
    }

    private fun snap(dark: Boolean = false, content: @Composable () -> Unit) {
        if (dark) paparazzi.unsafeUpdateConfig(DeviceConfig.PIXEL_5.copy(nightMode = NightMode.NIGHT))
        paparazzi.snapshot {
            // The list screen asks for the notification permission through this.
            val registry = object : ActivityResultRegistryOwner {
                override val activityResultRegistry = object : ActivityResultRegistry() {
                    override fun <I, O> onLaunch(requestCode: Int, contract: ActivityResultContract<I, O>, input: I, options: ActivityOptionsCompat?) {}
                }
            }
            CompositionLocalProvider(LocalActivityResultRegistryOwner provides registry) {
                TooDooTheme(dark = dark) {
                    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) { content() }
                }
            }
        }
    }

    private val list = ItemList(items, open = 5, done = 1, hideDone = false)

    @Test
    fun list() = snap { ListScreen(screen(ListState(list = list))) }

    @Test
    fun listDark() = snap(dark = true) { ListScreen(screen(ListState(list = list))) }

    @Test
    fun listFiltered() = snap {
        val only = items.filter { home in it.labels }
        ListScreen(screen(ListState(list = ItemList(only, 2, 0, false, home), filter = home)))
    }

    @Test
    fun empty() = snap { ListScreen(screen(ListState(list = ItemList(emptyList(), 0, 0, false)))) }

    @Test
    fun itemSheet() = snap {
        ItemSheetBody(items[1], onSave = { _, _ -> true }, onPostpone = {}, onRemoveLabel = {}, onAddLabel = {}, onDelete = {}, onDismiss = {})
    }

    // No date: "Date", and the time button is off.
    @Test
    fun itemSheetNoDate() = snap {
        ItemSheetBody(items[2], onSave = { _, _ -> true }, onPostpone = {}, onRemoveLabel = {}, onAddLabel = {}, onDelete = {}, onDismiss = {})
    }

    // A 360 dp phone and a date in another year, the longest date text: it must fit.
    @Test
    fun itemSheetSmallPhone() {
        paparazzi.unsafeUpdateConfig(DeviceConfig.NEXUS_5)
        val nextYear = ZonedDateTime.now().plusYears(1).withHour(14).withMinute(30).toInstant().toString()
        snap {
            ItemSheetBody(items[1].copy(dueAt = nextYear), onSave = { _, _ -> true }, onPostpone = {}, onRemoveLabel = {}, onAddLabel = {}, onDelete = {}, onDismiss = {})
        }
    }

    @Test
    fun itemSheetAllDay() = snap {
        ItemSheetBody(items[3], onSave = { _, _ -> true }, onPostpone = {}, onRemoveLabel = {}, onAddLabel = {}, onDelete = {}, onDismiss = {})
    }

    @OptIn(ExperimentalMaterial3Api::class)
    @Test
    fun timePicker() = snap {
        Box(Modifier.padding(24.dp)) {
            TimePickerBody(rememberTimePickerState(14, 30, is24Hour = true), onOk = {}, onAllDay = {}, onDismiss = {})
        }
    }

    @OptIn(ExperimentalMaterial3Api::class)
    @Test
    fun timeInput() = snap {
        Box(Modifier.padding(24.dp)) {
            TimePickerBody(rememberTimePickerState(14, 30, is24Hour = true), onOk = {}, onAllDay = {}, onDismiss = {}, startTyping = true)
        }
    }

    @Test
    fun labelPicker() = snap {
        LabelPickerBody(
            labels = listOf(health, home, Label(6, "help desk", "#8A4410", 1)),
            mode = PickerMode.ForItem("Send invoice to Kowalski", setOf(home.id)),
            query = "he", onQuery = {}, onPick = {}, create = {}, canCreate = true,
            onClearFilter = {}, onColor = {}, onDelete = {}, onDismiss = {},
        )
    }
}
