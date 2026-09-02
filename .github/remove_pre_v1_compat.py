from __future__ import annotations

from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text()


def write(path: str, text: str) -> None:
    (ROOT / path).write_text(text)


def replace(path: str, old: str, new: str, *, count: int | None = None) -> None:
    text = read(path)
    found = text.count(old)
    if count is not None and found != count:
        raise RuntimeError(f"{path}: expected {count} occurrences, found {found}: {old[:80]!r}")
    if count is None and found == 0:
        raise RuntimeError(f"{path}: pattern not found: {old[:80]!r}")
    write(path, text.replace(old, new))


def regex(path: str, pattern: str, replacement: str, *, count: int = 1, flags: int = re.S) -> None:
    text = read(path)
    updated, found = re.subn(pattern, replacement, text, count=count, flags=flags)
    if found != count:
        raise RuntimeError(f"{path}: expected {count} regex replacements, found {found}: {pattern[:100]!r}")
    write(path, updated)


def remove_line(path: str, line: str, *, count: int | None = None) -> None:
    replace(path, line, "", count=count)


# ---------------------------------------------------------------------------
# Canonical allocation lifecycle only: phase + health, durable ID + generation.
# ---------------------------------------------------------------------------
regex(
    "orchestrator/internal/lifecycle/lifecycle.go",
    r"\n// Legacy converts the pre-lifecycle status values.*?(?=\n// Diagnostic records details)",
    "\n",
)

path = "orchestrator/internal/lifecycle/lifecycle_test.go"
text = read(path)
marker = "func TestLegacyStatus"
if marker not in text:
    raise RuntimeError(f"{path}: {marker} not found")
write(path, text[: text.index(marker)].rstrip() + "\n")

# Wire structs stop carrying the old request/response fields.
remove_line(
    "orchestrator/internal/api/agent.go",
    '\tName          string                  `json:"name"`\n',
    count=1,
)
replace(
    "orchestrator/internal/api/server.go",
    '\tGeneration uint64           `json:"generation,omitempty"`\n',
    '\tGeneration uint64           `json:"generation"`\n',
    count=1,
)
replace(
    "orchestrator/internal/api/server.go",
    '\tPhase      lifecycle.Phase  `json:"phase,omitempty"`\n',
    '\tPhase      lifecycle.Phase  `json:"phase"`\n',
    count=1,
)
replace(
    "orchestrator/internal/api/server.go",
    '\tHealth     lifecycle.Health `json:"health,omitempty"`\n',
    '\tHealth     lifecycle.Health `json:"health"`\n',
    count=1,
)
remove_line(
    "orchestrator/internal/api/server.go",
    '\tStatus     string           `json:"status"`\n',
    count=1,
)
remove_line(
    "orchestrator/internal/api/server.go",
    '\tTask             string            `json:"task,omitempty"`\n',
    count=1,
)
remove_line(
    "orchestrator/internal/api/server.go",
    '\tStatus           string            `json:"status"`\n',
    count=1,
)

# Agent requests must use the current required identity/fencing fields.
path = "orchestrator/internal/agent/handler.go"
remove_line(path, '\t"crypto/sha256"\n', count=1)
remove_line(path, '\t"encoding/hex"\n', count=1)
replace(
    path,
    '''\tif request.AllocationID == "" {\n\t\trequest.AllocationID = request.Name\n\t}\n\tif request.Generation == 0 {\n\t\trequest.Generation = 1\n\t}\n\tif request.ExecutionHash == "" {\n\t\trequestCopy := request\n\t\trequestCopy.Epoch, requestCopy.ExecutionHash = 0, ""\n\t\traw, _ := json.Marshal(requestCopy)\n\t\tsum := sha256.Sum256(raw)\n\t\trequest.ExecutionHash = hex.EncodeToString(sum[:])\n\t}\n''',
    '''\tif request.AllocationID == "" {\n\t\treturn echo.NewHTTPError(http.StatusBadRequest, "allocation_id is required")\n\t}\n\tif request.Generation == 0 {\n\t\treturn echo.NewHTTPError(http.StatusBadRequest, "generation must be greater than zero")\n\t}\n\tif request.ExecutionHash == "" {\n\t\treturn echo.NewHTTPError(http.StatusBadRequest, "execution_hash is required")\n\t}\n''',
    count=1,
)
replace(path, '"allocation", request.Name', '"allocation", request.AllocationID', count=1)

