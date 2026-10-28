package com.codeleet.toodoo.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// The web app's colors (internal/web/static/app.css), and a dark version.
private val Light = lightColorScheme(
    primary = Color(0xFF2450C7),
    onPrimary = Color.White,
    primaryContainer = Color(0xFFE4EAF9),
    onPrimaryContainer = Color(0xFF1A3C99),
    background = Color(0xFFF4F3EF),
    onBackground = Color(0xFF1F1E1B),
    surface = Color.White,
    onSurface = Color(0xFF1F1E1B),
    surfaceVariant = Color(0xFFF4F3EF),
    onSurfaceVariant = Color(0xFF5C5A55),
    surfaceContainer = Color.White,
    surfaceContainerLow = Color.White,
    surfaceContainerHigh = Color.White,
    outline = Color(0xFFB9B6AD),
    outlineVariant = Color(0xFFE3E1DA),
    error = Color(0xFFB3261E),
    onError = Color.White,
    inverseSurface = Color(0xFF2F2E2B),
    inverseOnSurface = Color(0xFFF4F3EF),
    inversePrimary = Color(0xFFA9BEF2),
)

private val Dark = darkColorScheme(
    primary = Color(0xFFA9BEF2),
    onPrimary = Color(0xFF0E2466),
    primaryContainer = Color(0xFF233A7A),
    onPrimaryContainer = Color(0xFFDCE4FA),
    background = Color(0xFF1C1B19),
    onBackground = Color(0xFFECEAE4),
    surface = Color(0xFF262522),
    onSurface = Color(0xFFECEAE4),
    surfaceVariant = Color(0xFF1C1B19),
    onSurfaceVariant = Color(0xFFB4B1A9),
    surfaceContainer = Color(0xFF262522),
    surfaceContainerLow = Color(0xFF262522),
    surfaceContainerHigh = Color(0xFF2F2E2B),
    outline = Color(0xFF7A776F),
    outlineVariant = Color(0xFF3A3935),
    error = Color(0xFFF2B8B5),
    onError = Color(0xFF601410),
    inverseSurface = Color(0xFFECEAE4),
    inverseOnSurface = Color(0xFF1F1E1B),
    inversePrimary = Color(0xFF2450C7),
)

@Composable
fun TooDooTheme(dark: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = if (dark) Dark else Light, content = content)
}
