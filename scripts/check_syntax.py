import ast
import os
import sys

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

dirs_to_scan = [
    os.path.join(REPO_ROOT, "academic_service"),
    os.path.join(REPO_ROOT, "ai_service"),
    os.path.join(REPO_ROOT, "tests"),
]

files_to_check = []
for d in dirs_to_scan:
    if not os.path.exists(d):
        continue
    for root, _, files in os.walk(d):
        for file in files:
            if file.endswith(".py"):
                files_to_check.append(os.path.join(root, file))

errors = 0
for f in files_to_check:
    rel = os.path.relpath(f, REPO_ROOT)
    try:
        with open(f, encoding="utf-8") as fh:
            ast.parse(fh.read())
        print(f"  OK   {rel}")
    except SyntaxError as e:
        print(f"  FAIL {rel}: {e}")
        errors += 1
    except Exception as e:
        print(f"  ERR  {rel}: {e}")
        errors += 1

print(f"\nResult: {len(files_to_check) - errors}/{len(files_to_check)} Python files valid")
if errors:
    sys.exit(1)
print("All Python syntax checks passed!")