# Current agents no longer manufacture the obsolete status projection.
path = "orchestrator/internal/agent/agent.go"
text = read(path)
old = 'actual = append(actual, api.AllocationStatus{ID: alloc.AllocationID, Generation: alloc.Generation, Task: alloc.TaskName, Phase: lifecycle.Phase(alloc.Status), Health: lifecycle.Health(alloc.Health), Status: lifecycle.CompatibilityStatus(lifecycle.Phase(alloc.Status), lifecycle.Health(alloc.Health)), Ports: ports})'
new = 'actual = append(actual, api.AllocationStatus{ID: alloc.AllocationID, Generation: alloc.Generation, Task: alloc.TaskName, Phase: lifecycle.Phase(alloc.Status), Health: lifecycle.Health(alloc.Health), Ports: ports})'
if text.count(old) != 1:
    raise RuntimeError(f"{path}: heartbeat compatibility projection not found exactly once")
write(path, text.replace(old, new))

# Server Allocation has one identity/revision/state representation.
path = "orchestrator/internal/server/server.go"
regex(
    path,
    r"\n// AllocationStatus is the legacy allocation state representation\..*?(?=\n// Allocation contains desired and observed allocation state\.)",
    "\n",
)
regex(
    path,
    r"// Allocation contains desired and observed allocation state\.\ntype Allocation struct \{.*?\n\}\n\n// AllocationID returns the stable allocation identifier\..*?(?=// Transition records a validated allocation phase change\.)",
    '''// Allocation contains desired and observed allocation state.\ntype Allocation struct {\n\tmu            sync.Mutex\n\tNamespace     string\n\tJobName       string\n\tTaskGroupName string\n\tID            string `json:"allocation_id"`\n\tGeneration    uint64 `json:"generation"`\n\tJobRevision   int    `json:"job_revision"`\n\tTasks         []spec.TaskSpec\n\tPhase         lifecycle.Phase  `json:"phase"`\n\tHealth        lifecycle.Health `json:"health"`\n\tlifecycle.Diagnostic\n\tNode  *Node\n\tPorts []api.PortMapping `json:"ports,omitempty"`\n\t// Draining marks an allocation whose job revision is superseded under\n\t// a rolling update strategy. Draining allocations are not restarted on\n\t// failure and are not counted toward the desired count.\n\tDraining bool `json:"draining,omitempty"`\n\t// Events is an in-memory ring buffer of recent phase transitions.\n\t// It is not persisted and resets on leader failover.\n\tEvents *lifecycle.RingBuffer `json:"-"`\n}\n\n''',
)
remove_line(path, '\ta.normalize(now)\n', count=1)
remove_line(path, '\ta.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, a.Health))\n', count=2)
regex(
    path,
    r"\n// UnmarshalJSON decodes allocation state with legacy compatibility\..*?(?=\n// NewServer constructs an orchestrator server\.)",
    "\n",
)

# Replace the remaining legacy-aware ID accessor calls in server code.
for server_path in [
    "orchestrator/internal/server/server.go",
    "orchestrator/internal/server/reconciler.go",
    "orchestrator/internal/server/allocations.go",
]:
    text = read(server_path)
    text = text.replace("alloc.AllocationID()", "alloc.ID")
    text = text.replace("allocation.AllocationID()", "allocation.ID")
    text = text.replace("a.AllocationID()", "a.ID")
    write(server_path, text)

# No compatibility initialization pass remains.
for server_path in [
    "orchestrator/internal/server/server.go",
    "orchestrator/internal/server/reconciler.go",
    "orchestrator/internal/server/allocations.go",
]:
    text = read(server_path)
    text = re.sub(r"^\s*[A-Za-z_][A-Za-z0-9_]*\.normalize\([^\n]*\)\n", "", text, flags=re.M)
    write(server_path, text)

