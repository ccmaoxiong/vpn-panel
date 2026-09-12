import re, os, sys

js = open("static/js/app.js", encoding="utf-8").read()

# 1. all getElementById targets must exist in at least one template
ids = set(re.findall(r'getElementById\("([^"]+)"\)', js))
tmpl = ""
for f in os.listdir("templates"):
    tmpl += open("templates/" + f, encoding="utf-8").read()
problems = []
for i in sorted(ids):
    if ('id="%s"' % i) not in tmpl:
        problems.append("missing element id: %s" % i)
print("JS 引用的元素 ID: %d 个" % len(ids))

# 2. all functions called in template onclick attributes must be defined in app.js
onclicks = set(re.findall(r'onclick="([A-Za-z_$][\w$]*)\(', tmpl))
onkeys = set(re.findall(r'onkeyup="([A-Za-z_$][\w$]*)\(', tmpl))
defined = set(re.findall(r'^\s*(?:async\s+)?function\s+([A-Za-z_$][\w$]*)', js, re.M))
defined |= set(re.findall(r'window\.([A-Za-z_$][\w$]*)\s*=', js))
for fn in sorted(onclicks | onkeys):
    if fn not in defined:
        problems.append("undefined handler: %s" % fn)
print("模板 onclick/keyup 函数: %d 个" % len(onclicks | onkeys))

# 3. functions app.js calls internally on load must exist (basic call graph of direct calls)
internal = set(re.findall(r'(?<![\w$.])([A-Za-z_$][\w$]*)\s*\(', js))
for fn in sorted(internal):
    if fn not in defined and fn not in {"function", "if", "for", "while", "return", "catch", "switch", "setTimeout", "clearTimeout", "confirm", "fetch", "parseInt", "parseFloat", "isNaN", "JSON", "Number", "String", "Boolean", "Array", "Object", "Math", "Date", "Promise", "Error", "console", "document", "window", "navigator", "URL", "Blob", "location", "history", "escapeHtml", "getElementById", "querySelector", "querySelectorAll", "addEventListener", "createElement", "appendChild", "createObjectURL", "revokeObjectURL", "classList", "preventDefault", "stopPropagation", "textContent", "setAttribute", "getAttribute", "dataset", "innerHTML", "toast", "api", "openModal", "closeModal"}:
        problems.append("undefined internal call: %s" % fn)

print("\n" + ("PROBLEMS:" if problems else "FRONTEND CROSS-CHECK PASSED"))
for p in problems:
    print(" -", p)
sys.exit(1 if problems else 0)
