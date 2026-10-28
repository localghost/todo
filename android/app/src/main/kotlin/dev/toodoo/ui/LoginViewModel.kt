package dev.toodoo.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dev.toodoo.TooDooApp
import dev.toodoo.api.ApiClient
import dev.toodoo.api.ApiException
import dev.toodoo.api.OfflineException
import dev.toodoo.data.Session
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class LoginState(
    val username: String = "",
    val password: String = "",
    val server: String = "",
    val busy: Boolean = false,
    val error: String? = null,
)

/** What the login screen needs; the screenshot tests give a fake. */
interface LoginActions {
    val state: StateFlow<LoginState>
    fun setUsername(v: String)
    fun setPassword(v: String)
    fun setServer(v: String)
    fun logIn()
}

class LoginViewModel(private val app: TooDooApp) : ViewModel(), LoginActions {
    private val _state = MutableStateFlow(LoginState())
    override val state: StateFlow<LoginState> = _state

    init {
        viewModelScope.launch {
            val server = app.sessions.lastServer.first()
            val username = app.sessions.lastUsername.first()
            _state.update { it.copy(server = it.server.ifEmpty { server }, username = it.username.ifEmpty { username }) }
        }
    }

    override fun setUsername(v: String) = _state.update { it.copy(username = v, error = null) }
    override fun setPassword(v: String) = _state.update { it.copy(password = v, error = null) }
    override fun setServer(v: String) = _state.update { it.copy(server = v, error = null) }

    override fun logIn() {
        val s = _state.value
        if (s.busy) return
        if (s.username.isBlank() || s.password.isEmpty()) {
            _state.update { it.copy(error = "Please type your username and password.") }
            return
        }
        val api = app.loginApi(s.server)
        if (api == null) {
            _state.update { it.copy(error = "The server address must start with https://.") }
            return
        }
        _state.update { it.copy(busy = true, error = null) }
        viewModelScope.launch {
            try {
                val res = api.login(s.username.trim(), s.password)
                // The session changes the screen to the list.
                app.sessions.save(Session(ApiClient.checkServer(s.server).toString(), res.token, res.user.username))
                _state.update { it.copy(password = "", busy = false) }
            } catch (e: ApiException) {
                _state.update { it.copy(busy = false, error = e.message) }
            } catch (e: OfflineException) {
                _state.update { it.copy(busy = false, error = "Cannot reach the server. Check the connection and the address.") }
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(busy = false, error = "That did not work. Is the server address right?") }
            }
        }
    }
}
