package com.blackcyclone.vpn.ui

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

val BrandRed = Color(0xFFE00000)
val OkGreen = Color(0xFF34C77B)
val WarnAmber = Color(0xFFF0B429)

private val colors = darkColorScheme(
    primary = BrandRed,
    onPrimary = Color.White,
    secondary = Color(0xFFFF8A80),
    background = Color(0xFF16181D),
    onBackground = Color(0xFFE8E9EC),
    surface = Color(0xFF16181D),
    onSurface = Color(0xFFE8E9EC),
    surfaceVariant = Color(0xFF22252C),
    onSurfaceVariant = Color(0xFFAEB2BB),
    surfaceContainer = Color(0xFF1C1F25),
    surfaceContainerHigh = Color(0xFF22252C),
    surfaceContainerHighest = Color(0xFF2A2E36),
    outline = Color(0xFF3A3F49),
    error = Color(0xFFFF6B6B),
)

@Composable
fun RcvTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = colors, content = content)
}
