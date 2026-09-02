from pathlib import Path
import re

path = Path('.github/remove_pre_v1_compat.py')
text = path.read_text()

old = "remove_line(path, '\\ta.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, a.Health))\\n', count=2)"
new = "remove_line(path, '\\ta.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, a.Health))\\n', count=1)\nremove_line(path, '\\ta.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, health))\\n', count=1)"
if text.count(old) != 1:
    raise RuntimeError('expected status cleanup transform line not found')
text = text.replace(old, new)

old = '''text = read(path)\ntext = text.replace("action.Allocation.Name", "action.Allocation.ID")\ntext = text.replace("allocation.Name", "allocation.ID")\ntext = text.replace("alloc.Name", "alloc.ID")\ntext = re.sub(r",\\s*Name:\\s*alloc\\.ID", "", text)\nwrite(path, text)'''
new = '''text = read(path)\ntext = re.sub(r"\\baction\\.Allocation\\.Name\\b", "action.Allocation.ID", text)\ntext = re.sub(r"\\bactions\\[i\\]\\.Allocation\\.Name\\b", "actions[i].Allocation.ID", text)\ntext = re.sub(r"\\ballocation\\.Name\\b", "allocation.ID", text)\ntext = re.sub(r"\\balloc\\.Name\\b", "alloc.ID", text)\ntext = re.sub(r",\\s*Name:\\s*alloc\\.ID", "", text)\nwrite(path, text)'''
if text.count(old) != 1:
    raise RuntimeError('expected reconciler field rewrite block not found')
text = text.replace(old, new)

needle = '''    text = re.sub(r"^\\s*Status:\\s*string\\([^\\n]*\\.Status\\),\\n", "", text, flags=re.M)\n    write(response_path, text)'''
replacement = '''    text = re.sub(r"^\\s*Status:\\s*string\\([^\\n]*\\.Status\\),\\n", "", text, flags=re.M)\n    text = text.replace(", Status: string(a.Status)", "")\n    text = text.replace(", Status: string(allocation.Status)", "")\n    if response_path == "orchestrator/internal/server/server.go":\n        text = re.sub(r"\\ba\\.Name\\b", "a.ID", text)\n    write(response_path, text)'''
if text.count(needle) != 1:
    raise RuntimeError('expected public response cleanup block not found')
text = text.replace(needle, replacement)

path.write_text(text)

# Removing compatibility helpers makes these imports unused.
types = Path('orchestrator/internal/spec/types.go')
types_text = types.read_text().replace('\t"fmt"\n', '')
types.write_text(types_text)

allocations = Path('orchestrator/internal/server/allocations.go')
allocations_text = allocations.read_text().replace('\t"time"\n', '')
allocations.write_text(allocations_text)

# Tests should construct and diagnose allocations using the canonical fields,
# without invoking the deleted persisted-state migration helper.
alloc_test = Path('orchestrator/internal/server/allocations_test.go')
alloc_text = alloc_test.read_text()
alloc_text = alloc_text.replace('Name:          "default-web-frontend-deadbeef"', 'ID:            "default-web-frontend-deadbeef"')
alloc_text = alloc_text.replace('Name:          "default-web-frontend-old"', 'ID:            "default-web-frontend-old"')
alloc_text = re.sub(r'^\s*alloc\.normalize\(now\)\n', '', alloc_text, flags=re.M)
alloc_test.write_text(alloc_text)

update_test = Path('orchestrator/internal/server/update_test.go')
update_text = update_test.read_text().replace('a.Name', 'a.ID')
update_text = re.sub(r'^\s*a\.normalize\(now\)\n', '', update_text, flags=re.M)
update_test.write_text(update_text)

regression_test = Path('orchestrator/internal/server/update_regression_test.go')
regression_text = regression_test.read_text().replace('old.Name', 'old.ID')
regression_text = re.sub(r'^\s*a\.normalize\(now\)\n', '', regression_text, flags=re.M)
regression_test.write_text(regression_text)

state_test = Path('orchestrator/internal/server/state_test.go')
state_text = state_test.read_text()
state_text = state_text.replace('"github.com/clofour/trellis/internal/spec"\n', '"github.com/clofour/trellis/internal/lifecycle"\n\t"github.com/clofour/trellis/internal/spec"\n')
state_test.write_text(state_text)
