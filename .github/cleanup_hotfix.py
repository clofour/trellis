from pathlib import Path

path = Path('.github/remove_pre_v1_compat.py')
text = path.read_text()
old = "remove_line(path, '\\ta.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, a.Health))\\n', count=2)"
new = "remove_line(path, '\\ta.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, a.Health))\\n', count=1)\nremove_line(path, '\\ta.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, health))\\n', count=1)"
if text.count(old) != 1:
    raise RuntimeError('expected cleanup transform line not found')
path.write_text(text.replace(old, new))

# Removing APIAccessMode.UnmarshalJSON makes fmt unused in this file.
types = Path('orchestrator/internal/spec/types.go')
types_text = types.read_text()
types_text = types_text.replace('\t"fmt"\n', '')
types.write_text(types_text)
