// Komponen UI: Table
// Fungsi helper untuk merender tabel secara dinamis agar seragam di semua halaman.

export function renderTable(columns, data) {
    // Placeholder implementation
    return `
        <table class="data-table">
            <thead>
                <tr>${columns.map(c => `<th>${c.label}</th>`).join('')}</tr>
            </thead>
            <tbody>
                ${data.map(row => `
                    <tr>${columns.map(c => `<td>${row[c.key]}</td>`).join('')}</tr>
                `).join('')}
            </tbody>
        </table>
    `;
}
