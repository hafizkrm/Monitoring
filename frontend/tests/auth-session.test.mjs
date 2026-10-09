import assert from 'node:assert/strict';
import { createServer } from 'vite';

class MemoryStorage {
    values = new Map();
    getItem(key) { return this.values.get(key) ?? null; }
    setItem(key, value) { this.values.set(key, String(value)); }
    removeItem(key) { this.values.delete(key); }
}

globalThis.localStorage = new MemoryStorage();
globalThis.sessionStorage = new MemoryStorage();
globalThis.window = { dispatchEvent() {} };
globalThis.CustomEvent = class CustomEvent { constructor(type) { this.type = type; } };

let responseData = { user: { id: 9, username: 'viewer1', role: 'viewer' } };
globalThis.fetch = async () => new Response(JSON.stringify(responseData), {
    status: 200,
    headers: { 'Content-Type': 'application/json' }
});

const server = await createServer({ configFile: false, server: { middlewareMode: true }, appType: 'custom' });
try {
    const { login } = await server.ssrLoadModule('/src/core/services/auth.service.js');
    const { canAccess, isAdmin } = await server.ssrLoadModule('/src/core/services/permission.service.js');

    await login(' viewer1 ', 'not-stored', true);
    assert.notEqual(sessionStorage.getItem('netmon_session'), null);
    assert.equal(localStorage.getItem('netmon_session'), null);
    assert.equal(localStorage.getItem('netmon_remembered_username'), 'viewer1');
    assert.equal(canAccess('dashboard'), true);
    assert.equal(canAccess('alerts'), true);
    assert.equal(canAccess('devices'), false);
    assert.equal(canAccess('reports'), false);
    assert.equal(isAdmin(), false);

    sessionStorage.removeItem('netmon_session');
    responseData = { user: { id: 1, username: 'admin1', role: 'admin' } };
    await login('admin1', 'not-stored', false);
    assert.equal(canAccess('settings'), true);
    assert.equal(canAccess('users'), true);
    assert.equal(isAdmin(), true);
    assert.equal(localStorage.getItem('netmon_remembered_username'), null);

    sessionStorage.removeItem('netmon_session');
    responseData = { status: 'success' };
    await assert.rejects(login('viewer1', 'not-stored'), /Respons autentikasi tidak valid/);
    assert.equal(sessionStorage.getItem('netmon_session'), null);
    console.log('Auth session checks passed');
} finally {
    await server.close();
}
