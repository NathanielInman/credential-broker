package com.nathanielinman.broker

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.compose.runtime.getValue
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning

class MainActivity : ComponentActivity() {
    private val model: BrokerModel by viewModels()
    private lateinit var reader: KeyReader

    /** Notifications are asked for when enrolling starts; scanning goes on whatever the answer. */
    private val askNotifications = registerForActivityResult(ActivityResultContracts.RequestPermission()) { scanNow() }

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        reader = KeyReader(this)
        model.reader = reader
        if (savedInstanceState == null) model.refresh(approve = intent.getBooleanExtra(Notifications.EXTRA_APPROVE, false))

        val actions = Actions(
            scan = ::scan,
            paste = model::enroll,
            notificationSettings = {
                startActivity(Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, packageName))
            },
            close = ::finish,
        )
        setContent {
            BrokerTheme {
                val screen by model.screen.collectAsStateWithLifecycle()
                val autoClose by model.autoClose.collectAsStateWithLifecycle()
                val enrollment = Store.enrollment(this)
                BrokerScreen(
                    screen = screen,
                    autoClose = autoClose,
                    model = model,
                    actions = actions,
                    origin = enrollment?.origin,
                    notifying = Store.notifying(this) && Notifications.enabled(this),
                    stale = Store.fcmStale(this),
                )
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        model.refresh(approve = intent.getBooleanExtra(Notifications.EXTRA_APPROVE, false))
    }

    override fun onResume() {
        super.onResume()
        model.resumed()
    }

    override fun onDestroy() {
        if (model.reader === reader) model.reader = null
        super.onDestroy()
    }

    private fun scan() {
        if (Build.VERSION.SDK_INT >= 33 &&
            ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            askNotifications.launch(Manifest.permission.POST_NOTIFICATIONS)
        } else {
            scanNow()
        }
    }

    private fun scanNow() {
        val options = GmsBarcodeScannerOptions.Builder()
            .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
            .enableAutoZoom()
            .build()
        GmsBarcodeScanning.getClient(this, options).startScan()
            .addOnSuccessListener { code -> code.rawValue?.let(model::enroll) }
            .addOnFailureListener { model.setupError("Couldn't scan: ${it.message}") }
    }
}
