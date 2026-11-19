// 5-admin: monitoring seluruh pengguna & artikel (khusus admin).
import assert from 'node:assert/strict'
import { before, describe, test } from 'node:test'
import { api, createPost, expectFail, expectOk, loginAdmin, registerAndLogin } from './helpers.js'

describe('5-admin', () => {
  let admin
  let member
  let draft

  before(async () => {
    admin = await loginAdmin()
    member = await registerAndLogin('dimonitor')
    await createPost(member.token, { is_published: true })
    draft = await createPost(member.token)
  })

  test('login admin (ADMIN_USERNAME / ADMIN_PASSWORD) → role admin', () => {
    assert.equal(admin.user.role, 'admin')
  })

  test('tanpa token 401, penulis biasa 403', async () => {
    expectFail(await api('GET', '/api/admin/users'), 401)
    for (const path of ['/api/admin/users', '/api/admin/posts', `/api/admin/users/${member.user.id}/posts`]) {
      expectFail(await api('GET', path, { token: member.token }), 403)
    }
  })

  test('GET /api/admin/users → seluruh pengguna beserta total artikel', async () => {
    const users = expectOk(await api('GET', '/api/admin/users', { token: admin.token }))
    const row = users.find((u) => u.id === member.user.id)
    assert.ok(row, 'penulis baru tidak ada di daftar')
    assert.equal(row.total_posts, 2)
    assert.equal(row.published_posts, 1)
    assert.ok(users.some((u) => u.role === 'admin'))
  })

  test('GET /api/admin/posts → seluruh artikel termasuk draf', async () => {
    const posts = expectOk(await api('GET', '/api/admin/posts', { token: admin.token }))
    const row = posts.find((p) => p.id === draft.id)
    assert.ok(row, 'draf penulis lain tidak terlihat oleh admin')
    assert.equal(row.author.username, member.username)
  })

  test('GET /api/admin/users/:id/posts → artikel milik pengguna tertentu', async () => {
    const posts = expectOk(await api('GET', `/api/admin/users/${member.user.id}/posts`, { token: admin.token }))
    assert.equal(posts.length, 2)
    assert.ok(posts.every((p) => p.user_id === member.user.id))
  })

  test('id pengguna bukan angka 400; tidak ada → daftar kosong (bukan error)', async () => {
    expectFail(await api('GET', '/api/admin/users/bukan-angka/posts', { token: admin.token }), 400)
    assert.deepEqual(expectOk(await api('GET', '/api/admin/users/999999999/posts', { token: admin.token })), [])
  })
})
