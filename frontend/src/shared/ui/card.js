// Komponen UI: Card
// Helper untuk membungkus konten dalam desain card.

export function createCard(title, contentHTML) {
    return `
        <div class="stat-card">
            <h3>${title}</h3>
            <div class="stat-card-content">
                ${contentHTML}
            </div>
        </div>
    `;
}
