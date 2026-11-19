// Helper uji API black-box Delcom Blog (meniru pola penguji otomatis delcom.grading).
//
// Jalankan dari folder api-tests:
//   npm test                                              -> server lokal http://localhost:8081
//   BASE_URL=https://<backend>.s1if.cloud npm test        -> server hasil deploy
// PowerShell:  $env:BASE_URL="https://..."; npm test
import assert from 'node:assert/strict'

export const BASE_URL = (process.env.BASE_URL || 'http://localhost:8081').replace(/\/+$/, '')
export const ADMIN = {
  username: process.env.ADMIN_USERNAME || 'admin',
  password: process.env.ADMIN_PASSWORD || 'admin',
}
export const CATEGORIES = ['Teknologi', 'Pendidikan', 'Gaya Hidup', 'Bisnis', 'Opini', 'Umum']

export const uid = () => `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`

// api mengirim request JSON dan mengembalikan { status, body, text, headers }.
export async function api(method, path, { token, body, headers = {} } = {}) {
  const init = { method, headers: { Accept: 'application/json', ...headers }, redirect: 'manual' }
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json'
    init.body = typeof body === 'string' ? body : JSON.stringify(body)
  }
  if (token) init.headers.Authorization = `Bearer ${token}`

  const res = await fetch(BASE_URL + path, init)
  const text = await res.text()
  let json = null
  try {
    json = JSON.parse(text)
  } catch {
    // bukan JSON (misal HTML Swagger)
  }
  return { status: res.status, body: json, text, headers: res.headers }
}

// expectStatus memeriksa HTTP status dan menampilkan isi respons bila tidak sesuai.
export function expectStatus(res, code) {
  assert.equal(res.status, code, `HTTP ${res.status} (diharapkan ${code}): ${res.text.slice(0, 400)}`)
}

// expectEnvelope memeriksa format respons standar {status, message, data?}.
export function expectEnvelope(res, status) {
  assert.ok(res.body && typeof res.body === 'object', `respons bukan JSON: ${res.text.slice(0, 200)}`)
  assert.equal(res.body.status, status)
  assert.equal(typeof res.body.message, 'string')
  assert.ok(res.body.message.length > 0, 'message kosong')
}

export function expectOk(res, code = 200) {
  expectStatus(res, code)
  expectEnvelope(res, 'success')
  return res.body.data
}

export function expectFail(res, code) {
  expectStatus(res, code)
  expectEnvelope(res, 'fail')
}

export async function register(prefix = 'penulis') {
  const username = `${prefix}_${uid()}`
  const password = 'rahasia123'
  const res = await api('POST', '/api/auth/register', {
    body: { name: `Penulis ${username}`, username, email: `${username}@example.com`, password },
  })
  expectStatus(res, 201)
  return { username, password, user: res.body.data.user }
}

export async function login(username, password) {
  const res = await api('POST', '/api/auth/login', { body: { username, password } })
  const data = expectOk(res)
  assert.ok(data.token, 'token tidak ada di data.token')
  return data
}

// registerAndLogin membuat penulis baru lalu login, mengembalikan { username, password, token, user }.
export async function registerAndLogin(prefix) {
  const account = await register(prefix)
  const { token, user } = await login(account.username, account.password)
  return { ...account, token, user }
}

export async function loginAdmin() {
  return login(ADMIN.username, ADMIN.password)
}

export function postPayload(overrides = {}) {
  return {
    title: `Artikel Uji ${uid()}`,
    category: 'Teknologi',
    excerpt: 'Ringkasan singkat artikel uji.',
    content: `## Pendahuluan\n\n${'kata '.repeat(450)}`,
    cover_image_path: '',
    is_published: false,
    ...overrides,
  }
}

export async function createPost(token, overrides) {
  return expectOk(await api('POST', '/api/posts', { token, body: postPayload(overrides) }), 201)
}