# Heartbeats require the current volume and allocation-state shape.
path = "orchestrator/internal/server/server.go"
replace(
    path,
    "func (s *Server) Heartbeat(ctx context.Context, nodeID uuid.UUID, actual []api.AllocationStatus, version string, volumes ...[]string) error {",
    "func (s *Server) Heartbeat(ctx context.Context, nodeID uuid.UUID, actual []api.AllocationStatus, version string, volumes []string) error {",
    count=1,
)
replace(
    path,
    '''\tnode.Version = version\n\tif len(volumes) != 0 {\n\t\tnode.Volumes = append([]string(nil), volumes[0]...)\n\t}\n''',
    '''\tnode.Version = version\n\tnode.Volumes = append([]string(nil), volumes...)\n''',
    count=1,
)
replace(
    path,
    '''\tfor _, a := range actual {\n\t\tphase, health := a.Phase, a.Health\n\t\tif !phase.Valid() || !health.Valid() {\n\t\t\tphase, health = lifecycle.Legacy(a.Status)\n\t\t}\n\t\tinfo := statuses[a.ID]\n''',
    '''\tfor _, a := range actual {\n\t\tif !a.Phase.Valid() || !a.Health.Valid() {\n\t\t\treturn fmt.Errorf("invalid allocation state for %s: phase=%q health=%q", a.ID, a.Phase, a.Health)\n\t\t}\n\t\tphase, health := a.Phase, a.Health\n\t\tinfo := statuses[a.ID]\n''',
    count=1,
)
replace(
    path,
    "\t\tif !ok || (info.Generation != 0 && info.Generation != a.Generation) {\n",
    "\t\tif !ok || info.Generation != a.Generation {\n",
    count=1,
)

# Public allocation responses expose only phase/health and task-group identity.
for response_path in [
    "orchestrator/internal/server/server.go",
    "orchestrator/internal/server/allocations.go",
]:
    text = read(response_path)
    text = re.sub(r"^\s*Status:\s*string\([^\n]*\.Status\),\n", "", text, flags=re.M)
    write(response_path, text)

# Persist allocations by their canonical ID.
replace(
    "orchestrator/internal/server/state.go",
    'allocation.Name)',
    'allocation.ID)',
    count=1,
)

# Reconciler writes/sends only canonical allocation fields.
path = "orchestrator/internal/server/reconciler.go"
text = read(path)
text = text.replace("action.Allocation.Name", "action.Allocation.ID")
text = text.replace("allocation.Name", "allocation.ID")
text = text.replace("alloc.Name", "alloc.ID")
text = re.sub(r",\s*Name:\s*alloc\.ID", "", text)
write(path, text)

# Rewrite direct server Allocation literals that still use legacy fields.
def matching_brace(text: str, open_index: int) -> int:
    depth = 0
    quote: str | None = None
    escape = False
    line_comment = False
    block_comment = False
    i = open_index
    while i < len(text):
        c = text[i]
        n = text[i + 1] if i + 1 < len(text) else ""
        if line_comment:
            if c == "\n":
                line_comment = False
            i += 1
            continue
        if block_comment:
            if c == "*" and n == "/":
                block_comment = False
                i += 2
                continue
            i += 1
            continue
        if quote:
            if quote != "`" and escape:
                escape = False
            elif quote != "`" and c == "\\":
                escape = True
            elif c == quote:
                quote = None
            i += 1
            continue
        if c == "/" and n == "/":
            line_comment = True
            i += 2
            continue
        if c == "/" and n == "*":
            block_comment = True
            i += 2
            continue
        if c in {'"', "'", "`"}:
            quote = c
            i += 1
            continue
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return i
        i += 1
    raise RuntimeError("unmatched Allocation literal brace")


