package dev.toodoo

import androidx.activity.compose.LocalActivityResultRegistryOwner
import androidx.activity.result.ActivityResultRegistry
import androidx.activity.result.ActivityResultRegistryOwner
import androidx.activity.result.contract.ActivityResultContract
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.ui.Alignment
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.sp
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
import com.android.resources.ScreenOrientation
import dev.toodoo.api.Item
import dev.toodoo.api.ItemList
import dev.toodoo.api.Label
import dev.toodoo.ui.AddBarPreview
import dev.toodoo.ui.LabelChipRow
import dev.toodoo.ui.ItemSheetBody
import dev.toodoo.ui.LabelPickerBody
import dev.toodoo.ui.ListActions
import dev.toodoo.ui.ListScreen
import dev.toodoo.ui.ListState
import dev.toodoo.ui.LoginActions
import dev.toodoo.ui.LoginScreen
import dev.toodoo.ui.LoginState
import dev.toodoo.ui.Pending
import dev.toodoo.ui.PickerMode
import dev.toodoo.ui.TimePickerBody
import dev.toodoo.ui.TooDooTheme
import dev.toodoo.ui.labelsByUse
import java.time.LocalDate
import java.time.ZoneId
import java.time.ZonedDateTime
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.Ignore
import org.junit.Rule
import org.junit.Test

/**
 * The data and the screens that the screenshot tests share: [ScreenshotTest]
 * for the guide and [PlayScreenshotTest] for Google Play.
 */
abstract class Screens {
    abstract val paparazzi: Paparazzi

    protected val home = Label(1, "home", "#38c75d", 6)
    protected val work = Label(2, "work", "#2555da", 4)
    protected val money = Label(3, "money", "#e9bd16", 2)
    protected val errands = Label(4, "errands", "#e07a1f", 1)
    protected val health = Label(5, "health", "#2ed1c8", 3)

    protected fun at(daysFromToday: Long, hour: Int, minute: Int = 0): String {
        val zone = ZoneId.systemDefault()
        return ZonedDateTime.of(LocalDate.now(zone).plusDays(daysFromToday).atTime(hour, minute), zone).toInstant().toString()
    }

    protected val items = listOf(
        Item(10, "Pay rent", false, dueAt = at(-1, 18), overdue = true, labels = listOf(home)),
        Item(11, "Send invoice to Kowalski", false, dueAt = at(0, 14, 30), labels = listOf(money, work)),
        Item(12, "Buy batteries and light bulbs", false, labels = listOf(errands, home)),
        Item(13, "Book dentist appointment", false, dueAt = at(3, 9), dueAllDay = true, labels = listOf(health)),
        Item(14, "Water the plants", false),
        Item(15, "Call the bank about the card", true, labels = listOf(money)),
    )

    protected fun screen(state: ListState) = object : ListActions {
        override val state = MutableStateFlow(state)
        override val username = "zbyszek"
        override val server = "https://toodoo.fly.dev"
        override fun exactReminders() = true
        override fun messageShown() {}
        override fun refresh() {}
        override suspend fun add(text: String, labels: List<String>) = true
        override fun toggle(item: Item) {}
        override fun delete(item: Item) {}
        override fun undo() {}
        override fun commit(pending: Pending) {}
        override fun gestureHintSeen() {}
        override fun setFilter(label: Label?) {}
        override fun loadLabels() {}
        override fun openSheet(item: Item) {}
        override fun closeSheet() {}
        override suspend fun save(item: Item, text: String?, due: Pair<String, String>?) = true
        override fun postpone(item: Item, minutes: Int) {}
        override fun addLabel(item: Item, name: String) {}
        override fun createLabel(name: String) {}
        override fun removeLabel(item: Item, label: Label) {}
        override fun setLabelColor(label: Label, color: String) {}
        override fun deleteLabel(label: Label) {}
        override fun move(id: Long, order: List<Long>) {}
        override fun setHideDone(hide: Boolean) {}
        override fun clearDone() {}
        override fun logOut() {}
    }

