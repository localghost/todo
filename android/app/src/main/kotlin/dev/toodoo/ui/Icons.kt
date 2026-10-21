package dev.toodoo.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.dp

// Icons that material-icons-core lacks, drawn like the mockups (page 17).

/** The grip of a row: two columns of three dots. */
@Composable
fun GripIcon(color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(12.dp, 18.dp)) {
        val r = 1.6.dp.toPx()
        for (x in listOf(3f, 9f)) for (y in listOf(3f, 9f, 15f)) {
            drawCircle(color, r, Offset(x.dp.toPx(), y.dp.toPx()))
        }
    }
}

/** A tag, for the label filter. */
@Composable
fun TagIcon(color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(22.dp)) {
        val u = size.width / 24f
        val path = Path().apply {
            moveTo(3 * u, 12 * u)
            lineTo(3 * u, 4 * u)
            quadraticTo(3 * u, 3 * u, 4 * u, 3 * u)
            lineTo(12 * u, 3 * u)
            lineTo(21 * u, 12 * u)
            lineTo(12 * u, 21 * u)
            close()
        }
        drawPath(path, color, style = Stroke(width = 1.8f * u, join = StrokeJoin.Round))
        drawCircle(color, 1.6f * u, Offset(8 * u, 8 * u))
    }
}
