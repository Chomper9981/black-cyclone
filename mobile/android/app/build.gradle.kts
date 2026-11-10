import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.plugin.compose")
}

// Release signing from android/keystore.properties (storeFile, storePassword, keyAlias,
// keyPassword); without it release builds are signed with the debug key.
val keystoreFile = rootProject.file("keystore.properties")

// Copies the repo's license texts into the APK (assets/licenses/), shown under
// Cài đặt → Giới thiệu → Giấy phép, plus bin/geoip-vn.srs (from fetch-bins) when
// present; without it sing-box downloads the rule set on first connect.
// Regenerate licenses/go-android.txt with tools/licenses (see
// THIRD_PARTY_NOTICES.md) after changing the Go core's dependencies.
abstract class CopyAssets : DefaultTask() {
    @get:InputFiles
    abstract val licenses: ConfigurableFileCollection

    @get:InputFiles
    @get:Optional
    abstract val optional: ConfigurableFileCollection

    @get:OutputDirectory
    abstract val outputDir: DirectoryProperty

    @TaskAction
    fun copy() {
        val root = outputDir.get().asFile
        root.deleteRecursively()
        val out = root.resolve("licenses").apply { mkdirs() }
        licenses.forEach { f ->
            check(f.isFile) { "missing license file: $f" }
            f.copyTo(out.resolve(f.name), overwrite = true)
        }
        optional.filter { it.isFile }.forEach { it.copyTo(root.resolve(it.name), overwrite = true) }
    }
}

val repoRoot = rootProject.file("../..")
val copyAssets = tasks.register<CopyAssets>("copyAssets") {
    licenses.from(
        repoRoot.resolve("LICENSE"),
        repoRoot.resolve("THIRD_PARTY_NOTICES.md"),
        repoRoot.resolve("licenses/GPL-3.0.txt"),
        repoRoot.resolve("licenses/go-android.txt"),
        repoRoot.resolve("licenses/Apache-2.0.txt"),
    )
    optional.from(repoRoot.resolve("bin/geoip-vn.srs"))
}

androidComponents {
    onVariants { variant ->
        variant.sources.assets?.addGeneratedSourceDirectory(copyAssets, CopyAssets::outputDir)
    }
}

android {
    namespace = "com.blackcyclone.vpn"
    compileSdk {
        version = release(37) { minorApiLevel = 2 }
    }

    defaultConfig {
        applicationId = "com.blackcyclone.vpn"
        minSdk = 26
        targetSdk = 36
        versionCode = 5
        versionName = "1.3.0"
    }

    signingConfigs {
        if (keystoreFile.exists()) {
            val p = Properties().apply { keystoreFile.inputStream().use { load(it) } }
            create("release") {
                storeFile = rootProject.file(p.getProperty("storeFile"))
                storePassword = p.getProperty("storePassword")
                keyAlias = p.getProperty("keyAlias")
                keyPassword = p.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.findByName("release") ?: signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    // One APK per CPU type plus a universal one: the Go core (sing-box) is ~45 MB per
    // ABI, so most phones only need app-arm64-v8a-*.apk.
    splits {
        abi {
            isEnable = true
            reset()
            include("arm64-v8a", "armeabi-v7a", "x86_64")
            isUniversalApk = true
        }
    }

    packaging {
        // Compress the native core in the APK (about a third of its size): smaller
        // downloads, at the cost of Android extracting it once at install.
        jniLibs { useLegacyPackaging = true }
    }
}

dependencies {
    implementation(files("libs/rcvcore.aar"))

    implementation("androidx.core:core-ktx:1.19.1")
    implementation("androidx.activity:activity-compose:1.13.0")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.11.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.11.0")

    implementation(platform("androidx.compose:compose-bom:2026.09.00"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
}