    protected fun snap(dark: Boolean = false, device: DeviceConfig? = null, content: @Composable () -> Unit) {
        if (dark || device != null) {
            paparazzi.unsafeUpdateConfig((device ?: DeviceConfig.PIXEL_5).copy(nightMode = if (dark) NightMode.NIGHT else NightMode.NOTNIGHT))
        }
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

    protected val list = ItemList(items, open = 5, done = 1, hideDone = false)

    // All labels, as the item sheet loads them (counts as in the list above).
    protected val sheetLabels = listOf(home, work, money, errands, health)

    /** A plain keyboard, like the one Android shows; only a picture. */
    @Composable
    protected fun Keyboard() {
        val rows = listOf("qwertyuiop", "asdfghjkl", "⇧zxcvbnm⌫").map { r -> r.map { it.toString() } } +
            listOf(listOf("?123", ",", " ", ".", "↵"))
        Column(
            Modifier.fillMaxWidth().background(Color(0xFFE8EAED)).padding(start = 4.dp, end = 4.dp, top = 8.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            rows.forEachIndexed { i, keys ->
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = if (i == 1) 18.dp else 0.dp),
                    horizontalArrangement = Arrangement.spacedBy(5.dp),
                ) {
                    keys.forEach { k ->
                        val wide = k.length > 1 || k in listOf("⇧", "⌫")
                        val weight = when (k) { " " -> 5f; "?123", "↵" -> 1.6f; else -> if (wide) 1.4f else 1f }
                        val fill = if (wide || k == "," || k == ".") Color(0xFFD3D6DB) else Color.White
                        Box(
                            Modifier.weight(weight).height(46.dp).clip(RoundedCornerShape(6.dp)).background(if (k == "↵") Color(0xFF2450C7) else fill),
                            contentAlignment = Alignment.Center,
                        ) {
                            Text(k, fontSize = if (k.length > 1) 15.sp else 21.sp, color = if (k == "↵") Color.White else Color(0xFF1F1F1F))
                        }
                    }
                }
            }
        }
    }
}

/**
 * Renders the screens on the JVM (Paparazzi), like the mockups on page
 * "17 · Android app". `./gradlew recordPaparazziDebug` writes the images to
 * app/src/test/snapshots/.
 */
class ScreenshotTest : Screens() {
    @get:Rule
    override val paparazzi = Paparazzi(deviceConfig = DeviceConfig.PIXEL_5, maxPercentDifference = 0.1)

    // The login screen, with the link to the privacy policy under the sign-up
    // link (canvas page 17, "link to the privacy policy", option A).
    @Test
    fun login() = snap {
        LoginScreen(object : LoginActions {
            override val state = MutableStateFlow(LoginState(username = "zbyszek", server = "https://toodoo.fly.dev"))
            override fun setUsername(v: String) {}
            override fun setPassword(v: String) {}
            override fun setServer(v: String) {}
            override fun logIn() {}
        })
    }

    @Test
    fun list() = snap { ListScreen(screen(ListState(list = list))) }

