import { getRememberedUsername, isAuthenticated, login, verifySession } from '../../core/services/auth.service.js?v=auth-session-1';

const loginContainer = document.getElementById('login-container');
const form = document.getElementById('login-form');
const submitBtn = document.getElementById('btn-submit');
const errorMsg = document.getElementById('error-msg');
const errorText = document.getElementById('error-text');
const usernameInput = document.getElementById('username');
const passwordInput = document.getElementById('password');
const rememberInput = document.getElementById('remember');
const togglePasswordBtn = document.getElementById('toggle-password');
const toggleIcon = document.getElementById('toggle-icon');
const infoModal = document.getElementById('info-modal');
const modalTitle = document.getElementById('modal-title');
const modalDesc = document.getElementById('modal-desc');
const modalIcon = document.getElementById('modal-icon');
const modalCloseBtn = document.getElementById('modal-close-btn');

const rememberedUsername = getRememberedUsername();
usernameInput.value = rememberedUsername;
rememberInput.checked = Boolean(rememberedUsername);

if (isAuthenticated()) {
    verifySession().then(user => {
        if (user) window.location.replace('index.html');
    }).catch(() => {});
}

function openModal(title, description, iconClass, trigger) {
    modalTitle.textContent = title;
    modalDesc.textContent = description;
    modalIcon.className = `fas ${iconClass}`;
    infoModal.classList.add('active');
    infoModal.setAttribute('aria-hidden', 'false');
    infoModal.dataset.triggerId = trigger.id;
    modalCloseBtn.focus();
}

function closeModal() {
    infoModal.classList.remove('active');
    infoModal.setAttribute('aria-hidden', 'true');
    document.getElementById(infoModal.dataset.triggerId)?.focus();
}

function showError(message, targetInput) {
    errorText.textContent = message;
    errorMsg.classList.add('visible');
    loginContainer.classList.remove('shake');
    void loginContainer.offsetWidth;
    loginContainer.classList.add('shake');
    targetInput?.focus();
}

modalCloseBtn.addEventListener('click', closeModal);
infoModal.addEventListener('click', event => {
    if (event.target === infoModal) closeModal();
});
document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && infoModal.classList.contains('active')) closeModal();
});

togglePasswordBtn.addEventListener('click', () => {
    const showPassword = passwordInput.type === 'password';
    passwordInput.type = showPassword ? 'text' : 'password';
    toggleIcon.className = showPassword ? 'far fa-eye-slash' : 'far fa-eye';
    togglePasswordBtn.setAttribute('aria-pressed', String(showPassword));
});

document.getElementById('btn-forgot').addEventListener('click', event => {
    event.preventDefault();
    openModal('Pemulihan Kata Sandi', 'Untuk menjaga keamanan portal monitoring jaringan, pemulihan kata sandi dilakukan terpusat oleh Network Administrator. Silakan hubungi Tim IT Infrastructure / System Admin.', 'fa-key', event.currentTarget);
});

document.getElementById('btn-register').addEventListener('click', event => {
    event.preventDefault();
    openModal('Pendaftaran Akun Baru', 'Portal NetMon menerapkan kebijakan akses terbatas. Hak akses dan pendaftaran akun baru hanya dapat dibuat langsung oleh Network Administrator.', 'fa-user-shield', event.currentTarget);
});

form.addEventListener('submit', async event => {
    event.preventDefault();
    const username = usernameInput.value.trim();
    const password = passwordInput.value;
    if (!username) return showError('Silakan masukkan username Anda.', usernameInput);
    if (!password) return showError('Silakan masukkan password Anda.', passwordInput);

    errorMsg.classList.remove('visible');
    submitBtn.disabled = true;
    submitBtn.setAttribute('aria-busy', 'true');
    submitBtn.innerHTML = '<i class="fas fa-circle-notch spinner" aria-hidden="true"></i><span>Memproses...</span>';
    try {
        await login(username, password, rememberInput.checked);
        submitBtn.innerHTML = '<i class="fas fa-check" aria-hidden="true"></i><span>Berhasil</span>';
        submitBtn.style.background = 'linear-gradient(135deg, #059669, #10b981)';
        window.location.replace('index.html');
    } catch (error) {
        const message = error.message === 'Failed to fetch'
            ? 'Tidak dapat terhubung ke server. Periksa koneksi Anda.'
            : error.message || 'Terjadi kesalahan saat login';
        showError(message, usernameInput);
        submitBtn.disabled = false;
        submitBtn.removeAttribute('aria-busy');
        submitBtn.innerHTML = '<span>MASUK</span>';
    }
});