def split_top_level(content: str) -> list[str]:
    fields: list[str] = []
    start = 0
    brace = bracket = paren = 0
    quote: str | None = None
    escape = False
    line_comment = block_comment = False
    i = 0
    while i < len(content):
        c = content[i]
        n = content[i + 1] if i + 1 < len(content) else ""
        if line_comment:
            if c == "\n":
                line_comment = False
            i += 1
            continue
        if block_comment:
            if c == "*" and n == "/":
                block_comment = False
                i += 2
                continue
            i += 1
            continue
        if quote:
            if quote != "`" and escape:
                escape = False
            elif quote != "`" and c == "\\":
                escape = True
            elif c == quote:
                quote = None
            i += 1
            continue
        if c == "/" and n == "/":
            line_comment = True
            i += 2
            continue
        if c == "/" and n == "*":
            block_comment = True
            i += 2
            continue
        if c in {'"', "'", "`"}:
            quote = c
        elif c == "{":
            brace += 1
        elif c == "}":
            brace -= 1
        elif c == "[":
            bracket += 1
        elif c == "]":
            bracket -= 1
        elif c == "(":
            paren += 1
        elif c == ")":
            paren -= 1
        elif c == "," and brace == bracket == paren == 0:
            fields.append(content[start:i])
            start = i + 1
        i += 1
    if content[start:].strip():
        fields.append(content[start:])
    return fields


def field_key(fragment: str) -> str | None:
    match = re.match(r"\s*([A-Za-z_][A-Za-z0-9_]*)\s*:", fragment)
    return match.group(1) if match else None


def rewrite_allocation_literals(path: Path) -> None:
    text = path.read_text()
    starts = [m.start() for m in re.finditer(r"&Allocation\{", text)]
    for start in reversed(starts):
        open_index = text.index("{", start)
        close_index = matching_brace(text, open_index)
        content = text[open_index + 1 : close_index]
        fields = split_top_level(content)
        keys = {field_key(field) for field in fields}
        legacy = bool(keys & {"Name", "Revision", "Status", "Task"})
        if not legacy:
            continue
        rewritten: list[str] = []
        for field in fields:
            key = field_key(field)
            if key == "Name":
                if "ID" in keys:
                    continue
                field = re.sub(r"^(\s*)Name\s*:", r"\1ID:", field, count=1)
            elif key == "Revision":
                if "JobRevision" in keys:
                    continue
                field = re.sub(r"^(\s*)Revision\s*:", r"\1JobRevision:", field, count=1)
            elif key == "Status":
                value = field.split(":", 1)[1].strip()
                if "Phase" in keys or "Health" in keys:
                    continue
                mapping = {
                    "AllocationStatusHealthy": ("lifecycle.PhaseRunning", "lifecycle.HealthHealthy"),
                    "AllocationStatusUnhealthy": ("lifecycle.PhaseRunning", "lifecycle.HealthUnhealthy"),
                    "AllocationStatusPending": ("lifecycle.PhasePlaced", "lifecycle.HealthUnknown"),
                }
                if value not in mapping:
                    raise RuntimeError(f"{path}: unsupported legacy allocation status {value!r}")
                phase, health = mapping[value]
                indent = re.match(r"\s*", field).group(0)
                field = f"{indent}Phase: {phase}, Health: {health}"
            elif key == "Task":
                if "Tasks" in keys:
                    continue
                value = field.split(":", 1)[1].strip()
                if value == "nil":
                    continue
                if value.startswith("&spec.TaskSpec{"):
                    value = value[1:]
                    indent = re.match(r"\s*", field).group(0)
                    field = f"{indent}Tasks: []spec.TaskSpec{{{value}}}"
                else:
                    raise RuntimeError(f"{path}: cannot canonicalize Task field {value[:80]!r}")
            rewritten.append(field)
        new_keys = {field_key(field) for field in rewritten}
        indent_match = re.search(r"\n([ \t]*)[A-Za-z_]", content)
        indent = indent_match.group(1) if indent_match else ""
        if "Generation" not in new_keys:
            rewritten.append(f"\n{indent}Generation: 1")
        if "Phase" not in new_keys:
            rewritten.append(f"\n{indent}Phase: lifecycle.PhasePlaced")
        if "Health" not in new_keys:
            rewritten.append(f"\n{indent}Health: lifecycle.HealthUnknown")
        new_content = ",".join(rewritten)
        if content.rstrip().endswith(","):
            new_content += ","
        text = text[: open_index + 1] + new_content + text[close_index:]
    path.write_text(text)


