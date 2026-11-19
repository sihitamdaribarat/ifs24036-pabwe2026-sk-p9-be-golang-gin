// 0-setup: server hidup, database terhubung, Swagger, format 404 & CORS.
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { BASE_URL, api, expectFail, expectOk, expectStatus } from './helpers.js'

describe('0-setup', () => {
  test(`server dapat dihubungi di ${BASE_URL}`, async () => {
    const res = await api('GET', '/api/health')
    assert.ok(res.status > 0)
  })

  test('GET /api/health → database ok', async () => {
    const data = expectOk(await api('GET', '/api/health'))
    assert.equal(data.database, 'ok')
    assert.ok(data.users >= 1, 'akun admin belum di-seed')
  })

  test('GET / dialihkan ke Swagger UI', async () => {
    const res = await api('GET', '/')
    assert.ok([301, 302, 307, 308].includes(res.status), `HTTP ${res.status}`)
    assert.match(res.headers.get('location') || '', /swagger/)
  })

  test('GET /swagger/index.html menampilkan Swagger UI', async () => {
    const res = await api('GET', '/swagger/index.html')
    expectStatus(res, 200)
    assert.match(res.text, /swagger/i)
  })

  test('GET /api.json berisi spesifikasi OpenAPI seluruh endpoint', async () => {
    const res = await api('GET', '/api.json')
    expectStatus(res, 200)
    const paths = Object.keys(res.body.paths || {})
    for (const p of ['/api/auth/login', '/api/public/posts', '/api/posts', '/api/posts/ai-generate', '/api/admin/users']) {
      assert.ok(paths.includes(p), `path ${p} tidak ada di api.json`)
    }
  })

  test('rute tidak dikenal → 404 "Endpoint not found"', async () => {
    const res = await api('GET', '/api/tidak-ada')
    expectFail(res, 404)
    assert.equal(res.body.message, 'Endpoint not found')
  })

  test('CORS preflight diizinkan', async () => {
    const res = await api('OPTIONS', '/api/posts', {
      headers: { Origin: 'https://contoh.s1if.cloud', 'Access-Control-Request-Method': 'POST' },
    })
    assert.ok([200, 204].includes(res.status), `HTTP ${res.status}`)
    assert.ok(res.headers.get('access-control-allow-origin'), 'header Access-Control-Allow-Origin tidak ada')
  })
})
