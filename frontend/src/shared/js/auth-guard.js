(function () {
    try {
        var sessionStr = localStorage.getItem('netmon_session') || sessionStorage.getItem('netmon_session');
        if (!sessionStr) throw new Error('No session');
        var session = JSON.parse(sessionStr);
        if (!session.expiredAt) throw new Error('Invalid session format');
        if (new Date() > new Date(session.expiredAt)) throw new Error('Session expired');

        // Browser will automatically send the HttpOnly cookie with every fetch request
        // to the same origin, so we no longer need to manually inject the Authorization header.

    } catch (e) {
        window.location.href = 'login.html';
    }
})();