for allocation_path in (ROOT / "orchestrator/internal/server").glob("*.go"):
    rewrite_allocation_literals(allocation_path)

# Inferred Allocation literals in the allocation-list fixture are the only old
# anonymous pointer literals in the package.
path = "orchestrator/internal/server/allocations_test.go"
replace(
    path,
    '{Namespace: "acme", JobName: "web", TaskGroupName: "frontend", Name: "acme-web-1", Status: AllocationStatusHealthy, Node: acmeNode}',
    '{Namespace: "acme", JobName: "web", TaskGroupName: "frontend", ID: "acme-web-1", Generation: 1, Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthHealthy, Node: acmeNode}',
    count=1,
)
replace(
    path,
    '{Namespace: "acme", JobName: "db", TaskGroupName: "primary", Name: "acme-db-1", Status: AllocationStatusHealthy, Node: acmeNode}',
    '{Namespace: "acme", JobName: "db", TaskGroupName: "primary", ID: "acme-db-1", Generation: 1, Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthHealthy, Node: acmeNode}',
    count=1,
)
replace(
    path,
    '{Namespace: "staging", JobName: "web", TaskGroupName: "frontend", Name: "staging-web-1", Status: AllocationStatusHealthy, Node: stagingNode}',
    '{Namespace: "staging", JobName: "web", TaskGroupName: "frontend", ID: "staging-web-1", Generation: 1, Phase: lifecycle.PhaseRunning, Health: lifecycle.HealthHealthy, Node: stagingNode}',
    count=1,
)

# Remove the migration-only lifecycle test entirely.
(ROOT / "orchestrator/internal/server/lifecycle_test.go").unlink()

# Scheduler accepts only the colocated task-group scheduling unit.
path = "orchestrator/internal/server/scheduler.go"
remove_line(path, '\t// Task is deprecated; Tasks represents the colocated scheduling unit.\n\tTask *spec.TaskSpec\n', count=1)
replace(
    path,
    '''\t\tif len(intent.Tasks) == 0 && intent.Task != nil && intent.Task.Resources != nil {\n\t\t\treqCPU = intent.Task.Resources.CPU\n\t\t\treqMemory = int64(intent.Task.Resources.Memory)\n\t\t}\n''',
    "",
    count=1,
)
replace(
    "orchestrator/internal/server/scheduler_test.go",
    'task := &spec.TaskSpec{Resources: &spec.ResourcesSpec{CPU: 750, Memory: 700}}\n\tplacements := Schedule(&PlacementIntent{Count: 4, Nodes: []*Node{a, b, draining}, Task: task})',
    'tasks := []spec.TaskSpec{{Resources: &spec.ResourcesSpec{CPU: 750, Memory: 700}}}\n\tplacements := Schedule(&PlacementIntent{Count: 4, Nodes: []*Node{a, b, draining}, Tasks: tasks})',
    count=1,
)

# RestartController was only a terminology bridge; use the canonical reconciler.
replace(
    "orchestrator/cmd/trellis/main.go",
    "agent.NewRestartController(runtimeClient, nil)",
    "agent.NewAllocationReconciler(runtimeClient, nil)",
    count=1,
)
(ROOT / "orchestrator/internal/agent/restart.go").unlink()

# ---------------------------------------------------------------------------
# API access mode accepts only the current explicit string enum.
# ---------------------------------------------------------------------------
path = "orchestrator/internal/spec/parse.go"
remove_line(path, "\tnormalizeLegacyAPIAccess(data)\n", count=1)
regex(path, r"\nfunc normalizeLegacyAPIAccess\(data map\[string\]interface\{\}\) \{.*?\n\}\n?$", "\n")

path = "orchestrator/internal/spec/types.go"
regex(
    path,
    r"\n// UnmarshalJSON accepts the canonical string modes and legacy booleans\..*?(?=\n// TaskNetworkMode controls)",
    "\n",
)

