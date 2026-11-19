// 2-public: kategori, daftar & detail artikel terbit, pencarian dan filter kategori.
import assert from 'node:assert/strict'
import { before, describe, test } from 'node:test'
import { CATEGORIES, api, createPost, expectFail, expectOk, registerAndLogin, uid } from './helpers.js'

describe('2-public', () => {
  const keyword = `publik${uid()}`
  let published
  let draft

  before(async () => {
    const { token } = await registerAndLogin('publik')
    published = await createPost(token, { title: `Artikel ${keyword}`, category: 'Pendidikan', is_published: true })
    draft = await createPost(token, { title: `Draf ${keyword}`, category: 'Pendidikan', is_published: false })
  })

  test('GET /api/public/categories → 6 kategori', async () => {
    assert.deepEqual(expectOk(await api('GET', '/api/public/categories')), CATEGORIES)
  })

  test('GET /api/public/posts → array artikel terbit saja', async () => {
    const data = expectOk(await api('GET', '/api/public/posts'))
    assert.ok(Array.isArray(data))
    const serialized = JSON.stringify(data).toLowerCase()
    assert.ok(!serialized.includes('password'), 'data publik memuat password')
    assert.ok(!serialized.includes('user_id'), 'data publik memuat user_id (data internal)')
    for (const p of data) {
      assert.equal(typeof p.author_name, 'string', 'author_name tidak ada')
      assert.ok(p.reading_time >= 1)
      assert.ok(p.published_at)
    }
    assert.ok(data.every((p) => p.is_published === true), 'ada draf yang tampil di publik')
    assert.ok(data.some((p) => p.id === published.id))
    assert.ok(!data.some((p) => p.id === draft.id))
  })

  test('pencarian kata kunci ?q=', async () => {
    const data = expectOk(await api('GET', `/api/public/posts?q=${keyword}`))
    assert.deepEqual(data.map((p) => p.id), [published.id])
  })

  test('filter kategori ?category=', async () => {
    const data = expectOk(await api('GET', `/api/public/posts?category=Pendidikan&q=${keyword}`))
    assert.equal(data.length, 1)
    const other = expectOk(await api('GET', `/api/public/posts?category=Bisnis&q=${keyword}`))
    assert.equal(other.length, 0)
  })

  test('detail artikel berdasarkan slug lengkap dengan penulis & reading time', async () => {
    const data = expectOk(await api('GET', `/api/public/posts/${published.slug}`))
    assert.equal(data.id, published.id)
    assert.equal(data.user_id, undefined)
    assert.ok(data.author && data.author.username, 'data penulis tidak ada')
    assert.ok(data.reading_time >= 1)
    assert.ok(data.content.length > 0)
  })

  test('slug draf atau tidak ada → 404', async () => {
    expectFail(await api('GET', `/api/public/posts/${draft.slug}`), 404)
    expectFail(await api('GET', `/api/public/posts/tidak-ada-${uid()}`), 404)
  })
})
