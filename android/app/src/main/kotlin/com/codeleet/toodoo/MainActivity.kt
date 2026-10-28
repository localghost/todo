package com.codeleet.toodoo

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.codeleet.toodoo.data.Session
import com.codeleet.toodoo.ui.ListScreen
import com.codeleet.toodoo.ui.ListViewModel
import com.codeleet.toodoo.ui.LoginScreen
import com.codeleet.toodoo.ui.LoginViewModel
import com.codeleet.toodoo.ui.TooDooTheme
import kotlinx.coroutines.flow.map

/** Who is logged in, once the session store has been read. */
private sealed interface Start {
    data object Reading : Start
    data object LoggedOut : Start
    data class LoggedIn(val session: Session) : Start
}

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val app = application as TooDooApp
        setContent {
            TooDooTheme {
                val flow = remember { app.sessions.session.map { s -> s?.let { Start.LoggedIn(it) } ?: Start.LoggedOut } }
                val start by flow.collectAsStateWithLifecycle(initialValue = Start.Reading)
                Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
                    when (val s = start) {
                        Start.Reading -> {}
                        Start.LoggedOut -> LoginScreen(viewModel { LoginViewModel(app) })
                        // A new log-in gets a new list (the key is the token).
                        is Start.LoggedIn -> ListScreen(viewModel(key = s.session.token) { ListViewModel(app, s.session) })
                    }
                }
            }
        }
    }
}