path = "orchestrator/internal/spec/api_access_test.go"
remove_line(path, '\t"encoding/json"\n', count=1)
remove_line(path, '\t\t{name: "legacy true", value: "true", want: APIAccessNamespace},\n', count=1)
remove_line(path, '\t\t{name: "legacy false", value: "false", want: APIAccessNone},\n', count=1)
regex(path, r"\nfunc TestAPIAccessJSONCompatibility\(t \*testing\.T\) \{.*?(?=\nfunc TestValidateRejectsInvalidAPIAccessMode)", "\n")

# ---------------------------------------------------------------------------
# CLI contexts and commands have one current shape/vocabulary.
# ---------------------------------------------------------------------------
path = "orchestrator/cmd/trellisctl/main.go"
replace(
    path,
    '''\tCurrentContext string                       `yaml:"current_context,omitempty"`\n\tContexts       map[string]contextFileConfig `yaml:"contexts,omitempty"`\n\n\t// Legacy flat fields remain supported as defaults beneath a selected context.\n\tServerAddr   *string `yaml:"server_addr,omitempty"`\n\tClusterToken *string `yaml:"cluster_token,omitempty"`\n\tNamespace    *string `yaml:"namespace,omitempty"`\n\tCACert       *string `yaml:"ca_cert,omitempty"`\n''',
    '''\tCurrentContext string                       `yaml:"current_context,omitempty"`\n\tContexts       map[string]contextFileConfig `yaml:"contexts,omitempty"`\n''',
    count=1,
)
replace(
    path,
    '''\t// 2. User config file. Legacy flat values are defaults; a selected named\n\t// context overlays them before environment variables and explicit flags.\n''',
    '''\t// 2. User config file. The selected named context is applied before\n\t// environment variables and explicit flags.\n''',
    count=1,
)
regex(
    path,
    r"\tif err == nil \{\n\t\tif file\.ServerAddr != nil \{.*?\n\n\t\tselected := file\.CurrentContext",
    "\tif err == nil {\n\t\tselected := file.CurrentContext",
)

path = "orchestrator/cmd/trellisctl/jobs.go"
remove_line(path, '\tvar task string\n', count=1)
replace(path, '\t\tUse:   "logs JOB_OR_ALLOCATION",\n', '\t\tUse:   "logs JOB",\n', count=1)
replace(
    path,
    '\t\tLong:  "Show logs for a job or allocation. For a job with multiple allocations, non-following output includes every matching allocation. Use --allocation with the short prefix shown by \'jobs status\' to select one allocation.",\n',
    '\t\tLong:  "Show logs for a job. For a job with multiple allocations, non-following output includes every matching allocation. Use --allocation with the short prefix shown by \'jobs status\' to select one allocation.",\n',
    count=1,
)
replace(
    path,
    'return runJobLogs(cmd.Context(), cmd.OutOrStdout(), serverClient, args[0], allocation, group, task, follow, tail)',
    'return runJobLogs(cmd.Context(), cmd.OutOrStdout(), serverClient, args[0], allocation, group, follow, tail)',
    count=1,
)
remove_line(path, '\tflags.StringVar(&task, "task", "", "Only allocations for this task")\n', count=1)
remove_line(path, '\t\tAliases: []string{"destroy"},\n', count=1)
replace(path, '"Allocation\\tTask group/Task\\tNode\\tLifecycle\\tHealth\\tDiagnostic"', '"Allocation\\tTask group\\tNode\\tLifecycle\\tHealth\\tDiagnostic"', count=1)
replace(
    path,
    'fmt.Fprintf(tw, "%s\\t%s/%s\\t%s\\t%s\\t%s\\t%s\\n", shortID(a.ID), a.Group, displayTask(a.Task), allocationNode(a), a.Phase, a.Health, diagnosticSummary(a))',
    'fmt.Fprintf(tw, "%s\\t%s\\t%s\\t%s\\t%s\\t%s\\n", shortID(a.ID), a.Group, allocationNode(a), a.Phase, a.Health, diagnosticSummary(a))',
    count=1,
)
replace(
    path,
    'fmt.Fprintf(w, "\\n- %s %s/%s on %s: lifecycle=%s health=%s\\n", shortID(a.ID), a.Group, displayTask(a.Task), allocationNode(a), a.Phase, a.Health)',
    'fmt.Fprintf(w, "\\n- %s %s on %s: lifecycle=%s health=%s\\n", shortID(a.ID), a.Group, allocationNode(a), a.Phase, a.Health)',
    count=1,
)
regex(path, r"\nfunc displayTask\(task string\) string \{.*?\n\}\n", "\n")

