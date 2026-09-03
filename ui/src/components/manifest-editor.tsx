"use client";

import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import { parseJobManifest } from "@/lib/manifest";
import {
  getManifestSchema,
  manifestCompletions,
  manifestContainerKeys,
  manifestValueCompletions,
  validateManifestShape,
  type ManifestSchema,
} from "@/lib/manifest-schema";

interface ManifestEditorProps {
  value: string;
  onChange: (value: string) => void;
  onDirty?: () => void;
}

interface ValueTarget {
  key: string;
  token: string;
}

interface EditorCompletion {
  label: string;
  insert: string;
  description: string;
  kind: "key" | "value";
}

interface EditorDiagnostic {
  message: string;
  line: number | null;
}

function currentContext(
  source: string,
  cursor: number,
  containerKeys: Set<string>,
): string {
  const before = source.slice(0, cursor);
  const lines = before.split("\n");
  const current = lines.at(-1) ?? "";
  const indent = current.match(/^\s*/)?.[0].length ?? 0;
  const stack: Array<{ indent: number; key: string }> = [];

  for (const line of lines.slice(0, -1)) {
    const match = line.match(/^(\s*)(?:-\s*)?([A-Za-z_][A-Za-z0-9_-]*):\s*(?:#.*)?$/);
    if (!match || !containerKeys.has(match[2])) continue;
    const lineIndent = match[1].length;
    while (stack.length > 0 && stack.at(-1)!.indent >= lineIndent) stack.pop();
    stack.push({ indent: lineIndent, key: match[2] });
  }

  while (stack.length > 0 && stack.at(-1)!.indent >= indent) stack.pop();
  return stack.at(-1)?.key ?? "root";
}

function currentToken(source: string, cursor: number): string {
  const line = source.slice(0, cursor).split("\n").at(-1) ?? "";
  const match = line.match(/(?:^|\s)([A-Za-z_][A-Za-z0-9_-]*)$/);
  return match?.[1] ?? "";
}

function currentValueTarget(source: string, cursor: number): ValueTarget | null {
  const line = source.slice(0, cursor).split("\n").at(-1) ?? "";
  const match = line.match(
    /^\s*(?:-\s*)?([A-Za-z_][A-Za-z0-9_-]*):\s*([A-Za-z0-9_./-]*)$/,
  );
  if (!match) return null;
  return { key: match[1], token: match[2] };
}

function schemaDiagnosticLine(source: string, message: string): number | null {
  const path = message.match(/^manifest(?:\.[A-Za-z_][A-Za-z0-9_-]*|\[\d+\])*/)?.[0];
  if (!path) return null;
  const lines = manifestPathLines(source);
  let candidate = path;
  while (candidate !== "manifest") {
    const line = lines.get(candidate);
    if (line !== undefined) return line;
    candidate = candidate.replace(/(?:\.[A-Za-z_][A-Za-z0-9_-]*|\[\d+\])$/, "");
  }
  return lines.get("manifest") ?? null;
}

function manifestPathLines(source: string): Map<string, number> {
  const result = new Map<string, number>([["manifest", 1]]);
  const stack: Array<{ indent: number; path: string }> = [
    { indent: -1, path: "manifest" },
  ];
  const indexes = new Map<string, number>();

  source.split("\n").forEach((raw, index) => {
    const trimmed = raw.trim();
    if (trimmed === "" || trimmed.startsWith("#")) return;
    const indent = raw.match(/^\s*/)?.[0].length ?? 0;
    while (stack.length > 1 && stack.at(-1)!.indent >= indent) stack.pop();
    const parent = stack.at(-1)!.path;
    const line = index + 1;

    if (trimmed.startsWith("-")) {
      const itemIndex = indexes.get(parent) ?? 0;
      indexes.set(parent, itemIndex + 1);
      const itemPath = `${parent}[${itemIndex}]`;
      result.set(itemPath, line);
      stack.push({ indent, path: itemPath });

      const rest = trimmed.slice(1).trim();
      const match = rest.match(/^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$/);
      if (!match) return;
      const keyPath = `${itemPath}.${match[1]}`;
      result.set(keyPath, line);
      if (match[2] === "" || match[2].startsWith("#")) {
        stack.push({ indent: indent + 2, path: keyPath });
      }
      return;
    }

    const match = trimmed.match(/^([A-Za-z_][A-Za-z0-9_-]*):\s*(.*)$/);
    if (!match) return;
    const keyPath = `${parent}.${match[1]}`;
    result.set(keyPath, line);
    if (match[2] === "" || match[2].startsWith("#")) {
      stack.push({ indent, path: keyPath });
    }
  });

  return result;
}

export function ManifestEditor({ value, onChange, onDirty }: ManifestEditorProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const gutterRef = useRef<HTMLDivElement>(null);
  const [cursor, setCursor] = useState(0);
  const [completionOpen, setCompletionOpen] = useState(false);
  const [selectedCompletion, setSelectedCompletion] = useState(0);
  const [schema, setSchema] = useState<ManifestSchema | null>(null);

  useEffect(() => {
    let cancelled = false;
    void getManifestSchema()
      .then((loaded) => {
        if (!cancelled) setSchema(loaded);
      })
      .catch(() => {
        // YAML parsing and server validation remain available if schema assistance cannot load.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const lineCount = Math.max(1, value.split("\n").length);
  const containerKeys = useMemo(
    () => (schema ? manifestContainerKeys(schema) : new Set<string>()),
    [schema],
  );
  const context = currentContext(value, cursor, containerKeys);
  const keyToken = currentToken(value, cursor).toLowerCase();
  const valueTarget = currentValueTarget(value, cursor);
  const valueOptions = useMemo(
    () =>
      schema && valueTarget
        ? manifestValueCompletions(schema, context, valueTarget.key).filter((item) =>
            item.label.toLowerCase().startsWith(valueTarget.token.toLowerCase()),
          )
        : [],
    [context, schema, valueTarget],
  );
  const options = useMemo<EditorCompletion[]>(() => {
    if (valueTarget && valueOptions.length > 0) {
      return valueOptions.map((item) => ({
        label: item.label,
        insert: item.value,
        description: item.description,
        kind: "value",
      }));
    }
    return (schema ? manifestCompletions(schema, context) : [])
      .filter((item) => item.key.toLowerCase().startsWith(keyToken))
      .map((item) => ({
        label: item.key,
        insert: `${item.key}: `,
        description: item.description,
        kind: "key",
      }));
  }, [context, keyToken, schema, valueOptions, valueTarget]);

  const diagnostic = useMemo<EditorDiagnostic | null>(() => {
    try {
      const parsed = parseJobManifest(value);
      const message = schema ? validateManifestShape(schema, parsed) : null;
      return message
        ? { message, line: schemaDiagnosticLine(value, message) }
        : null;
    } catch (error) {
      const message = error instanceof Error ? error.message : "Invalid YAML";
      const marked = error as { mark?: { line?: number } };
      const line =
        typeof marked.mark?.line === "number" ? marked.mark.line + 1 : null;
      return { message, line };
    }
  }, [schema, value]);

  const syncCursor = () => {
    setCursor(textareaRef.current?.selectionStart ?? 0);
  };

  const applyCompletion = (completion: EditorCompletion) => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    const end = textarea.selectionStart;
    const target = currentValueTarget(value, end);
    const replaceLength =
      completion.kind === "value" && target
        ? target.token.length
        : currentToken(value, end).length;
    const start = end - replaceLength;
    const next = `${value.slice(0, start)}${completion.insert}${value.slice(end)}`;
    const nextCursor = start + completion.insert.length;
    onChange(next);
    onDirty?.();
    setCompletionOpen(false);
    requestAnimationFrame(() => {
      textarea.focus();
      textarea.setSelectionRange(nextCursor, nextCursor);
      setCursor(nextCursor);
    });
  };

  const insertText = (start: number, end: number, text: string, nextCursor: number) => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    onChange(`${value.slice(0, start)}${text}${value.slice(end)}`);
    onDirty?.();
    requestAnimationFrame(() => {
      textarea.focus();
      textarea.setSelectionRange(nextCursor, nextCursor);
      setCursor(nextCursor);
    });
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    const textarea = event.currentTarget;

    if ((event.ctrlKey || event.metaKey) && event.key === " ") {
      event.preventDefault();
      setSelectedCompletion(0);
      setCompletionOpen(true);
      return;
    }

    if (completionOpen && options.length > 0) {
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const delta = event.key === "ArrowDown" ? 1 : -1;
        setSelectedCompletion((index) => (index + delta + options.length) % options.length);
        return;
      }
      if (event.key === "Enter" || event.key === "Tab") {
        event.preventDefault();
        applyCompletion(options[Math.min(selectedCompletion, options.length - 1)]);
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        setCompletionOpen(false);
        return;
      }
    }

    if (event.key === "Tab") {
      event.preventDefault();
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      insertText(start, end, "  ", start + 2);
      return;
    }

    if (event.key === "Enter" && textarea.selectionStart === textarea.selectionEnd) {
      event.preventDefault();
      const start = textarea.selectionStart;
      const line = value.slice(0, start).split("\n").at(-1) ?? "";
      const indent = line.match(/^\s*/)?.[0] ?? "";
      const extra = /:\s*(?:#.*)?$/.test(line) ? "  " : "";
      const text = `\n${indent}${extra}`;
      insertText(start, start, text, start + text.length);
    }
  };

  return (
    <div className="overflow-hidden rounded-lg border border-border bg-zinc-950 focus-within:ring-2 focus-within:ring-emerald-500/40">
      <div className="flex items-center justify-between border-b border-zinc-800 px-3 py-2 text-[11px] text-zinc-400">
        <div className="flex items-center gap-3">
          <span className="font-medium text-zinc-200">YAML</span>
          <span>Ctrl/⌘ + Space for manifest keys and values</span>
        </div>
        <span className={diagnostic ? "text-amber-400" : "text-emerald-400"}>
          {diagnostic
            ? diagnostic.line
              ? `Line ${diagnostic.line} needs attention`
              : "Manifest needs attention"
            : schema
              ? "Schema valid"
              : "YAML parsed"}
        </span>
      </div>

      <div className="relative flex min-h-[520px] max-h-[70vh]">
        <div
          ref={gutterRef}
          aria-hidden="true"
          className="w-12 shrink-0 overflow-hidden border-r border-zinc-800 bg-zinc-900/70 py-4 text-right font-mono text-xs leading-5 text-zinc-600"
        >
          {Array.from({ length: lineCount }, (_, index) => (
            <div
              key={index}
              className={`h-5 pr-3 ${diagnostic?.line === index + 1 ? "bg-amber-500/10 text-amber-300" : ""}`}
            >
              {index + 1}
            </div>
          ))}
        </div>
        <textarea
          ref={textareaRef}
          id="job-manifest"
          value={value}
          onChange={(event) => {
            onChange(event.target.value);
            onDirty?.();
            setCompletionOpen(false);
          }}
          onClick={syncCursor}
          onKeyUp={syncCursor}
          onSelect={syncCursor}
          onKeyDown={handleKeyDown}
          onScroll={(event) => {
            if (gutterRef.current) gutterRef.current.scrollTop = event.currentTarget.scrollTop;
          }}
          spellCheck={false}
          aria-describedby="manifest-editor-help manifest-editor-diagnostic"
          className="min-h-[520px] flex-1 resize-none overflow-auto bg-zinc-950 p-4 font-mono text-xs leading-5 text-zinc-100 outline-none"
        />

        {completionOpen && (
          <div className="absolute bottom-3 left-14 right-3 z-10 max-h-56 overflow-auto rounded-md border border-zinc-700 bg-zinc-900 p-1 shadow-xl">
            {options.length === 0 ? (
              <p className="px-3 py-2 text-xs text-zinc-400">No schema completions match here.</p>
            ) : (
              options.map((item, index) => (
                <button
                  key={`${item.kind}-${item.label}`}
                  type="button"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => applyCompletion(item)}
                  className={`flex w-full items-start gap-3 rounded px-3 py-2 text-left ${
                    index === selectedCompletion ? "bg-zinc-800" : "hover:bg-zinc-800/70"
                  }`}
                >
                  <span className="w-28 shrink-0 font-mono text-xs text-emerald-400">{item.label}</span>
                  <span className="text-xs text-zinc-400">{item.description}</span>
                </button>
              ))
            )}
          </div>
        )}
      </div>

      <div className="border-t border-zinc-800 px-3 py-2 text-[11px] text-zinc-400">
        <p id="manifest-editor-help">Tab inserts two spaces; Enter preserves YAML indentation. Completion and structural diagnostics come from the generated authoring schema; Trellis validation and planning remain authoritative.</p>
        {diagnostic && (
          <p
            id="manifest-editor-diagnostic"
            className="mt-1 text-amber-400"
            title={diagnostic.message}
          >
            {diagnostic.line ? `Line ${diagnostic.line}: ` : ""}{diagnostic.message}
          </p>
        )}
      </div>
    </div>
  );
}
