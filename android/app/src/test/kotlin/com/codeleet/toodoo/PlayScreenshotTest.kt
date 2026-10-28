package com.codeleet.toodoo

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.ui.Modifier
import app.cash.paparazzi.DeviceConfig
import app.cash.paparazzi.Paparazzi
import app.cash.paparazzi.Snapshot
import app.cash.paparazzi.SnapshotHandler
import com.codeleet.toodoo.ui.AddBarPreview
import com.codeleet.toodoo.ui.ItemSheetBody
import com.codeleet.toodoo.ui.LabelPickerBody
import com.codeleet.toodoo.ui.ListScreen
import com.codeleet.toodoo.ui.ListState
import com.codeleet.toodoo.ui.PickerMode
import java.io.File
import java.lang.reflect.Proxy
import org.junit.Rule
import org.junit.Test

/**
 * The app screens for the Google Play screenshots (canvas page 17, "Google Play
 * graphics", option A): 1080 x 1920, because Google Play takes no screen longer
 * than twice its width. These go at full size (useDeviceResolution; Paparazzi
 * makes its own images at most 1000 px high) to app/build/play-screens/;
 * android/play/make.sh puts the captions and the frames around them.
 */
class PlayScreenshotTest : Screens() {
    @get:Rule
    override val paparazzi = Paparazzi(
        deviceConfig = DeviceConfig.PIXEL_5.copy(screenHeight = 1920),
        snapshotHandler = FullSize(File("build/play-screens")),
        useDeviceResolution = true,
    )

    @Test
    fun play1List() = snap { ListScreen(screen(ListState(list = list))) }

    @Test
    fun play2Swipe() = snap { ListScreen(screen(ListState(list = list, gestureHint = true))) }

    @Test
    fun play3ItemSheet() = snap {
        ItemSheetBody(items[1], onSave = { _, _ -> true }, onPostpone = {}, labels = sheetLabels, onToggleLabel = {}, onAllLabels = {}, onToggleDone = {}, onDelete = {}, onDismiss = {})
    }

    @Test
    fun play4AddLabels() = snap {
        Column(Modifier.fillMaxSize()) {
            Box(Modifier.weight(1f)) {
                ListScreen(screen(ListState(list = list, labels = sheetLabels)), AddBarPreview("Buy vitamins", listOf(home.id, health.id)))
            }
            Keyboard()
        }
    }

    @Test
    fun play5LabelFilter() = snap {
        LabelPickerBody(
            labels = listOf(errands, health, home),
            mode = PickerMode.Filter(null),
            query = "", onQuery = {}, onPick = {}, create = {}, canCreate = false,
            onClearFilter = {}, onColor = {}, onDelete = {}, onDismiss = {},
        )
    }

    private companion object {
        /**
         * A handler that writes each image as it is, named after the test
         * method. The test code compiles against android.jar, which has no
         * java.awt, so the handler is a Proxy and ImageIO is called by name;
         * Paparazzi runs on a normal JVM, which has both.
         */
        fun FullSize(dir: File): SnapshotHandler {
            val write = Class.forName("javax.imageio.ImageIO")
                .getMethod("write", Class.forName("java.awt.image.RenderedImage"), String::class.java, File::class.java)
            fun <T> proxy(type: Class<T>, call: (String, Array<Any?>) -> Any?): T =
                type.cast(Proxy.newProxyInstance(type.classLoader, arrayOf(type)) { _, m, args -> call(m.name, args ?: emptyArray()) })
            return proxy(SnapshotHandler::class.java) { name, args ->
                if (name != "newFrameHandler") return@proxy null
                val snapshot = args[0] as Snapshot
                proxy(SnapshotHandler.FrameHandler::class.java) { frameCall, frameArgs ->
                    if (frameCall == "handle") {
                        dir.mkdirs()
                        write.invoke(null, frameArgs[0], "PNG", File(dir, snapshot.testName.methodName + ".png"))
                    }
                    null
                }
            }
        }
    }
}
