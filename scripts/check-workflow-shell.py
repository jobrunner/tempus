#!/usr/bin/env python3
"""check-workflow-shell.py — every toolchain step of a workflow runs in the dev shell.

A runner image ships its own Go (and node, python, …), so a step that calls a
tool outside the flake's dev shell silently uses the image's version. For each
step whose `run:` calls go, make, a tool the flake provides, or a script that
calls one, this checks that

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

# go, make, the tools of devShells.default in flake.nix, and the repo scripts
# that call them, in command position (line start, after ; & | ( or $(, or as
# the script of bash/sh), with an optional path. A tool named inside an echo
# or a heredoc is prose, not a call. Extend the list together with flake.nix.
TOOL = re.compile(
    r"(?:^|[;&|(]|\$\()\s*(?:(?:bash|sh)\s+)?(?:\S*/)?"
    r"(go|make|golangci-lint|govulncheck|goimports|gremlins|oasdiff|gcov2lcov"
    r"|zizmor|ccsh|node|npm|npx|java"
    r"|mutation-gate\.sh|vuln-gate\.sh|vuln-autofix\.sh)"
    r"(?=[\s;&|)]|$)",
    re.M,
)
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
                    step["run"] += value + "\n"

        stack.append((indent, key))

    return wf_shell, jobs


def check(path):
    wf_shell, jobs = parse(path)
    errors = []
    for job, spec in jobs.items():
        have_nix = False
        for n, step in enumerate(spec["steps"], 1):
            if step["uses"] == NIX_ACTION:
                have_nix = True
            if not TOOL.search(step["run"]):
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
