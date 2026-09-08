"""Plan and publish immutable native releases using the published SemVer ledger."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

STABLE = re.compile(r"^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$")
SHA = re.compile(r"^[0-9a-f]{40}$")


def command(*args):
    return subprocess.check_output(args, text=True).strip()


def gh_json(*args):
    return json.loads(command("gh", *args))


def version_tuple(tag):
    match = STABLE.fullmatch(tag)
    return tuple(map(int, match.groups())) if match else None


def bump_kind(messages, labels=()):
    level = 0  # Untyped fixes, maintenance and cleanup default to patch.
    for message in messages:
        if re.search(r"(?m)^[a-z]+(?:\([^\n]*\))?!:\s|^BREAKING[ -]CHANGE:\s", message):
            level = max(level, 2)
        elif re.search(r"(?m)^feat(?:\([^\n]*\))?:\s", message):
            level = max(level, 1)
    for label in labels:
        if label in {"semver:major", "semver:minor", "semver:patch"}:
            level = max(level, {"semver:major": 2, "semver:minor": 1, "semver:patch": 0}[label])
    return ("patch", "minor", "major")[level]


def increment(version, kind):
    major, minor, patch = version
    return {"major": (major + 1, 0, 0), "minor": (major, minor + 1, 0),
            "patch": (major, minor, patch + 1)}[kind]


def plan_release(event, releases, repo):
    pr = event["pull_request"]
    if not pr.get("merged") or pr["base"]["ref"] != "main":
        raise ValueError("Only merged PRs into main can publish")
    if pr["head"]["repo"]["full_name"] != repo or pr["base"]["repo"]["full_name"] != repo:
        raise ValueError("Release PR must come from this repository")
    sha = pr["merge_commit_sha"]
    if not SHA.fullmatch(sha) or command("git", "rev-parse", "HEAD") != sha:
        raise ValueError("Checkout must be the exact PR merge commit")
    stable = [r for r in releases if not r["draft"] and not r["prerelease"] and version_tuple(r["tag_name"])]
    stable.sort(key=lambda r: version_tuple(r["tag_name"]), reverse=True)
    for release in stable:
        target = command("git", "rev-parse", release["tag_name"] + "^{commit}")
        if target == sha:
            return {"version": release["tag_name"][1:], "sha": sha, "published": True, "previous": "", "kind": "existing"}
    previous = stable[0]["tag_name"] if stable else ""
    if previous:
        # Prevent an old queued/retried merge from becoming a newer release.
        subprocess.run(["git", "merge-base", "--is-ancestor", previous, sha], check=True)
    revision = f"{previous}..{sha}" if previous else sha
    messages = command("git", "log", "--format=%B%x00", revision).split("\x00")
    messages.append(pr.get("title", ""))
    messages.append(pr.get("body") or "")
    kind = bump_kind(messages, [label["name"] for label in pr.get("labels", [])])
    base = version_tuple(previous) if previous else (0, 0, 0)
    version = ".".join(map(str, increment(base, kind)))
    drafts = [r for r in releases if r["draft"] and r["target_commitish"] == sha and version_tuple(r["tag_name"])]
    if len(drafts) > 1:
        raise ValueError("Multiple release drafts refer to this commit; reconcile first")
    if drafts:
        if previous and version_tuple(drafts[0]["tag_name"]) <= version_tuple(previous):
            raise ValueError("Draft is older than the published version")
        version = drafts[0]["tag_name"][1:]  # Retry the same version, never bump again.
    tag = "v" + version
    if tag in command("git", "tag", "--list", tag).splitlines():
        if command("git", "rev-parse", tag + "^{commit}") != sha:
            raise ValueError("Calculated version already belongs to another commit")
    return {"version": version, "sha": sha, "published": False, "previous": previous, "kind": kind}


def release_by_tag(repo, tag):
    # The tags endpoint excludes draft releases. List with authenticated access.
    pages = gh_json("api", "--paginate", "--slurp", f"repos/{repo}/releases?per_page=100")
    return next(release for page in pages for release in page if release["tag_name"] == tag)


def publish(plan, releases, repo):
    tag, sha = "v" + plan["version"], plan["sha"]
    if command("git", "rev-parse", "HEAD") != sha:
        raise ValueError("Publishing checkout differs from the tested merge commit")
    existing = next((r for r in releases if r["tag_name"] == tag), None)
    if existing and not existing["draft"]:
        if command("git", "rev-parse", tag + "^{commit}") != sha:
            raise ValueError("Published tag belongs to another commit")
        print("Release already published; preserving immutable assets")
        return
    if existing and existing["target_commitish"] != sha:
        raise ValueError("Draft belongs to another commit")
    if not existing:
        args = ["api", "--method", "POST", f"repos/{repo}/releases/generate-notes",
                "-f", f"tag_name={tag}", "-f", f"target_commitish={sha}"]
        if plan["previous"]:
            args += ["-f", f"previous_tag_name={plan['previous']}"]
        generated = gh_json(*args)
        notes = generated.get("body", "") + "\n\n" + Path("RELEASE-NOTES.md").read_text()
        Path("release-notes.md").write_text(notes)
        command("gh", "release", "create", tag, "--repo", repo, "--target", sha,
                "--draft", "--title", f"OStrm Go {plan['version']}", "--notes-file", "release-notes.md")
    release = release_by_tag(repo, tag)
    uploaded = {asset["name"]: asset for asset in release["assets"]}
    files = sorted(p for p in Path("dist").iterdir() if p.is_file())
    # Check every existing asset before uploading any missing one. No --clobber.
    for file in files:
        if file.name in uploaded:
            digest = "sha256:" + hashlib.sha256(file.read_bytes()).hexdigest()
            if uploaded[file.name].get("digest") != digest:
                raise ValueError(f"Draft asset differs: {file.name}; reuse original build artifacts")
    for file in files:
        if file.name not in uploaded:
            command("gh", "release", "upload", tag, str(file), "--repo", repo)
    final = release_by_tag(repo, tag)
    if {a["name"] for a in final["assets"]} != {f.name for f in files}:
        raise ValueError("Release assets do not match the verified build")
    # No newer stable release may be replaced as latest by a retry.
    latest = [version_tuple(r["tag_name"]) for r in releases
              if not r["draft"] and not r["prerelease"] and version_tuple(r["tag_name"])]
    if latest and max(latest) >= version_tuple(tag):
        raise ValueError("A same or newer stable release already exists")
    command("gh", "release", "edit", tag, "--repo", repo, "--draft=false", "--prerelease=false", "--latest")
    print(f"Published {tag} at {sha}")


def main():
    repo = os.environ["GITHUB_REPOSITORY"]
    pages = gh_json("api", "--paginate", "--slurp", f"repos/{repo}/releases?per_page=100")
    releases = [release for page in pages for release in page]
    if sys.argv[1] == "plan":
        result = plan_release(json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text()), releases, repo)
        print(json.dumps(result))
        Path("release-plan.json").write_text(json.dumps(result))
        if os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a") as output:
                for key in ("version", "sha", "previous", "kind", "published"):
                    value = str(result[key]).lower() if isinstance(result[key], bool) else result[key]
                    output.write(f"{key}={value}\n")
    elif sys.argv[1] == "publish":
        publish(json.loads(Path("release-plan.json").read_text()), releases, repo)
    else:
        raise ValueError("Expected plan or publish")


if __name__ == "__main__":
    main()
