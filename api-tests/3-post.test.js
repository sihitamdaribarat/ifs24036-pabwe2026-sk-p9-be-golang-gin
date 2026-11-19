// 3-post: CRUD artikel penulis, slug unik, reading time, toggle publish, statistik.
import assert from 'node:assert/strict'
import { before, describe, test } from 'node:test'
import { api, createPost, expectFail, expectOk, postPayload, registerAndLogin, uid } from './helpers.js'

describe('3-post', () => {
  let author
  let other

  before(async () => {
    author = await registerAndLogin('penulis')
    other = await registerAndLogin('lain')
  })

  test('tanpa token → 401', async () => {
    expectFail(await api('GET', '/api/posts'), 401)
    expectFail(await api('POST', '/api/posts', { body: postPayload() }), 401)
  })

  test('buat artikel → 201 dengan slug & reading time (kata / 200)', async () => {
    const title = `Belajar Golang Gin ${uid()}`
    const post = await createPost(author.token, { title })
    assert.equal(post.title, title)
    assert.equal(post.slug, title.toLowerCase().replace(/\s+/g, '-'))
    assert.equal(post.reading_time, 3) // 451 kata → ceil(451/200)
    assert.equal(post.is_published, false)
    assert.equal(post.published_at, null)
  })

  test('judul sama → slug unik', async () => {
    const title = `Judul Kembar ${uid()}`
    const a = await createPost(author.token, { title })
    const b = await createPost(author.token, { title })
    assert.notEqual(a.slug, b.slug)
  })

  test('validasi: judul/konten kosong & kategori tidak valid → 400', async () => {
    expectFail(await api('POST', '/api/posts', { token: author.token, body: postPayload({ title: '' }) }), 400)
    expectFail(await api('POST', '/api/posts', { token: author.token, body: postPayload({ content: '' }) }), 400)
    expectFail(await api('POST', '/api/posts', { token: author.token, body: postPayload({ category: 'Olahraga' }) }), 400)
  })

  test('daftar artikel milik sendiri (terbit & draf)', async () => {
    const published = await createPost(author.token, { is_published: true })
    const list = expectOk(await api('GET', '/api/posts', { token: author.token }))
    assert.ok(list.some((p) => p.id === published.id))
    assert.ok(list.every((p) => p.user_id === author.user.id), 'artikel penulis lain ikut tampil')
  })

  test('detail by id: milik sendiri 200, tidak ada 404, id rusak 400, milik orang lain 403', async () => {
    const post = await createPost(author.token)
    assert.equal(expectOk(await api('GET', `/api/posts/${post.id}`, { token: author.token })).id, post.id)
    expectFail(await api('GET', '/api/posts/99999999', { token: author.token }), 404)
    expectFail(await api('GET', '/api/posts/abc', { token: author.token }), 400)
    expectFail(await api('GET', `/api/posts/${post.id}`, { token: other.token }), 403)
  })

  test('update parsial: hanya judul; ID tidak ada 404; milik orang lain 403', async () => {
    const post = await createPost(author.token)
    const updated = expectOk(await api('PUT', `/api/posts/${post.id}`, { token: author.token, body: { title: `Hanya Judul ${uid()}` } }))
    assert.equal(updated.content, post.content)
    expectFail(await api('PUT', '/api/posts/99999999', { token: author.token, body: { title: 'Tidak Ada' } }), 404)
    expectFail(await api('PUT', `/api/posts/${post.id}`, { token: other.token, body: { title: 'Dibajak' } }), 403)
    expectFail(await api('PATCH', `/api/posts/${post.id}/toggle-publish`, { token: other.token }), 403)
    expectFail(await api('PATCH', '/api/posts/99999999/toggle-publish', { token: author.token }), 404)
  })

  test('update artikel → judul & slug berubah', async () => {
    const post = await createPost(author.token)
    const title = `Judul Diperbarui ${uid()}`
    const updated = expectOk(await api('PUT', `/api/posts/${post.id}`, { token: author.token, body: postPayload({ title, category: 'Bisnis' }) }))
    assert.equal(updated.title, title)
    assert.equal(updated.category, 'Bisnis')
    assert.notEqual(updated.slug, post.slug)
  })

  test('toggle publish: draf ↔ terbit dengan published_at', async () => {
    const post = await createPost(author.token)
    const on = expectOk(await api('PATCH', `/api/posts/${post.id}/toggle-publish`, { token: author.token }))
    assert.equal(on.is_published, true)
    assert.ok(on.published_at)
    const off = expectOk(await api('PATCH', `/api/posts/${post.id}/toggle-publish`, { token: author.token }))
    assert.equal(off.is_published, false)
  })

  test('statistik memakai total_articles / total_selesai / belum_selesai', async () => {
    const fresh = await registerAndLogin('statistik')
    await createPost(fresh.token, { is_published: true })
    await createPost(fresh.token)
    await createPost(fresh.token)
    assert.deepEqual(expectOk(await api('GET', '/api/posts/stats', { token: fresh.token })), { total_articles: 3, total_selesai: 1, belum_selesai: 2 })
  })

  test('hapus artikel → statistik berkurang; orang lain 403; tidak ada 404', async () => {
    const post = await createPost(author.token)
    const before = expectOk(await api('GET', '/api/posts/stats', { token: author.token }))
    expectFail(await api('DELETE', `/api/posts/${post.id}`, { token: other.token }), 403)
    expectOk(await api('DELETE', `/api/posts/${post.id}`, { token: author.token }))
    const after = expectOk(await api('GET', '/api/posts/stats', { token: author.token }))
    assert.equal(after.total_articles, before.total_articles - 1)
    expectFail(await api('DELETE', `/api/posts/${post.id}`, { token: author.token }), 404)
  })
})
