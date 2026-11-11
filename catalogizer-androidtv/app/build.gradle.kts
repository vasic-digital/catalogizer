import com.google.firebase.perf.plugin.FirebasePerfExtension
import java.util.Properties

// Disable JDK image transform at project level to avoid jlink issues
project.ext.set("android.useNewJdkImageTransform", false)
project.ext.set("android.experimental.jdkImageTransform", false)
project.ext.set("android.enableNewJdkImageTransform", false)
project.ext.set("android.experimental.useNewJdkImageTransform", false)

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("kotlin-kapt")
    id("kotlin-parcelize")
    id("org.jetbrains.kotlin.plugin.serialization")
    id("jacoco")
    id("io.github.takahirom.roborazzi")
    // Firebase plugins
    id("com.google.gms.google-services")
    id("com.google.firebase.crashlytics")
    id("com.google.firebase.firebase-perf")
}

android {
    namespace = "com.catalogizer.androidtv"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.catalogizer.androidtv"
        minSdk = 26
        targetSdk = 34
        versionCode = 9
        versionName = "2.4.1"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        vectorDrawables {
            useSupportLibrary = true
        }
    }

    signingConfigs {
        create("release") {
            val signingPropsFile = file("${rootProject.projectDir}/../docker/signing/signing.properties")
            if (signingPropsFile.exists()) {
                val props = Properties()
                props.load(signingPropsFile.inputStream())
                storeFile = file(props.getProperty("storeFile"))
                storePassword = props.getProperty("storePassword")
                keyAlias = props.getProperty("keyAlias")
                keyPassword = props.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        debug {
            isDebuggable = true
            buildConfigField("String", "API_BASE_URL", "\"\"")
            buildConfigField("String", "WS_URL", "\"\"")
            // Disable Firebase Performance bytecode instrumentation for debug only.
            // The perf plugin wraps HttpURLConnection/OkHttp, which forces
            // com.google.firebase.perf.config.RemoteConfigManager to class-init in the
            // pure-JVM unit-test environment (no Android/Firebase context) and throws
            // ExceptionInInitializerError -> NoClassDefFoundError, poisoning the JVM fork.
            // Release perf monitoring is unaffected (instrumentation stays on for release).
            configure<FirebasePerfExtension> {
                setInstrumentationEnabled(false)
            }
        }
        release {
            isMinifyEnabled = true
            signingConfig = signingConfigs.getByName("release")
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
            buildConfigField("String", "API_BASE_URL", "\"https://catalogizer.dev\"")
            buildConfigField("String", "WS_URL", "\"wss://catalogizer.dev/ws\"")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    testOptions {
        unitTests.isReturnDefaultValues = true
        // REQUIRED by Roborazzi — Robolectric must inflate real resources to render
        // pixels; without this the screenshot renders blank/throws (cited research).
        unitTests.isIncludeAndroidResources = true
        unitTests.all {
            it.useJUnit()
            it.jvmArgs("-Xmx4096m", "-XX:MaxMetaspaceSize=1024m", "-Djava.security.manager=allow")
            it.maxParallelForks = 1
            it.forkEvery = 5
        }
    }

    tasks.withType<org.jetbrains.kotlin.gradle.tasks.KotlinCompile> {
        kotlinOptions {
            jvmTarget = "17"
            freeCompilerArgs = freeCompilerArgs + listOf(
                "-Xjvm-default=all"
            )
        }
    }

    // JDK 21 requires --add-opens for kapt (Room compiler)
    kapt {
        javacOptions {
            option("--add-opens", "jdk.compiler/com.sun.tools.javac.api=ALL-UNNAMED")
            option("--add-opens", "jdk.compiler/com.sun.tools.javac.code=ALL-UNNAMED")
            option("--add-opens", "jdk.compiler/com.sun.tools.javac.comp=ALL-UNNAMED")
            option("--add-opens", "jdk.compiler/com.sun.tools.javac.main=ALL-UNNAMED")
            option("--add-opens", "jdk.compiler/com.sun.tools.javac.processing=ALL-UNNAMED")
            option("--add-opens", "jdk.compiler/com.sun.tools.javac.tree=ALL-UNNAMED")
            option("--add-opens", "jdk.compiler/com.sun.tools.javac.util=ALL-UNNAMED")
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    composeOptions {
        kotlinCompilerExtensionVersion = "1.5.8"
    }

    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }
}

dependencies {
    // Core Android
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.7.0")
    implementation("androidx.activity:activity-compose:1.8.2")

    // Compose BOM — pinned to 2024.06.00 (Compose 1.6.8) for binary compatibility
    // with tv-foundation:1.0.0-alpha11 and tv-material:1.0.0 (both compiled against 1.6.8).
    // BOM 2024.01.00 (Compose 1.6.0) caused NoSuchMethodError on KeyframesSpec.at() at runtime
    // because tv-foundation alpha10 was compiled against Compose 1.4.x animation-core.
    implementation(platform("androidx.compose:compose-bom:2024.06.00"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-graphics")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")

    // TV-specific Compose — aligned with Compose 1.6.8 (BOM 2024.06.00)
    implementation("androidx.tv:tv-foundation:1.0.0-alpha11")
    implementation("androidx.tv:tv-material:1.0.0")

    // Navigation
    implementation("androidx.navigation:navigation-compose:2.7.5")

    // Lifecycle
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.7.0")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.7.0")

    // Networking
    implementation("com.squareup.retrofit2:retrofit:2.9.0")
    implementation("com.jakewharton.retrofit:retrofit2-kotlinx-serialization-converter:1.0.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("com.squareup.okhttp3:logging-interceptor:4.12.0")

    // WebSocket
    implementation("org.java-websocket:Java-WebSocket:1.5.4")

    // Image Loading
    implementation("io.coil-kt:coil-compose:2.5.0")
    implementation("io.coil-kt:coil-svg:2.5.0")

    // JSON Serialization
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.6.2")

    // Datastore
    implementation("androidx.datastore:datastore-preferences:1.0.0")

    // Security: EncryptedSharedPreferences for persisting the JWT
    // session across cold start / force-stop. Uses Android keystore-
    // backed AES-256-GCM; no plaintext token ever touches disk.
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    // Room Database
    implementation("androidx.room:room-runtime:2.6.1")
    implementation("androidx.room:room-ktx:2.6.1")
    kapt("androidx.room:room-compiler:2.6.1")

    // Paging
    implementation("androidx.paging:paging-runtime:3.2.1")
    implementation("androidx.paging:paging-compose:3.2.1")

    // Leanback for TV
    implementation("androidx.leanback:leanback:1.0.0")
    implementation("androidx.leanback:leanback-preference:1.0.0")

    // Media Player - VLC Integration
    // Note: Using VLC instead of ExoPlayer for broader codec support
    // libVLC: upgraded from 3.6.0 to 3.6.2 (latest stable on
    // Maven Central, April 2025) to pick up stability fixes for
    // native Media.nativeNewFromLocation on armeabi-v7a that
    // surfaced as a SIGSEGV on Mi Box 4 Android 9.
    implementation("org.videolan.android:libvlc-all:3.6.2")
    
    // Keep ExoPlayer as fallback for specific use cases
    implementation("androidx.media3:media3-exoplayer:1.2.0")
    implementation("androidx.media3:media3-datasource:1.2.0")
    implementation("androidx.media3:media3-ui:1.2.0")
    implementation("androidx.media3:media3-common:1.2.0")
    implementation("androidx.media3:media3-session:1.2.0")

    // TV Input Framework
    implementation("androidx.tvprovider:tvprovider:1.0.0")

    // WorkManager for periodic channel sync
    implementation("androidx.work:work-runtime-ktx:2.9.0")
    // WorkManager testing
    testImplementation("androidx.work:work-testing:2.9.0")

    // Firebase BoM — Analytics, Crashlytics, Performance, App Distribution
    // Pinned to 32.7.0 for Kotlin 1.9.x compatibility (play-services-measurement
    // in 33.x ships Kotlin 2.1 metadata that Kotlin 1.9.22 cannot parse).
    implementation(platform("com.google.firebase:firebase-bom:32.7.0"))
    implementation("com.google.firebase:firebase-analytics")
    implementation("com.google.firebase:firebase-crashlytics")
    implementation("com.google.firebase:firebase-perf")

    // Testing
    testImplementation("junit:junit:4.13.2")
    testImplementation("org.mockito:mockito-core:5.8.0")
    testImplementation("org.mockito.kotlin:mockito-kotlin:5.2.1")
    testImplementation("io.mockk:mockk:1.13.8")
    testImplementation("androidx.arch.core:core-testing:2.2.0")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.7.3")
    testImplementation("androidx.room:room-testing:2.6.1")
    testImplementation("org.robolectric:robolectric:4.11.1")
    testImplementation("androidx.test:core:1.5.0")
    testImplementation("com.squareup.okhttp3:mockwebserver:4.12.0")
    // §11.4.107/.158/.159 host-side visual-proof harness (renders Compose-for-TV → PNG,
    // no device). compose-bom platform pins ui-test-* to the SAME Compose 1.6.8 the app
    // compiles against (cited research: mismatched Compose → NoSuchMethodError on tv-foundation).
    testImplementation(platform("androidx.compose:compose-bom:2024.06.00"))
    testImplementation("androidx.compose.ui:ui-test-junit4")
    testImplementation("androidx.compose.ui:ui-test-manifest")
    testImplementation("io.github.takahirom.roborazzi:roborazzi:1.13.0")
    testImplementation("io.github.takahirom.roborazzi:roborazzi-compose:1.13.0")
    testImplementation("io.github.takahirom.roborazzi:roborazzi-junit-rule:1.13.0")

    androidTestImplementation("androidx.test.ext:junit:1.1.5")
    androidTestImplementation("androidx.test.espresso:espresso-core:3.5.1")
    androidTestImplementation("androidx.test:core:1.5.0")
    androidTestImplementation("androidx.test:runner:1.5.2")
    androidTestImplementation("androidx.test:rules:1.5.0")
    androidTestImplementation("org.mockito:mockito-android:5.8.0")
    androidTestImplementation(platform("androidx.compose:compose-bom:2024.06.00"))
    androidTestImplementation("androidx.compose.ui:ui-test-junit4")
    debugImplementation("androidx.compose.ui:ui-tooling")
    debugImplementation("androidx.compose.ui:ui-test-manifest")
}

tasks.register<JacocoReport>("jacocoTestReport") {
    dependsOn("testDebugUnitTest")
    reports {
        xml.required.set(true)
        html.required.set(true)
        csv.required.set(false)
    }
    val fileFilter = listOf(
        "**/R.class", "**/R\$*.class", "**/BuildConfig.*",
        "**/Manifest*.*", "**/*Test*.*", "**/com/catalogizer/androidtv/databinding/**"
    )
    val debugTree = fileTree(layout.buildDirectory.dir("tmp/kotlin-classes/debug")) { exclude(fileFilter) }
    val mainSrc = "${project.projectDir}/src/main/java"
    sourceDirectories.setFrom(files(mainSrc))
    classDirectories.setFrom(files(debugTree))
    executionData.setFrom(fileTree(layout.buildDirectory) { include("jacoco/testDebugUnitTest.exec") })
}