    // The first start after the update: the hint about swipe and hold.
    @Test
    fun listHint() = snap { ListScreen(screen(ListState(list = list, gestureHint = true))) }

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
        ItemSheetBody(items[1], onSave = { _, _ -> true }, onPostpone = {}, labels = sheetLabels, onToggleLabel = {}, onAllLabels = {}, onToggleDone = {}, onDelete = {}, onDismiss = {})
    }

    // No date: "Date", and the time button is off.
    @Test
    fun itemSheetNoDate() = snap {
        ItemSheetBody(items[2], onSave = { _, _ -> true }, onPostpone = {}, labels = sheetLabels, onToggleLabel = {}, onAllLabels = {}, onToggleDone = {}, onDelete = {}, onDismiss = {})
    }

    // A 360 dp phone and a date in another year, the longest date text: it must fit.
    @Test
    fun itemSheetSmallPhone() {
        paparazzi.unsafeUpdateConfig(DeviceConfig.NEXUS_5)
        val nextYear = ZonedDateTime.now().plusYears(1).withHour(14).withMinute(30).toInstant().toString()
        snap {
            ItemSheetBody(items[1].copy(dueAt = nextYear), onSave = { _, _ -> true }, onPostpone = {}, labels = sheetLabels, onToggleLabel = {}, onAllLabels = {}, onToggleDone = {}, onDelete = {}, onDismiss = {})
        }
    }

    @Test
    fun itemSheetAllDay() = snap {
        ItemSheetBody(items[3], onSave = { _, _ -> true }, onPostpone = {}, labels = sheetLabels, onToggleLabel = {}, onAllLabels = {}, onToggleDone = {}, onDelete = {}, onDismiss = {})
    }

    @OptIn(ExperimentalMaterial3Api::class)
    @Test
    fun timePicker() = snap {
        Box(Modifier.padding(24.dp)) {
            TimePickerBody(rememberTimePickerState(14, 30, is24Hour = true), onOk = {}, onAllDay = {}, onDismiss = {})
        }
    }

    // The layoutlib of Paparazzi 2.0.0-alpha05 (API 36) fails here inside
    // HandlerThread (Thread.setPosixNicenessInternal). Paparazzi 2.0.0-alpha05.1
    // has a newer layoutlib, but it needs AGP 9. The snapshot stays as it was.
    @Ignore("layoutlib 16.2.1 bug, see above")
    @OptIn(ExperimentalMaterial3Api::class)
    @Test
    fun timeInput() = snap {
        Box(Modifier.padding(24.dp)) {
            TimePickerBody(rememberTimePickerState(14, 30, is24Hour = true), onOk = {}, onAllDay = {}, onDismiss = {}, startTyping = true)
        }
    }

    // The chips under the add field while the keyboard is open (Paparazzi shows no keyboard).
    private fun addLabels(dark: Boolean) {
        // A strip as wide as the phone: 1080 x 260 px is 393 x 95 dp.
        snap(dark, DeviceConfig.PIXEL_5.copy(screenHeight = 260, orientation = ScreenOrientation.LANDSCAPE)) {
            Box(Modifier.padding(top = 16.dp)) {
                LabelChipRow(labelsByUse(listOf(money, errands, health, home, work), filter = null), picked = listOf(home.id, health.id), "Labels for the new item", onToggle = {})
            }
        }
    }

    // The whole screen while an item is added: the row of labels between the
    // add field and the keyboard (the guide image). The keyboard is drawn here.
    @Test
    fun addLabelsKeyboard() = snap {
        Column(Modifier.fillMaxSize()) {
            Box(Modifier.weight(1f)) {
                ListScreen(
                    screen(ListState(list = list, labels = sheetLabels)),
                    AddBarPreview("Buy vitamins", listOf(home.id, health.id)),
                )
            }
            Keyboard()
        }
    }

    @Test
    fun addLabels() = addLabels(dark = false)

    @Test
    fun addLabelsDark() = addLabels(dark = true)

    @Test
    fun labelPicker() = snap {
        LabelPickerBody(
            labels = listOf(health, home, Label(6, "help desk", "#8A4410", 1)),
            mode = PickerMode.ForItem("Send invoice to Kowalski", setOf(home.id)),
            query = "he", onQuery = {}, onPick = {}, create = {}, canCreate = true,
            onClearFilter = {}, onColor = {}, onDelete = {}, onDismiss = {},
        )
    }

    // The list from the top bar, as it opens: the placeholder and the hint.
    @Test
    fun labelPickerFilter() = snap {
        LabelPickerBody(
            labels = listOf(errands, health, home),
            mode = PickerMode.Filter(null),
            query = "", onQuery = {}, onPick = {}, create = {}, canCreate = false,
            onClearFilter = {}, onColor = {}, onDelete = {}, onDismiss = {},
        )
    }

    // The list from the top bar can make a label too (with no items).
    @Test
    fun labelPickerFilterCreate() = snap {
        LabelPickerBody(
            labels = listOf(errands, health, home),
            mode = PickerMode.Filter(null),
            query = "garden", onQuery = {}, onPick = {}, create = {}, canCreate = true,
            onClearFilter = {}, onColor = {}, onDelete = {}, onDismiss = {},
        )
    }
}
