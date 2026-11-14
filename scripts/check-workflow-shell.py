#!/usr/bin/env python3
"""check-workflow-shell.py — every toolchain step of a workflow runs in the dev shell.

A runner image ships its own Go (and node, python, …), so a step that calls a
tool outside the flake's dev shell silently uses the image's version. For each
step whose `run:` calls go, make, a tool the flake provides, or a script that
calls one (scripts/* are read and checked the same way), this checks that

  * its job has a step `uses: ./.github/actions/nix-shell` before it, and
  * its effective shell (step `shell:`, else job `defaults.run.shell`, else
    workflow `defaults.run.shell`) is `nix develop …`.

Comments never count, so a comment that mentions the action cannot satisfy
the check. Standard library only: harness.yml runs without a toolchain, so a
YAML library is not available. The parser follows indentation, which is all
the workflow syntax used here needs (block mappings, `- ` step lists, block
scalars for `run: |`).

Usage: check-workflow-shell.py <workflow.yml>...   Exit 0 ok, 1 violation.
"""
import re
import sys

# A command position: line start, after ; & | ( or $(, then any number of
# compound-command keywords (if ! go test …; then …), negations, VAR=value
# assignments and env/exec/time/command wrappers, then optionally bash/sh
# with flags (the script it runs is the command), then a path.
PREFIX = (
    r"(?:^|[;&|(]|\$\()\s*"
    r"(?:(?:if|elif|then|else|while|until|do|!|\{)\s+"
    r"|(?:env|exec|time|command|nice)(?:\s+-\S+)*\s+"
    r"|[A-Za-z_][A-Za-z0-9_]*=(?:\"[^\"]*\"|'[^']*'|\S*)\s+)*"
    r"(?:(?:bash|sh)(?:\s+-\S+)*\s+)?"
)
END = r"(?=[\s;&|)'\"]|$)"
# go, make and the tools of devShells.default in flake.nix. A tool named
# inside an echo or a heredoc is prose, not a call. Extend with flake.nix.
TOOL = re.compile(
    PREFIX + r"(?:\S*/)?"
    r"(go|make|golangci-lint|govulncheck|goimports|gremlins|oasdiff|gcov2lcov"
    r"|zizmor|ccsh|node|npm|npx|java)" + END,
    re.M,
)
# A repo script in command position. Its own text is checked for tool calls,
# so a script that calls go (coverage-gate.sh: go list) counts without a list
# of script names to keep in sync.
SCRIPT = re.compile(PREFIX + r"((?:\./)?scripts/[\w.-]+)" + END, re.M)
NIX_ACTION = "./.github/actions/nix-shell"


def parse(path):
    """Return (workflow default shell, {job: {"shell": str, "steps": [...]}})."""
    wf_shell = None
    jobs = {}
    stack = []  # (indent, key) of the enclosing mapping keys
    block = None  # (indent of the key, target dict) while inside a block scalar
    step = None

    with open(path, encoding="utf-8") as f:
        lines = f.read().splitlines()

    for raw in lines:
        indent = len(raw) - len(raw.lstrip(" "))
        text = raw.strip()
        if block is not None:
            if text == "" or indent > block[0]:
                block[1]["run"] += text + "\n"
                continue
            block = None
        if text == "" or text.startswith("#"):
            continue

        item = text.startswith("- ")
        if item:
            text = text[2:].lstrip()
            indent += 2
        m = re.match(r"([A-Za-z0-9_.-]+):(?:\s+(.*))?$", text)
        if not m:
            continue
        key, value = m.group(1), (m.group(2) or "").strip()
        if value.startswith("#"):
            value = ""

        while stack and stack[-1][0] >= indent:
            stack.pop()
        path_keys = [k for _, k in stack]

        # A new item in a job's steps list.
        if item and len(path_keys) == 3 and path_keys[0] == "jobs" and path_keys[2] == "steps":
            step = {"uses": "", "shell": None, "run": ""}
            jobs[path_keys[1]]["steps"].append(step)

        if path_keys == [] and key == "jobs":
            pass
        elif len(path_keys) == 1 and path_keys[0] == "jobs":
            jobs[key] = {"shell": None, "steps": []}
        elif path_keys == ["defaults", "run"] and key == "shell":
            wf_shell = value
        elif len(path_keys) == 4 and path_keys[0] == "jobs" and path_keys[2:] == ["defaults", "run"] and key == "shell":
            jobs[path_keys[1]]["shell"] = value
        elif len(path_keys) == 3 and path_keys[0] == "jobs" and path_keys[2] == "steps" and step is not None:
            if key == "uses":
                step["uses"] = value.strip("'\"")
            elif key == "shell":
                step["shell"] = value
            elif key == "run":
                if value[:1] in ("|", ">"):
                    block = (indent, step)
                else:
                    step["run"] += unquote(value) + "\n"

        stack.append((indent, key))

    return wf_shell, jobs


def strip_comments(text):
    return "\n".join(l for l in text.splitlines() if not l.lstrip().startswith("#"))


def strip_quoted(text):
    """Drop quoted literals from a script, so a grep pattern like
    "(go|node)" is not read as a call. Single quotes never run anything; a
    double-quoted string is kept when it holds a $(…) command substitution."""
    text = re.sub(r"'[^'\n]*'", "''", text)
    return re.sub(r'"(?:[^"\\\n]|\\.)*"', lambda m: m.group(0) if "$(" in m.group(0) else '""', text)


def calls_tool(text, seen=None, script=False):
    """True if text (a run: body, or a script with script=True) calls a
    toolchain command, directly or through a repo script it runs."""
    seen = set() if seen is None else seen
    text = strip_comments(text)
    if script:
        text = strip_quoted(text)
    if TOOL.search(text):
        return True
    for m in SCRIPT.finditer(text):
        script = m.group(1).removeprefix("./")
        if script in seen:
            continue
        seen.add(script)
        try:
            with open(script, encoding="utf-8") as f:
                if calls_tool(f.read(), seen, script=True):
                    return True
        except OSError:
            continue
    return False


def unquote(value):
    """Decode a single-line YAML scalar that is quoted as a whole."""
    if len(value) >= 2 and value[0] == value[-1] == "'":
        return value[1:-1].replace("''", "'")
    if len(value) >= 2 and value[0] == value[-1] == '"':
        return value[1:-1].replace('\\"', '"').replace("\\\\", "\\")
    return value


def check(path):
    wf_shell, jobs = parse(path)
    errors = []
    for job, spec in jobs.items():
        have_nix = False
        for n, step in enumerate(spec["steps"], 1):
            if step["uses"] == NIX_ACTION:
                have_nix = True
            if not calls_tool(step["run"]):
                continue
            shell = step["shell"] or spec["shell"] or wf_shell or "bash"
            if not have_nix:
                errors.append(f"{path}: job {job}, step {n}: runs a toolchain command without a prior {NIX_ACTION} step")
            if not shell.startswith("nix develop"):
                errors.append(f"{path}: job {job}, step {n}: runs a toolchain command in shell '{shell}', not nix develop")
    return errors


def main(paths):
    errors = [e for p in paths for e in check(p)]
    for e in errors:
        print(e)
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
