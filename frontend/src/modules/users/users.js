import './users.css';
import { isAdmin } from '../../core/services/permission.service.js';
import { getCurrentUser } from '../../core/services/auth.service.js';
import { activityLogger } from '../../core/services/log.service.js';
import { escapeHtml } from '../../shared/utils/helpers.js';

const API_URL = '/api/users';
const MIN_PASSWORD_LENGTH = 6;
const MODAL_FADE_MS = 200;
const SEARCH_DEBOUNCE_MS = 150;

const ROLE_ADMIN = 'admin';

// Module state: the list is fetched once per view entry and filtered locally,
// so typing in search or opening the edit modal never triggers a new request.
const state = {
    users: [],
    query: '',
    bound: false
};

const $ = (id) => document.getElementById(id);

/* -------------------------------------------------------------------------- */
/* Public API                                                                 */
/* -------------------------------------------------------------------------- */

export function initUser() {
    // Access is enforced by the router ACL; this is a defensive no-op guard.
    // (Never redirect here: the router pre-mounts modules at startup.)
    if (!isAdmin() || state.bound) return;

    bindEvents();
    state.bound = true;
}

/** Re-fetch users from the API and re-render. Called by the router on view entry. */
export async function refreshUsers() {
    if (!isAdmin()) return;
    try {
        const res = await fetch(API_URL);
        if (!res.ok) throw new Error(await readError(res));
        const data = await res.json();
        state.users = Array.isArray(data.data) ? data.data : [];
    } catch (e) {
        console.error('Failed to fetch users', e);
        window.showToast?.(`Failed to load user data: ${e.message}`, 'error');
        state.users = [];
    }
    render();
}

/* -------------------------------------------------------------------------- */
/* Rendering                                                                  */
/* -------------------------------------------------------------------------- */

function render() {
    renderStats();
    renderTable();
}

function renderStats() {
    const total = state.users.length;
    const admins = state.users.filter((u) => u.role === ROLE_ADMIN).length;

    const set = (id, text) => { const el = $(id); if (el) el.textContent = text; };
    set('stat-User-total', `${total} Akun`);
    set('stat-User-admin', `${admins} Admin`);
    set('stat-User-viewer', `${total - admins} User`);
}

function renderTable() {
    const tbody = $('User-table-body');
    if (!tbody) return;

    const q = state.query.toLowerCase();
    const rows = q
        ? state.users.filter((u) =>
            (u.name || '').toLowerCase().includes(q) ||
            (u.username || '').toLowerCase().includes(q))
        : state.users;

    if (rows.length === 0) {
        tbody.innerHTML = `<tr><td colspan="4" class="usr-table__empty"><i class="fas fa-user-slash mr-2"></i>Tidak ada data user yang sesuai.</td></tr>`;
        return;
    }

    const currentUserId = getCurrentUser()?.id;
    tbody.innerHTML = rows.map((u) => rowTemplate(u, u.id === currentUserId)).join('');
}

function getInitials(name) {
    if (!name) return 'U';
    return name.trim().split(/\s+/).map((n) => n[0]).join('').toUpperCase().slice(0, 2) || 'U';
}

function rowTemplate(u, isSelf) {
    const isAdminRole = u.role === ROLE_ADMIN;
    const name = escapeHtml(u.name);
    const username = escapeHtml(u.username);

    const roleBadge = isAdminRole
        ? `<span class="usr-role usr-role--admin"><i class="fas fa-shield-alt"></i> Administrator</span>`
        : `<span class="usr-role"><i class="fas fa-eye"></i> Viewer</span>`;

    const selfTag = isSelf ? '<span class="usr-self-tag">(Anda)</span>' : '';

    return `<tr>
        <td>
            <div class="usr-identity">
                <div class="usr-avatar${isAdminRole ? ' usr-avatar--admin' : ''}">${escapeHtml(getInitials(u.name))}</div>
                <div>
                    <div class="usr-name">${name} ${selfTag}</div>
                    <div class="usr-handle">@${username}</div>
                </div>
            </div>
        </td>
        <td>${roleBadge}</td>
        <td><span class="usr-pwd-mask" title="Password stored as hash">********</span></td>
        <td class="is-right">
            <div class="usr-actions">
                <button type="button" class="usr-action" data-action="edit" data-id="${u.id}" aria-label="Edit ${username}">
                    <i class="fas fa-edit"></i> Edit
                </button>
                <button type="button" class="usr-action usr-action--danger" data-action="delete" data-id="${u.id}" aria-label="Delete ${username}"${isSelf ? ' disabled title="Cannot delete your own account"' : ''}>
                    <i class="fas fa-trash"></i> Delete
                </button>
            </div>
        </td>
    </tr>`;
}

