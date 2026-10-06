const fs = require("fs");
const path = require("path");

function walk(dir) {
    let results = [];
    const list = fs.readdirSync(dir);
    list.forEach(file => {
        file = path.join(dir, file);
        const stat = fs.statSync(file);
        if (stat && stat.isDirectory()) {
            results = results.concat(walk(file));
        } else {
            if (file.endsWith(".html") || file.endsWith(".js") || file.endsWith(".css")) {
                results.push(file);
            }
        }
    });
    return results;
}

const files = walk("./src");
let changedFiles = 0;

files.forEach(file => {
    const content = fs.readFileSync(file, "utf8");
    let newContent = content.replace(/Rolegkat/g, "Perangkat");
    newContent = newContent.replace(/rolegkat/g, "perangkat");
    newContent = newContent.replace(/ROLEGKAT/g, "PERANGKAT");
    
    if (content !== newContent) {
        fs.writeFileSync(file, newContent, "utf8");
        changedFiles++;
    }
});

console.log(`Updated ${changedFiles} files.`);

