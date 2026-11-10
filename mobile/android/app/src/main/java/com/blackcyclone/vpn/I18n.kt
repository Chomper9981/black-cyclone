package com.blackcyclone.vpn

import android.content.res.Resources
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import rcvcore.Rcvcore

/**
 * The app's language. Translations live next to the code that uses them, as in the Go
 * code (internal/i18n): tr("tiếng Việt", "English").
 */
object Lang {
    const val AUTO = "auto"
    const val VI = "vi"
    const val EN = "en"

    /** The user's choice: AUTO (follow the system), VI or EN. */
    var setting by mutableStateOf(AUTO)
        private set

    /** The language in use, VI or EN. Compose state: composables calling tr() follow it. */
    var code by mutableStateOf(VI)
        private set

    /** Loads the saved choice; call once after Prefs.init. */
    fun init() {
        setting = Prefs.language.takeIf { it == VI || it == EN } ?: AUTO
        apply()
    }

    fun set(setting: String) {
        Prefs.language = setting
        this.setting = setting
        apply()
    }

    /** Re-resolves AUTO after the system language changed. */
    fun refresh() = apply()

    private fun apply() {
        code = when (setting) {
            VI, EN -> setting
            // Vietnamese systems get Vietnamese, everyone else English.
            else -> if (Resources.getSystem().configuration.locales[0].language == VI) VI else EN
        }
        // The Go core cannot see Android's locale: always tell it the resolved language.
        Rcvcore.setLanguage(code)
    }
}

/** Returns vi or en according to the current language. */
fun tr(vi: String, en: String): String = if (Lang.code == Lang.EN) en else vi
