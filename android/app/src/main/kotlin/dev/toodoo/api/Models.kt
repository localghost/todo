package dev.toodoo.api

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

// The JSON of the server's /api/v1 (spec 2026-10-21-android-app-design.md).

@Serializable
data class Label(
    val id: Long,
    val name: String,
    val color: String,
    val items: Int? = null, // only in the label list
)

@Serializable
data class Item(
    val id: Long,
    val text: String,
    val done: Boolean,
    @SerialName("due_at") val dueAt: String? = null, // RFC 3339, UTC
    @SerialName("due_all_day") val dueAllDay: Boolean = false,
    val overdue: Boolean = false,
    val labels: List<Label> = emptyList(),
)

@Serializable
data class ItemList(
    val items: List<Item>,
    val open: Int,
    val done: Int,
    @SerialName("hide_done") val hideDone: Boolean,
    val label: Label? = null, // the filter
)

@Serializable
data class User(
    val id: Long,
    val username: String,
    @SerialName("hide_done") val hideDone: Boolean = false,
)

@Serializable
data class LoginResult(
    val token: String,
    @SerialName("expires_at") val expiresAt: String,
    val user: User,
)

@Serializable
internal data class ErrorBody(val error: String = "", val message: String = "")
