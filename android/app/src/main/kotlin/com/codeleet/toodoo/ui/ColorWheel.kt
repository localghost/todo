package com.codeleet.toodoo.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.size
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableDoubleStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import com.codeleet.toodoo.api.Label
import kotlin.math.PI
import kotlin.math.atan2
import kotlin.math.cos
import kotlin.math.hypot
import kotlin.math.min
import kotlin.math.sin

// The color wheel, as on the web (static/wheel.js): the angle is the hue (red
// at the top, then clockwise), the distance from the middle the strength; the
// middle is gray. The color is hsl(hue, strength, 50 %).

/** Returns (hue, strength) for a point dx, dy from the middle, in parts of the wheel's width (-0.5 to 0.5). */
fun wheelPick(dx: Double, dy: Double): Pair<Double, Double> =
    Pair((atan2(dx, -dy) * 180 / PI + 360) % 360, min(1.0, 2 * hypot(dx, dy)))

/** The color of a point of the wheel. */
fun wheelColor(hue: Double, strength: Double): String = hslToHex(hue, strength, 0.5)

/** Asks for a new color of [label]; [onPick] gets "#rrggbb". */
@Composable
fun ColorWheelDialog(label: Label, onPick: (String) -> Unit, onDismiss: () -> Unit) {
    val start = remember(label.color) { hexToHsl(label.color) ?: Triple(0.0, 0.0, 0.5) }
    var hue by remember { mutableDoubleStateOf(start.first) }
    var strength by remember { mutableDoubleStateOf(start.second) }
    val color = wheelColor(hue, strength)
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Color of “${label.name}”") },
        text = {
            Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Wheel(hue, strength) { h, s ->
                    hue = h
                    strength = s
                }
                LabelChip(label.copy(color = color))
            }
        },
        confirmButton = { TextButton(onClick = { onPick(color) }) { Text("Done") } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}

@Composable
private fun Wheel(hue: Double, strength: Double, onChange: (Double, Double) -> Unit) {
    // The sweep starts at 3 o'clock; the wheel's hue 0 is at 12.
    val sweep = remember { (0..36).map { i -> hexColor(wheelColor((i * 10.0 + 90) % 360, 1.0)) } }
    fun pick(p: Offset, size: IntSize) {
        val (h, s) = wheelPick(p.x / size.width - 0.5, p.y / size.height - 0.5)
        onChange(h, s)
    }
    Canvas(
        Modifier.size(200.dp)
            .semantics { contentDescription = "Color wheel, hue ${hue.toInt()} degrees, strength ${(strength * 100).toInt()} percent" }
            .pointerInput(Unit) { detectTapGestures { pick(it, size) } }
            .pointerInput(Unit) {
                detectDragGestures(onDragStart = { pick(it, size) }) { change, _ ->
                    change.consume()
                    pick(change.position, size)
                }
            },
    ) {
        val r = size.minDimension / 2
        drawCircle(Brush.sweepGradient(sweep, center), r)
        drawCircle(Brush.radialGradient(listOf(Color(0xFF808080), Color(0x00808080)), center, r), r)
        val a = hue * PI / 180
        val at = Offset(center.x + (r * strength * sin(a)).toFloat(), center.y - (r * strength * cos(a)).toFloat())
        drawCircle(hexColor(wheelColor(hue, strength)), 9.dp.toPx(), at)
        drawCircle(Color.White, 9.dp.toPx(), at, style = Stroke(3.dp.toPx()))
        drawCircle(Color(0xFF1F1E1B), 10.5.dp.toPx(), at, style = Stroke(1.dp.toPx()))
    }
}

/** "#rrggbb" as a color. */
fun hexColor(hex: String): Color = Color(0xFF000000 or hex.removePrefix("#").toLong(16))
