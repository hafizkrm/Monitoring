const fs = require('fs');
const path = require('path');

function walkDir(dir, callback) {
    fs.readdirSync(dir).forEach(f => {
        let dirPath = path.join(dir, f);
        let isDirectory = fs.statSync(dirPath).isDirectory();
        isDirectory ? walkDir(dirPath, callback) : callback(path.join(dir, f));
    });
}

walkDir('frontend/src', function(filePath) {
    if (filePath.endsWith('.html') || filePath.endsWith('.js')) {
        let content = fs.readFileSync(filePath, 'utf8');
        let newContent = content.replace(/<select\s+([^>]+)>/gi, function(match, attrs) {
            let idMatch = attrs.match(/id=['"]([^'"]+)['"]/i);
            if (idMatch && !attrs.toLowerCase().includes('aria-label')) {
                return '<select ' + attrs + ' aria-label="' + idMatch[1] + '">';
            }
            return match;
        });
        if (newContent !== content) {
            fs.writeFileSync(filePath, newContent, 'utf8');
            console.log('Fixed', filePath);
        }
    }
});
console.log('Done.');
