// 1-auth: registrasi, login JWT, profil, logout.
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { api, expectFail, expectOk, login, register, registerAndLogin, uid } from './helpers.js'

describe('1-auth', () => {
  test('register penulis baru → 201 tanpa membocorkan password', async () => {
    const username = `budi_${uid()}`
    const res = await api('POST', '/api/auth/register', {
      body: { name: 'Budi Santoso', username, email: `${username}@example.com`, password: 'rahasia123' },
    })
    const data = expectOk(res, 201)
    assert.equal(data.username, username)
    assert.equal(data.role, 'member')
    assert.equal(data.password, undefined)
  })

  test('register hanya username & password (name/email opsional)', async () => {
    const username = `min-${uid()}`
    const data = expectOk(await api('POST', '/api/auth/register', { body: { username, password: 'rahasia123' } }), 201)
    assert.equal(data.username, username)
  })

  test('register username duplikat → 409', async () => {
    const { username } = await register()
    const res = await api('POST', '/api/auth/register', {
      body: { name: 'X', username, email: `lain_${uid()}@example.com`, password: 'rahasia123' },
    })
    expectFail(res, 409)
  })

  test('register data tidak valid → 400', async () => {
    expectFail(await api('POST', '/api/auth/register', { body: { username: `u_${uid()}` } }), 400)
    expectFail(await api('POST', '/api/auth/register', { body: { username: `u_${uid()}`, email: 'bukan-email', password: 'rahasia123' } }), 400)
    expectFail(await api('POST', '/api/auth/register', { body: '{rusak' }), 400)
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
