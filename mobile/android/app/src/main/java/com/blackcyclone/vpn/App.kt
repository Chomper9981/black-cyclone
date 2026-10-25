package com.blackcyclone.vpn

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager

class App : Application() {
    override fun onCreate() {
        super.onCreate()
        Prefs.init(this)
        val channel = NotificationChannel(
            CHANNEL_VPN, getString(R.string.channel_vpn), NotificationManager.IMPORTANCE_LOW,
        ).apply { setShowBadge(false) }
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    companion object {
        const val CHANNEL_VPN = "vpn"
    }
}
