package com.catalogizer.androidtv.ui

import android.content.Intent
import android.os.Bundle
import android.util.Log
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.Surface
import com.catalogizer.androidtv.CatalogizerTVApplication
import com.catalogizer.androidtv.ui.navigation.TVNavigation
import com.catalogizer.androidtv.ui.splash.SplashContent
import com.catalogizer.androidtv.ui.theme.CatalogizerTVTheme
import com.catalogizer.androidtv.ui.screens.search.SearchViewModel
import com.catalogizer.androidtv.ui.viewmodel.AuthViewModel
import com.catalogizer.androidtv.ui.viewmodel.HomeViewModel
import com.catalogizer.androidtv.ui.viewmodel.MainViewModel
import kotlinx.coroutines.launch

/**
 * Main entry point activity for the Catalogizer Android TV app.
 * Initializes ViewModels from [DependencyContainer], handles server URL from
 * intent extras (for ADB testing), and implements double-back-press to exit.
 */
class MainActivity : ComponentActivity() {

    private lateinit var authViewModel: AuthViewModel
    private lateinit var mainViewModel: MainViewModel
    private lateinit var homeViewModel: HomeViewModel
    private lateinit var searchViewModel: SearchViewModel
    private var backPressedOnce = false

    // Prevent accidental exit on Android TV — intercept finish() call
    // Back presses flow normally through Compose navigation; only when navigation
    // exhausts its stack does the system call finish(). We guard that final exit.
    override fun finish() {
        if (backPressedOnce) {
            super.finish()
            return
        }
        backPressedOnce = true
        Toast.makeText(this, "Press back again to exit", Toast.LENGTH_SHORT).show()
        window.decorView.postDelayed({ backPressedOnce = false }, 2000)
    }

    @OptIn(ExperimentalTvMaterial3Api::class)
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // CRITICAL: Get dependencyContainer first
        val dependencyContainer = (application as CatalogizerTVApplication).dependencyContainer
        
        // Handle server URL from intent extra (for ADB testing / HelixQA)
        // MUST be done BEFORE creating ViewModels to ensure correct API base URL!
        intent.getStringExtra("server_url")?.let { url ->
            if (url.isNotBlank()) {
                // Switch server synchronously on main thread - this just updates the URL string
                // The actual API client will be created lazily when first accessed
                try {
                    dependencyContainer.switchServer(url)
                    // Save to settings in background
                    kotlinx.coroutines.CoroutineScope(kotlinx.coroutines.Dispatchers.IO).launch {
                        dependencyContainer.settingsRepository.updateServerUrl(url)
                    }
                    Log.i("MainActivity", "Server URL set from intent: $url")
                } catch (e: Exception) {
                    Log.w("MainActivity", "Failed to set server URL: ${e.message}")
                }
            }
        }
        
        // NOW initialize ViewModels - they will use the correct server URL
        authViewModel = dependencyContainer.createAuthViewModel()
        mainViewModel = dependencyContainer.createMainViewModel()
        homeViewModel = dependencyContainer.createHomeViewModel()
        searchViewModel = dependencyContainer.createSearchViewModel()

        // On cold start, hydrate the auth state from disk BEFORE
        // deciding whether to show Login or Home. Without this the
        // user sees a flash of the login screen on every launch
        // even though their token is valid. hydrateFromDisk is
        // fast (<50ms) because TokenStore dispatches the
        // SharedPreferences read to Dispatchers.IO.
        kotlinx.coroutines.CoroutineScope(kotlinx.coroutines.Dispatchers.IO).launch {
            try {
                dependencyContainer.authRepository.hydrateFromDisk()
                android.util.Log.i("MainActivity", "Auth hydrated from disk")
            } catch (e: Exception) {
                android.util.Log.w("MainActivity", "Auth hydrate failed: ${e.message}")
            }
        }