path = "orchestrator/cmd/trellisctl/jobs_runtime.go"
replace(
    path,
    "func runJobLogs(ctx context.Context, w io.Writer, serverClient *client.ServerClient, target, allocationRef, group, task string, follow bool, tail int) error {\n\tselected, err := resolveLogAllocations(ctx, serverClient, target, allocationRef, group, task)",
    "func runJobLogs(ctx context.Context, w io.Writer, serverClient *client.ServerClient, target, allocationRef, group string, follow bool, tail int) error {\n\tselected, err := resolveLogAllocations(ctx, serverClient, target, allocationRef, group)",
    count=1,
)
replace(
    path,
    'return fmt.Errorf("--follow requires exactly one allocation; select one with --allocation PREFIX, --group, or --task (matches: %s)", allocationRefs(selected))',
    'return fmt.Errorf("--follow requires exactly one allocation; select one with --allocation PREFIX or --group (matches: %s)", allocationRefs(selected))',
    count=1,
)
replace(
    path,
    'fmt.Fprintf(w, "==> %s %s/%s <==\\n", shortID(allocation.ID), allocation.Group, displayTask(allocation.Task))',
    'fmt.Fprintf(w, "==> %s %s <==\\n", shortID(allocation.ID), allocation.Group)',
    count=1,
)
regex(
    path,
    r"func resolveLogAllocations\(ctx context\.Context, serverClient \*client\.ServerClient, target, allocationRef, group, task string\) \(\[\]api\.AllocationResponse, error\) \{.*?\n\}\n\nfunc filterAllocations",
    '''func resolveLogAllocations(ctx context.Context, serverClient *client.ServerClient, target, allocationRef, group string) ([]api.AllocationResponse, error) {\n\tstatus, err := serverClient.GetJob(ctx, target)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tmatches := append([]api.AllocationResponse(nil), status.Allocations...)\n\tmatches = filterAllocations(matches, group)\n\tif allocationRef != "" {\n\t\tresolved, err := resolveAllocationPrefix(matches, allocationRef)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tmatches = []api.AllocationResponse{resolved}\n\t}\n\tif len(matches) == 0 {\n\t\treturn nil, fmt.Errorf("job %s has no allocations matching the requested filters", target)\n\t}\n\treturn matches, nil\n}\n\nfunc filterAllocations''',
)
replace(
    path,
    "func filterAllocations(allocations []api.AllocationResponse, group, task string) []api.AllocationResponse {",
    "func filterAllocations(allocations []api.AllocationResponse, group string) []api.AllocationResponse {",
    count=1,
)
regex(path, r"\n\t\tif task != \"\" && allocation\.Task != task \{\n\t\t\tcontinue\n\t\t\}", "")

# ---------------------------------------------------------------------------
# Dashboard consumes only the current allocation response.
# ---------------------------------------------------------------------------
path = "ui/src/lib/types.ts"
regex(
    path,
    r"export type AllocationStatus =\n  \| AllocationPhase\n  \| \"pending\"\n  \| \"healthy\"\n  \| \"unhealthy\";\n",
    "",
    flags=0,
)
remove_line(path, "  task: string;\n", count=1)
remove_line(path, "  status: AllocationStatus;\n", count=1)

path = "ui/src/components/allocations-table.tsx"
remove_line(path, '              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Task</th>\n', count=1)
remove_line(path, '                <td className="px-4 py-3 text-card-foreground">{alloc.task || "—"}</td>\n', count=1)
replace(path, "<StatusBadge status={alloc.phase ?? alloc.status} />", "<StatusBadge status={alloc.phase} />", count=1)
replace(path, '<StatusBadge status={alloc.health ?? "unknown"} />', '<StatusBadge status={alloc.health} />', count=1)

