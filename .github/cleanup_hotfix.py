from pathlib import Path

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
replacement = '''    text = re.sub(r"^\\s*Status:\\s*string\\([^\\n]*\\.Status\\),\\n", "", text, flags=re.M)\n    text = text.replace(", Status: string(a.Status)", "")\n    text = text.replace(", Status: string(allocation.Status)", "")\n    write(response_path, text)'''
if text.count(needle) != 1:
    raise RuntimeError('expected public response cleanup block not found')
text = text.replace(needle, replacement)

path.write_text(text)

# Removing APIAccessMode.UnmarshalJSON makes fmt unused in this file.
types = Path('orchestrator/internal/spec/types.go')
types_text = types.read_text().replace('\t"fmt"\n', '')
types.write_text(types_text)
