// 4-ai: AI Blog Generator + Search Engine. Tetap lulus tanpa OPENAI_KEY karena backend
// memakai penulis lokal (generated_by = "local") bila OpenAI tidak tersedia.
import assert from 'node:assert/strict'
import { before, describe, test } from 'node:test'
import { CATEGORIES, api, expectFail, expectOk, registerAndLogin } from './helpers.js'

const AI_TIMEOUT = 120_000

function assertArticle(data) {
  for (const field of ['title', 'category', 'excerpt', 'content']) {
    assert.equal(typeof data[field], 'string', `${field} bukan string`)
    assert.ok(data[field].trim().length > 0, `${field} kosong`)
  }
  assert.ok(CATEGORIES.includes(data.category), `kategori ${data.category} tidak valid`)
  assert.ok(Array.isArray(data.sources), 'sources bukan array')
  assert.equal(typeof data.web_search_used, 'boolean')
  assert.ok(['openai', 'local'].includes(data.generated_by))
}

describe('4-ai', () => {
  let token

  before(async () => {
    ;({ token } = await registerAndLogin('ai'))
  })

  test('tanpa token → 401', async () => {
    expectFail(await api('POST', '/api/posts/ai-generate', { body: { topic: 'AI' } }), 401)
  })

  test('topik kosong / kategori tidak valid → 400', async () => {
    const res = await api('POST', '/api/posts/ai-generate', { token, body: {} })
    expectFail(res, 400)
    assert.match(res.body.message.toLowerCase(), /topic/)
    expectFail(await api('POST', '/api/posts/ai-generate', { token, body: { topic: '   ' } }), 400)
    expectFail(await api('POST', '/api/posts/ai-generate', { token, body: { topic: 'AI', category: 'Olahraga' } }), 400)
  })

  test('generate tanpa penelusuran web', { timeout: AI_TIMEOUT }, async () => {
    const data = expectOk(await api('POST', '/api/posts/ai-generate', {
      token,
      body: { topic: 'Manfaat membaca buku setiap hari', category: 'Gaya Hidup', include_web_search: false },
    }))
    assertArticle(data)
    assert.equal(data.web_search_used, false)
    assert.equal(data.sources.length, 0)
  })

  test('generate dengan penelusuran mesin pencari', { timeout: AI_TIMEOUT }, async () => {
    const data = expectOk(await api('POST', '/api/posts/ai-generate', {
      token,
      body: { topic: 'Tren kecerdasan buatan di pendidikan', category: 'Teknologi', tone: 'santai', include_web_search: true },
    }))
    assertArticle(data)
    assert.ok(data.sources.length <= 5)
    for (const s of data.sources) assert.match(s.url, /^https?:\/\//)
    if (data.sources.length > 0) assert.match(data.content, /Sumber Rujukan/)
    else assert.ok(data.search_warning || !data.web_search_used, 'tanpa sumber tetapi tidak ada search_warning')
  })

  test('hasil AI dapat disimpan sebagai artikel', { timeout: AI_TIMEOUT }, async () => {
    const draft = expectOk(await api('POST', '/api/posts/ai-generate', { token, body: { topic: 'Tips belajar Golang' } }))
    const post = expectOk(await api('POST', '/api/posts', {
      token,
      body: { title: draft.title, category: draft.category, excerpt: draft.excerpt, content: draft.content },
    }), 201)
    assert.ok(post.slug)
    assert.ok(post.reading_time >= 1)
  })
})
