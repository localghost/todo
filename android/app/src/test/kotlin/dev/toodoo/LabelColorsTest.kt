package dev.toodoo

import dev.toodoo.ui.hslToRgb
import dev.toodoo.ui.labelShades
import dev.toodoo.ui.luminance
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class LabelColorsTest {
    private fun rgb(c: Int) = doubleArrayOf((c shr 16 and 255) / 255.0, (c shr 8 and 255) / 255.0, (c and 255) / 255.0)

    private fun contrast(a: Int, b: Int): Double {
        val la = luminance(rgb(a))
        val lb = luminance(rgb(b))
        return (maxOf(la, lb) + 0.05) / (minOf(la, lb) + 0.05)
    }

    @Test
    fun sameShadesAsTheWeb() {
        // Values from static/chips.js labelShades("#2555da").
        val s = labelShades("#2555da")!!
        assertEquals("fill", 0xeef1fc, s.fill)
        assertEquals("border", 0xc6d3f5, s.border)
        assertEquals("text", 0x1e44ae, s.text)
    }

    @Test
    fun textHasSevenToOneContrast() {
        for (hex in listOf("#868279", "#d1382e", "#e5711a", "#e9bd16", "#38c75d", "#2ed1c8", "#2555da", "#6b44bb", "#ffff00", "#000000")) {
            for (dark in listOf(false, true)) {
                val s = labelShades(hex, dark)!!
                assertTrue("$hex dark=$dark: ${contrast(s.text, s.fill)}", contrast(s.text, s.fill) >= 7)
            }
        }
    }

    @Test
    fun badColorsHaveNoShades() {
        assertNull(labelShades("red"))
        assertNull(labelShades("#12345"))
    }

    @Test
    fun hslRoundTrip() {
        val c = hslToRgb(0.0, 1.0, 0.5)
        assertEquals(1.0, c[0], 1e-9)
        assertEquals(0.0, c[1], 1e-9)
    }
}
