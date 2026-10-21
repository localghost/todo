package dev.toodoo.data

import dev.toodoo.api.ItemList
import dev.toodoo.api.json
import java.io.File
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/**
 * The last list the app loaded, shown read-only when there is no network.
 * One file in the app's private storage; deleted on log-out.
 */
class ListCache(dir: File) {
    private val file = File(dir, "list.json")

    suspend fun load(): ItemList? = withContext(Dispatchers.IO) {
        runCatching { json.decodeFromString<ItemList>(file.readText()) }.getOrNull()
    }

    suspend fun save(list: ItemList) = withContext(Dispatchers.IO) {
        val tmp = File(file.parentFile, file.name + ".tmp")
        tmp.writeText(json.encodeToString(ItemList.serializer(), list))
        tmp.renameTo(file)
    }

    suspend fun clear() = withContext(Dispatchers.IO) {
        file.delete()
    }
}