        // Auto-login via intent extras (for ADB testing / HelixQA).
        // Usage: adb shell am start -n ... --es qa_username admin --es qa_password "$ADMIN_PASSWORD"
        // The login gate ensures the splash screen stays visible until login
        // completes, preventing the empty library race condition.
        val qaUser = intent.getStringExtra("qa_username")
        val qaPass = intent.getStringExtra("qa_password")
        if (!qaUser.isNullOrBlank() && !qaPass.isNullOrBlank()) {
            val loginGate = kotlinx.coroutines.CompletableDeferred<Unit>()
            mainViewModel.setQaLoginGate(loginGate)
            kotlinx.coroutines.CoroutineScope(kotlinx.coroutines.Dispatchers.IO).launch {
                try {
                    dependencyContainer.authRepository.login(qaUser, qaPass)
                    android.util.Log.i("MainActivity", "QA auto-login successful")
                } catch (e: Exception) {
                    android.util.Log.w("MainActivity", "QA auto-login failed: ${e.message}")
                } finally {
                    mainViewModel.completeQaLogin()
                }
            }
        }

        // Extract deep link from ChannelDeepLinkActivity
        processDeepLinkIntent(intent)

        setContent {
            CatalogizerTVTheme {
                Surface(
                    modifier = Modifier.fillMaxSize()
                ) {
                    CatalogizerTVApp(
                        authViewModel = authViewModel,
                        mainViewModel = mainViewModel,
                        homeViewModel = homeViewModel,
                        searchViewModel = searchViewModel
                    )
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent?) {
        super.onNewIntent(intent)
        processDeepLinkIntent(intent)
    }

    private fun processDeepLinkIntent(intent: Intent?) {
        val deepLinkMediaId = intent?.getLongExtra("deep_link_media_id", -1L) ?: -1L
        val deepLinkAction = intent?.getStringExtra("deep_link_action")
        val browseCategory = intent?.getStringExtra("deep_link_browse_category")
        val isHome = intent?.getBooleanExtra("deep_link_is_home", false) == true

        when {
            browseCategory != null -> {
                mainViewModel.setBrowseCategory(browseCategory)
                Log.d("MainActivity", "Deep link processed: browseCategory=$browseCategory")
            }
            isHome -> {
                mainViewModel.setBrowseCategory(null)
                mainViewModel.consumeDeepLink()
                Log.d("MainActivity", "Deep link processed: home")
            }
            deepLinkMediaId > 0 -> {
                mainViewModel.setDeepLink(deepLinkMediaId, deepLinkAction)
                Log.d("MainActivity", "Deep link processed: mediaId=$deepLinkMediaId, action=$deepLinkAction")
            }
        }
    }
}

/**
 * Root composable for the TV app managing the splash-to-navigation transition
 * based on authentication state and app initialization progress.
 */
@Composable
fun CatalogizerTVApp(
    authViewModel: AuthViewModel,
    mainViewModel: MainViewModel,
    homeViewModel: HomeViewModel,
    searchViewModel: SearchViewModel
) {
    val authState by authViewModel.authState.collectAsStateWithLifecycle()
    val isLoading by mainViewModel.isLoading.collectAsStateWithLifecycle()
    val deepLinkMediaId by mainViewModel.deepLinkMediaId.collectAsStateWithLifecycle()
    val deepLinkAction by mainViewModel.deepLinkAction.collectAsStateWithLifecycle()
    val browseCategory by mainViewModel.browseCategory.collectAsStateWithLifecycle()

    LaunchedEffect(Unit) {
        mainViewModel.initializeApp()
    }

    var splashComplete by remember { mutableStateOf(false) }

    if (!splashComplete) {
        SplashContent(
            isAppReady = !isLoading,
            onSplashComplete = { splashComplete = true }
        )
    } else {
        TVNavigation(
            isAuthenticated = authState.isAuthenticated,
            authViewModel = authViewModel,
            homeViewModel = homeViewModel,
            searchViewModel = searchViewModel,
            mainViewModel = mainViewModel,
            deepLinkMediaId = deepLinkMediaId,
            deepLinkAction = deepLinkAction,
            browseCategory = browseCategory
        )
    }
}