// Global switchView fallback definition to ensure inline onclick handlers never throw ReferenceError
window.switchView = window.switchView || function(view) {
    if (typeof window._switchViewImpl === 'function') {
        return window._switchViewImpl(view);
    }
    const navItem = document.querySelector(`.nav-item[data-view="${view}"]`);
    if (navItem) {
        navItem.click();
    }
};

(function fixCachedTooltipBug() {
    const oldTooltip = document.getElementById('global-tooltip');
    if (oldTooltip) oldTooltip.remove();
    
    const newTooltip = document.createElement('div');
    newTooltip.id = 'new-global-tooltip';
    newTooltip.style.cssText = 'display: none; position: absolute; z-index: 9999; background: rgba(15,23,42,0.95); border: 1px solid rgba(255,255,255,0.1); backdrop-filter: blur(8px); padding: 6px 12px; border-radius: 6px; font-size: 11px; font-weight: 500; color: #fff; box-shadow: 0 4px 12px rgba(0,0,0,0.5); pointer-events: none; transition: opacity 0.15s ease, transform 0.1s ease; transform: translateY(0); white-space: nowrap; opacity: 0;';
    document.body.appendChild(newTooltip);

    document.addEventListener('mouseover', (e) => {
        const target = e.target.closest('[data-tooltip], [title]');
        if (!target) return;
        
        if (target.hasAttribute('title') && target.getAttribute('title').trim() !== '') {
            target.setAttribute('data-tooltip', target.getAttribute('title'));
            target.removeAttribute('title');
        }
        
        const text = target.getAttribute('data-tooltip');
        if (text) {
            newTooltip.textContent = text;
            
            const positionTooltip = (evt) => {
                newTooltip.style.left = (evt.pageX + 15) + 'px';
                newTooltip.style.top = (evt.pageY + 15) + 'px';
            };
            positionTooltip(e);
            
            newTooltip.style.display = 'block';
            void newTooltip.offsetWidth;
            newTooltip.style.opacity = '1';
            newTooltip.style.transform = 'translateY(-4px)';
            
            const moveHandler = (evt) => positionTooltip(evt);
            target.addEventListener('mousemove', moveHandler);
            
            target.addEventListener('mouseleave', () => {
                newTooltip.style.opacity = '0';
                newTooltip.style.transform = 'translateY(0)';
                setTimeout(() => {
                    if (newTooltip.style.opacity === '0') newTooltip.style.display = 'none';
                }, 150);
                target.removeEventListener('mousemove', moveHandler);
            }, { once: true });
        }
    });
})();