/* -------------------------------------------------------------------------- */
/* Event wiring (single delegated listener per root, no window globals)       */
/* -------------------------------------------------------------------------- */

function bindEvents() {
    const roots = [$('view-User'), $('add-User-modal'), $('Edit-User-modal')].filter(Boolean);
    roots.forEach((root) => root.addEventListener('click', onClick));

    $('add-User-form')?.addEventListener('submit', onCreateSubmit);
    $('Edit-User-form')?.addEventListener('submit', onEditSubmit);

    let searchTimer;
    $('search-User')?.addEventListener('input', (e) => {
        clearTimeout(searchTimer);
        searchTimer = setTimeout(() => {
            state.query = e.target.value.trim();
            renderTable();
        }, SEARCH_DEBOUNCE_MS);
    });

    document.addEventListener('keydown', (e) => {
        if (e.key !== 'Escape') return;
        document.querySelectorAll('.usr-modal.is-open').forEach(closeModal);
    });
}

function onClick(e) {
    // Backdrop click closes the modal
    if (e.target.classList?.contains('usr-modal')) {
        closeModal(e.target);
        return;
    }

    const btn = e.target.closest('[data-action]');
    if (!btn || btn.disabled) return;

    const id = Number(btn.dataset.id);
    switch (btn.dataset.action) {
        case 'open-add':     openModal($('add-User-modal')); break;
        case 'close-modal':  closeModal(btn.closest('.usr-modal')); break;
        case 'toggle-pwd':   togglePasswordVisibility(btn); break;
        case 'edit':         openEditModal(id); break;
        case 'delete':       deleteUser(id); break;
    }
}

/* -------------------------------------------------------------------------- */
/* Modals                                                                     */
/* -------------------------------------------------------------------------- */

function openModal(modal) {
    if (!modal) return;
    modal.classList.add('is-open');
    requestAnimationFrame(() => modal.classList.add('is-visible'));
    modal.querySelector('input:not([type="hidden"])')?.focus();
}

function closeModal(modal) {
    if (!modal) return;
    modal.classList.remove('is-visible');
    setTimeout(() => {
        modal.classList.remove('is-open');
        const form = modal.querySelector('form');
        form?.reset();
        // Restore any password fields that were toggled to plain text
        modal.querySelectorAll('[data-action="toggle-pwd"]').forEach((b) => setPasswordVisible(b, false));
    }, MODAL_FADE_MS);
}

function setPasswordVisible(btn, visible) {
    const input = $(btn.dataset.target);
    if (!input) return;
    input.type = visible ? 'text' : 'password';
    btn.innerHTML = visible ? '<i class="far fa-eye-slash"></i>' : '<i class="far fa-eye"></i>';
    btn.setAttribute('aria-label', visible ? 'Sembunyikan password' : 'Tampilkan password');
}

function togglePasswordVisibility(btn) {
    const input = $(btn.dataset.target);
    if (input) setPasswordVisible(btn, input.type === 'password');
}

function openEditModal(id) {
    const user = state.users.find((u) => u.id === id);
    if (!user) {
        window.showToast?.('User not found', 'error');
        return;
    }

    $('Edit-User-id').value = user.id;
    $('Edit-User-name').value = user.name || '';
    $('Edit-User-Username').value = user.username || '';
    $('Edit-User-Password').value = '';
    $('Edit-User-Role').value = user.role || '';

    openModal($('Edit-User-modal'));
}

