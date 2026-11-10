package com.blackcyclone.vpn

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.res.Configuration

class App : Application() {
    override fun onCreate() {
        super.onCreate()
        Prefs.init(this)
        Lang.init()
        val channel = NotificationChannel(
            CHANNEL_VPN, getString(R.string.channel_vpn), NotificationManager.IMPORTANCE_LOW,
        ).apply { setShowBadge(false) }
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        Lang.refresh() // "auto" follows the system language
    }

    companion object {
        const val CHANNEL_VPN = "vpn"
    }
}
