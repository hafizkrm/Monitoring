import os, re
for root, dirs, files in os.walk('frontend'):
    for file in files:
        if file.endswith('.html') or file.endswith('.js'):
            path = os.path.join(root, file)
            with open(path, 'r', encoding='utf-8') as f:
                content = f.read()
            def repl(m):
                attrs = m.group(1)
                match = re.search(r'id=[\'\"]([^\'\"]+)[\'\"]', attrs, re.I)
                if match and 'aria-label' not in attrs.lower():
                    idx = match.group(1)
                    return f'<select {attrs} aria-label=\"{idx}\">'
                return m.group(0)
            new_content = re.sub(r'<select\s+([^>]+)>', repl, content, flags=re.I)
            if new_content != content:
                with open(path, 'w', encoding='utf-8') as f:
                    f.write(new_content)
                print('Fixed', path)
print('Done.')