/* -------------------------------------------------------------------------- */
/* CRUD                                                                       */
/* -------------------------------------------------------------------------- */

async function readError(res) {
    const text = await res.text().catch(() => '');
    try {
        const json = JSON.parse(text);
        return json.message || json.error || text || res.statusText;
    } catch {
        return text || res.statusText;
    }
}

async function sendJson(method, body) {
    const res = await fetch(API_URL, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    });
    if (!res.ok) throw new Error(await readError(res));
    return res;
}

function readForm(prefix) {
    return {
        name: $(`${prefix}-name`).value.trim(),
        username: $(`${prefix}-Username`).value.trim(),
        password: $(`${prefix}-Password`).value,
        role: $(`${prefix}-Role`).value
    };
}

/** Disable the submit button while a request is in flight (prevents double submit). */
async function withSubmitLock(form, task) {
    const btn = form.querySelector('[type="submit"]');
    if (btn?.disabled) return;
    if (btn) btn.disabled = true;
    try {
        await task();
    } finally {
        if (btn) btn.disabled = false;
    }
}

function onCreateSubmit(e) {
    e.preventDefault();
    const form = e.currentTarget;
    const data = readForm('User');

    if (!data.name || !data.username || !data.password || !data.role) {
        window.showToast?.('Mohon lengkapi semua data', 'error');
        return;
    }
    if (data.password.length < MIN_PASSWORD_LENGTH) {
        window.showToast?.(`Password minimal ${MIN_PASSWORD_LENGTH} karakter`, 'error');
        return;
    }

    withSubmitLock(form, async () => {
        try {
            await sendJson('POST', data);
            activityLogger.log({
                module: 'User',
                Action: 'CREATE',
                Description: `Membuat user baru: ${data.username} (${data.role})`
            });
            window.showToast?.('User berhasil ditambahkan', 'success');
            closeModal($('add-User-modal'));
            refreshUsers();
        } catch (err) {
            window.showToast?.(`Failed to add user: ${err.message}`, 'error');
        }
    });
}

function onEditSubmit(e) {
    e.preventDefault();
    const form = e.currentTarget;
    const id = parseInt($('Edit-User-id').value, 10);
    if (!id) return;

    const data = readForm('Edit-User');
    if (!data.name || !data.username || !data.role) {
        window.showToast?.('Mohon lengkapi semua data wajib', 'error');
        return;
    }
    if (data.password && data.password.length < MIN_PASSWORD_LENGTH) {
        window.showToast?.(`Password minimal ${MIN_PASSWORD_LENGTH} karakter`, 'error');
        return;
    }

    withSubmitLock(form, async () => {
        try {
            await sendJson('PUT', { id, ...data });
            activityLogger.log({
                module: 'User',
                Action: 'UPDATE',
                Description: `Memperbarui data user: ${data.username} (${data.role})`
            });
            window.showToast?.('Data user berhasil diperbarui', 'success');
            closeModal($('Edit-User-modal'));
            refreshUsers();
        } catch (err) {
            window.showToast?.(`Failed to update user: ${err.message}`, 'error');
        }
    });
}

async function deleteUser(id) {
    const user = state.users.find((u) => u.id === id);
    if (!user) return;
    if (user.id === getCurrentUser()?.id) {
        window.showToast?.('Tidak dapat menghapus akun sendiri', 'error');
        return;
    }
    if (!confirm(`Apakah Anda yakin ingin menghapus user "${user.username}"?`)) return;

    try {
        await sendJson('DELETE', { id });
        activityLogger.log({
            module: 'User',
            Action: 'HAPUS',
            Description: `Menghapus user: ${user.username} (ID: ${id})`
        });
        window.showToast?.('User berhasil dihapus', 'success');
        refreshUsers();
    } catch (err) {
        window.showToast?.(`Failed to delete user: ${err.message}`, 'error');
    }
}
