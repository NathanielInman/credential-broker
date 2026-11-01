package com.nathanielinman.broker

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

/** The desktop window's colors, so a request looks the same on both. */
@Immutable
data class Palette(
    val bg: Color,
    val fg: Color,
    val muted: Color,
    val faint: Color,
    val panel: Color,
    val line: Color,
    val button: Color,
    val primary: Color,
    val claimBg: Color,
    val claimLine: Color,
    val claimFg: Color,
    val touchBg: Color,
    val touchLine: Color,
    val ok: Color,
    val warnBg: Color,
    val warnLine: Color,
    val warnFg: Color,
)

private val Dark = Palette(
    bg = Color(0xFF1B1C21), fg = Color(0xFFE9E9EE), muted = Color(0xFF9A9CA8), faint = Color(0xFF7D808C),
    panel = Color(0xFF24262D), line = Color(0xFF30323B), button = Color(0xFF33353E), primary = Color(0xFF4F7EF7),
    claimBg = Color(0xFF2A261E), claimLine = Color(0xFFC9A24A), claimFg = Color(0xFFD9B45C),
    touchBg = Color(0xFF1D2A22), touchLine = Color(0xFF2F4C37), ok = Color(0xFF5FD38A),
    warnBg = Color(0xFF2D2020), warnLine = Color(0xFFA84A4A), warnFg = Color(0xFFEF8B8B),
)

private val Light = Palette(
    bg = Color(0xFFFBFBFC), fg = Color(0xFF1D1E22), muted = Color(0xFF5F6270), faint = Color(0xFF80838F),
    panel = Color(0xFFF0F1F4), line = Color(0xFFDCDEE4), button = Color(0xFFE6E7EB), primary = Color(0xFF4F7EF7),
    claimBg = Color(0xFFFBF4E2), claimLine = Color(0xFFC9A24A), claimFg = Color(0xFF8A6716),
    touchBg = Color(0xFFEAF7EE), touchLine = Color(0xFFB9E2C6), ok = Color(0xFF1F9D55),
    warnBg = Color(0xFFFDEEEE), warnLine = Color(0xFFE3A1A1), warnFg = Color(0xFFB23B3B),
)

val LocalPalette = staticCompositionLocalOf { Dark }

@Composable
fun BrokerTheme(content: @Composable () -> Unit) {
    val dark = isSystemInDarkTheme()
    val p = if (dark) Dark else Light
    val scheme = if (dark) {
        darkColorScheme(
            primary = p.primary, onPrimary = Color.White, background = p.bg, onBackground = p.fg,
            surface = p.bg, onSurface = p.fg, surfaceVariant = p.panel, onSurfaceVariant = p.muted,
            surfaceContainer = p.panel, surfaceContainerHigh = p.button, outline = p.line, outlineVariant = p.line,
            secondaryContainer = p.button, onSecondaryContainer = p.fg, error = p.warnFg,
        )
    } else {
        lightColorScheme(
            primary = p.primary, onPrimary = Color.White, background = p.bg, onBackground = p.fg,
            surface = p.bg, onSurface = p.fg, surfaceVariant = p.panel, onSurfaceVariant = p.muted,
            surfaceContainer = p.panel, surfaceContainerHigh = p.button, outline = p.line, outlineVariant = p.line,
            secondaryContainer = p.button, onSecondaryContainer = p.fg, error = p.warnFg,
        )
    }
    CompositionLocalProvider(LocalPalette provides p) {
        MaterialTheme(colorScheme = scheme, content = content)
    }
}
