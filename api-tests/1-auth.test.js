// 1-auth: registrasi, login JWT, profil, logout.
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { api, expectFail, expectOk, login, register, registerAndLogin, uid } from './helpers.js'

describe('1-auth', () => {
  test('register penulis baru → 201 dengan token JWT & user (tanpa password)', async () => {
    const username = `budi_${uid()}`
    const res = await api('POST', '/api/auth/register', {
      body: { name: 'Budi Santoso', username, email: `${username}@example.com`, password: 'rahasia123' },
    })
    const data = expectOk(res, 201)
    assert.equal(data.token.split('.').length, 3)
    assert.equal(data.user.username, username)
    assert.equal(data.user.role, 'member')
    assert.ok(!JSON.stringify(data).toLowerCase().includes('password'))
  })

  test('register tanpa name tetap berhasil (name = username)', async () => {
    const username = `min-${uid()}`
    const data = expectOk(await api('POST', '/api/auth/register', { body: { username, email: `${username}@example.com`, password: 'rahasia123' } }), 201)
    assert.equal(data.user.name, username)
  })

  test('register username / email duplikat → 400', async () => {
    const { username } = await register()
    expectFail(await api('POST', '/api/auth/register', {
      body: { name: 'X', username, email: `lain_${uid()}@example.com`, password: 'rahasia123' },
    }), 400)
    expectFail(await api('POST', '/api/auth/register', {
      body: { name: 'X', username: `lain_${uid()}`, email: `${username}@example.com`, password: 'rahasia123' },
    }), 400)
  })

  test('register tidak valid → 400 (tanpa email / password pendek / email salah)', async () => {
    const u = () => `u_${uid()}`
    for (const body of [
      { name: 'A', username: u(), password: 'rahasia123' },
      { name: 'A', username: u(), email: `${u()}@example.com`, password: '123' },
      { name: 'A', username: u(), email: 'bukan-email', password: 'rahasia123' },
      '{rusak',
    ]) {
      expectFail(await api('POST', '/api/auth/register', { body }), 400)
    }
  })

  test('login dengan username → token JWT & data user', async () => {
    const { username, password } = await register()
    const data = await login(username, password)
    assert.equal(data.token.split('.').length, 3, 'token bukan JWT')
    assert.equal(data.user.username, username)
  })

  test('login dengan email', async () => {
    const { username, password } = await register()
    expectOk(await api('POST', '/api/auth/login', { body: { username: `${username}@example.com`, password } }))
    expectOk(await api('POST', '/api/auth/login', { body: { email: `${username}@example.com`, password } }))
  })

  test('login password salah / user tidak ada → 401', async () => {
    const { username } = await register()
    expectFail(await api('POST', '/api/auth/login', { body: { username, password: 'salah' } }), 401)
    expectFail(await api('POST', '/api/auth/login', { body: { username: `tidakada_${uid()}`, password: 'x' } }), 401)
  })

  test('login tanpa password → 400', async () => {
    expectFail(await api('POST', '/api/auth/login', { body: { username: 'admin' } }), 400)
  })

  test('profil: tanpa token 401, dengan token 200, update parsial', async () => {
    expectFail(await api('GET', '/api/auth/profile'), 401)
    expectFail(await api('GET', '/api/auth/profile', { token: 'token.palsu.sekali' }), 401)

    const { token, username } = await registerAndLogin()
    assert.equal(expectOk(await api('GET', '/api/auth/profile', { token })).username, username)

    const updated = expectOk(await api('PUT', '/api/auth/profile', { token, body: { name: 'Nama Baru', bio: 'Penulis uji' } }))
    assert.equal(updated.name, 'Nama Baru')
    assert.equal(updated.bio, 'Penulis uji')
  })

  test('logout → token lama ditolak', async () => {
    const { token } = await registerAndLogin()
    expectOk(await api('POST', '/api/auth/logout', { token }))
    expectFail(await api('GET', '/api/auth/profile', { token }), 401)
  })
})
