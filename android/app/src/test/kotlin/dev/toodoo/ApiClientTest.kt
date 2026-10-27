package dev.toodoo

import dev.toodoo.api.ApiClient
import dev.toodoo.api.ApiException
import kotlinx.coroutines.test.runTest
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test

class ApiClientTest {
    private val server = MockWebServer()

    @Before
    fun start() = server.start()

    @After
    fun stop() = server.shutdown()

    private fun client(token: String? = "tok") =
        ApiClient(OkHttpClient(), server.url("/").toString().replace("localhost", "127.0.0.1"), token)

    @Test
    fun itemsSendsTokenAndZoneAndParses() = runTest {
        // The shape the Go server writes (internal/web/api.go).
        server.enqueue(MockResponse().setBody("""{"items":[{"id":3,"text":"Pay rent","done":false,
            "due_at":"2026-10-22T12:30:00Z","due_all_day":false,"overdue":true,
            "labels":[{"id":1,"name":"home","color":"#38c75d"}],"created_at":"2026-10-01T10:00:00Z",
            "updated_at":"2026-10-01T10:00:00Z"}],"open":1,"done":0,"hide_done":false,"label":null}"""))
        val list = client().items()
        assertEquals(1, list.items.size)
        assertEquals("home", list.items[0].labels[0].name)
        assertTrue(list.items[0].overdue)
        val req = server.takeRequest()
        assertEquals("/api/v1/items", req.path)
        assertEquals("Bearer tok", req.getHeader("Authorization"))
        assertNotNull(req.getHeader("Time-Zone"))
    }

    @Test
    fun createLabelSendsTheName() = runTest {
        server.enqueue(MockResponse().setBody("""{"id":7,"name":"garden","color":"#e9bd16"}"""))
        val label = client().createLabel("garden")
        assertEquals(7L, label.id)
        assertEquals("garden", label.name)
        val req = server.takeRequest()
        assertEquals("POST", req.method)
        assertEquals("/api/v1/labels", req.path)
        assertEquals("""{"name":"garden"}""", req.body.readUtf8())
    }

    @Test
    fun errorsCarryTheServerMessage() = runTest {
        server.enqueue(MockResponse().setResponseCode(422).setBody("""{"error":"bad_text","message":"Please type some text first."}"""))
        try {
            client().addItem(" ")
            fail("no exception")
        } catch (e: ApiException) {
            assertEquals(422, e.status)
            assertEquals("Please type some text first.", e.message)
        }
        val req = server.takeRequest()
        assertEquals("POST", req.method)
        assertEquals("""{"text":" ","labels":[]}""", req.body.readUtf8())
    }

    @Test
    fun loggedOutIsRecognised() = runTest {
        server.enqueue(MockResponse().setResponseCode(401).setBody("""{"error":"logged_out","message":"Please log in again."}"""))
        try {
            client().items()
            fail("no exception")
        } catch (e: ApiException) {
            assertTrue(e.loggedOut)
        }
    }

    @Test
    fun deleteAnswers204() = runTest {
        server.enqueue(MockResponse().setResponseCode(204))
        client().deleteItem(7)
        assertEquals("DELETE", server.takeRequest().method)
    }

    @Test
    fun itemSheetCalls() = runTest {
        val item = """{"id":7,"text":"a","done":false,"labels":[]}"""
        repeat(4) { server.enqueue(MockResponse().setBody(item)) }
        server.enqueue(MockResponse().setResponseCode(204))
        server.enqueue(MockResponse().setBody("""[{"id":1,"name":"home","color":"#38c75d","items":2}]"""))
        val c = client()
        c.setDue(7, "2026-10-22", "1430")
        c.postpone(7, 10)
        c.addLabel(7, "work")
        c.removeLabel(7, 3)
        c.move(7, 0)
        assertEquals(2, c.labels()[0].items)
        val sent = List(6) { server.takeRequest() }.map { "${it.method} ${it.path} ${it.body.readUtf8()}" }
        assertEquals(
            listOf(
                """PUT /api/v1/items/7/due {"date":"2026-10-22","time":"1430"}""",
                """POST /api/v1/items/7/postpone {"minutes":10}""",
                """POST /api/v1/items/7/labels {"name":"work"}""",
                "DELETE /api/v1/items/7/labels/3 ",
                """POST /api/v1/items/7/move {"after":0}""",
                "GET /api/v1/labels ",
            ),
            sent,
        )
    }

    @Test
    fun serverAddresses() {
        assertEquals("https://toodoo.fly.dev/", ApiClient.checkServer("toodoo.fly.dev").toString())
        assertEquals("https://toodoo.fly.dev/", ApiClient.checkServer(" https://toodoo.fly.dev/ ").toString())
        assertNull(ApiClient.checkServer("http://toodoo.fly.dev"))
        assertNotNull(ApiClient.checkServer("http://10.0.2.2:8811"))
        assertNull(ApiClient.checkServer("not a url"))
    }
}
