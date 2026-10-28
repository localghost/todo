package com.codeleet.toodoo.ui

import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min
import kotlin.math.pow

/** The colors of a label chip, as 0xRRGGBB. */
data class Shades(val fill: Int, val border: Int, val text: Int)

/**
 * Returns the chip shades for a label color "#rrggbb", or null for another
 * string. The same rule as the web's static/chips.js: a very light fill
 * (lightness 96 %), a light border (87 %), and text in the same hue, dark
 * enough for 7:1 contrast on the fill. In [dark] mode the fill is dark (16 %),
 * the border 30 %, and the text light enough for 7:1.
 */
fun labelShades(hex: String, dark: Boolean = false): Shades? {
    val (h, s, _) = hexToHsl(hex) ?: return null
    val fillL = if (dark) 0.16 else 0.96
    val fill = hslToRgb(h, s, fillL)
    val lFill = luminance(fill)
    val ts = min(s, 0.8)
    var text = hslToRgb(h, ts, if (dark) 1.0 else 0.05)
    val steps = if (dark) (60..100) else (40 downTo 6)
    for (l in steps) {
        val c = hslToRgb(h, ts, l / 100.0)
        val lc = luminance(c)
        val ratio = if (dark) (lc + 0.05) / (lFill + 0.05) else (lFill + 0.05) / (lc + 0.05)
        if (ratio >= 7.05) { // a little over 7, so it still holds after rounding to 8 bits
            text = c
            break
        }
    }
    return Shades(toInt(fill), toInt(hslToRgb(h, s, if (dark) 0.30 else 0.87)), toInt(text))
}

/** Returns hue in degrees, saturation 0..1, lightness 0..1. */
internal fun hexToHsl(hex: String): Triple<Double, Double, Double>? {
    if (!Regex("^#[0-9a-fA-F]{6}$").matches(hex)) return null
    val (r, g, b) = listOf(1, 3, 5).map { hex.substring(it, it + 2).toInt(16) / 255.0 }
    val mx = max(r, max(g, b))
    val mn = min(r, min(g, b))
    val l = (mx + mn) / 2
    val d = mx - mn
    if (d == 0.0) return Triple(0.0, 0.0, l)
    val s = d / (1 - abs(2 * l - 1))
    val h = when (mx) {
        r -> ((g - b) / d) % 6
        g -> (b - r) / d + 2
        else -> (r - g) / d + 4
    }
    return Triple((h * 60 + 360) % 360, s, l)
}

internal fun hslToRgb(h: Double, s: Double, l: Double): DoubleArray {
    val a = s * min(l, 1 - l)
    fun f(n: Int): Double {
        val k = (n + h / 30) % 12
        return l - a * max(-1.0, min(min(k - 3, 9 - k), 1.0))
    }
    return doubleArrayOf(f(0), f(8), f(4))
}

internal fun luminance(rgb: DoubleArray): Double {
    val (r, g, b) = rgb.map { if (it <= 0.04045) it / 12.92 else ((it + 0.055) / 1.055).pow(2.4) }
    return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

private fun toInt(rgb: DoubleArray): Int =
    rgb.fold(0) { acc, v -> (acc shl 8) or Math.round(v * 255).toInt().coerceIn(0, 255) }

/** Returns "#rrggbb" for hue in degrees, saturation and lightness 0..1. */
fun hslToHex(h: Double, s: Double, l: Double): String = "#%06x".format(toInt(hslToRgb(h, s, l)))