path = "ui/src/components/allocation-detail.tsx"
replace(path, "<StatusBadge status={allocation.phase ?? allocation.status} />", "<StatusBadge status={allocation.phase} />", count=1)
replace(path, '<StatusBadge status={allocation.health ?? "unknown"} />', '<StatusBadge status={allocation.health} />', count=1)
replace(path, 'String(allocation.generation ?? 1)', 'String(allocation.generation)', count=1)
remove_line(path, '              <Field label="Task" value={allocation.task || "—"} />\n', count=1)

replace(
    "ui/src/components/job-detail.tsx",
    '{allocation.group}/{allocation.task || "*"}',
    '{allocation.group}',
    count=1,
)
replace(
    "ui/src/lib/operations.ts",
    'const identity = `${allocation.group}/${allocation.task || "*"}`;',
    'const identity = allocation.group;',
    count=1,
)

# ---------------------------------------------------------------------------
# Internal state-store naming does not preserve a pre-V1 public name.
# ---------------------------------------------------------------------------
for go_path in (ROOT / "orchestrator").rglob("*.go"):
    text = go_path.read_text()
    if "StateStore" in text:
        go_path.write_text(text.replace("StateStore", "Store"))
path = "orchestrator/internal/state/interface.go"
text = read(path)
text = text.replace("//\n//nolint:revive // Store is retained as the established public interface name.\n", "")
write(path, text)

# ---------------------------------------------------------------------------
# Documentation describes only supported current inputs/verbs.
# ---------------------------------------------------------------------------
path = "docs/public/core-concepts.md"
replace(
    path,
    "An allocation can therefore be `running` and `unhealthy`. Older persisted state may still contain legacy status values for compatibility, but lifecycle and health are the canonical model.",
    "An allocation can therefore be `running` and `unhealthy`. Lifecycle and health are independent parts of the canonical allocation state.",
    count=1,
)

path = "examples/api-access/README.md"
remove_line(path, "For compatibility, old `api_access: true` manifests are interpreted as `namespace`, but new manifests should use the explicit mode.\n\n", count=1)

path = "docs/public/cli.md"
replace(path, "< legacy flat config values\n", "", count=1)
remove_line(path, "`jobs destroy` remains an alias for compatibility; documentation uses `delete`.\n\n", count=1)
replace(
    path,
    "The selected context itself comes from `current_context`, then `TRELLIS_CONTEXT`, then the explicit `--context` flag. This preserves script compatibility while making named contexts the convenient interactive path.",
    "The selected context itself comes from `current_context`, then `TRELLIS_CONTEXT`, then the explicit `--context` flag.",
    count=1,
)

path = "docs/public/operations.md"
text = read(path).replace("the compatibility/advanced command", "the advanced command")
write(path, text)

path = "docs/public/user-model.md"
text = read(path).replace(
    "Existing CLI aliases may remain for compatibility, but documentation and UI copy should prefer these terms.",
    "Documentation and UI copy use these canonical terms; any deliberate CLI aliases are convenience spellings rather than a compatibility layer.",
)
write(path, text)

# Remove any compatibility note about boolean api_access from the manifest reference.
path = "docs/public/job-specification.md"
text = read(path)
text = re.sub(r"\n[^\n]*`api_access: true`[^\n]*\n", "\n", text)
write(path, text)

# Final source assertions catch accidental half-migrations before compilation.
for forbidden in [
    "CompatibilityStatus(",
    "lifecycle.Legacy(",
    "AllocationStatusHealthy",
    "AllocationStatusUnhealthy",
    "AllocationStatusPending",
    "NewRestartController(",
    "RestartController",
    "RestartSubscriber",
    "normalizeLegacyAPIAccess",
]:
    hits = []
    for candidate in list((ROOT / "orchestrator").rglob("*.go")) + list((ROOT / "ui/src").rglob("*.ts")) + list((ROOT / "ui/src").rglob("*.tsx")):
        if forbidden in candidate.read_text():
            hits.append(str(candidate.relative_to(ROOT)))
    if hits:
        raise RuntimeError(f"forbidden compatibility symbol {forbidden!r} remains in {hits}")

print("pre-V1 compatibility cleanup applied")
