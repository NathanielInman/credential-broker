plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
}

// Builds are signed with a personal keystore (BROKER_ANDROID_KEYSTORE, by
// default ~/.android/broker-release.jks) when BROKER_ANDROID_KEYSTORE_PASSWORD
// is set, as `broker exec BROKER_ANDROID_KEYSTORE_PASSWORD -- ./gradlew ...`
// does, so one install can be upgraded by the next. Without it the SDK's
// debug key signs, and that install cannot be upgraded by a signed one
// without uninstalling (and enrolling again).
val keystore = file(System.getenv("BROKER_ANDROID_KEYSTORE") ?: "${System.getProperty("user.home")}/.android/broker-release.jks")
val keystorePassword = System.getenv("BROKER_ANDROID_KEYSTORE_PASSWORD")

android {
    namespace = "com.nathanielinman.broker"
    compileSdk = 36

    defaultConfig {
        applicationId = "com.nathanielinman.broker"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
    }

    signingConfigs {
        if (keystore.exists() && keystorePassword != null) {
            create("personal") {
                storeFile = keystore
                storePassword = keystorePassword
                keyAlias = "broker"
                keyPassword = keystorePassword
            }
        }
    }

    buildTypes {
        val personal = signingConfigs.findByName("personal")
        debug {
            if (personal != null) signingConfig = personal
        }
        release {
            isMinifyEnabled = false
            signingConfig = personal ?: signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        compose = true
    }

    packaging {
        resources.excludes += "/META-INF/{AL2.0,LGPL2.1}"
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.fragment.ktx)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.kotlinx.coroutines.play.services)
    implementation(platform(libs.firebase.bom))
    implementation(libs.firebase.messaging)
    implementation(libs.play.services.code.scanner)
    implementation(libs.yubikit.android)
    implementation(libs.yubikit.fido)
    debugImplementation(libs.androidx.compose.ui.tooling)
    testImplementation(libs.junit)
}
