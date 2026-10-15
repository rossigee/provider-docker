#!/bin/bash
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

if [[ $# -ne 1 ]]; then
    printf 'Usage: %s vMAJOR.MINOR.PATCH\n' "$0" >&2
    exit 1
fi

version=$1
if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    printf 'Invalid version: %s\n' "$version" >&2
    exit 1
fi

git fetch origin master

if [[ -n "$(git status --porcelain)" ]]; then
    printf 'Worktree is not clean.\n' >&2
    exit 1
fi

if [[ "$(git rev-parse HEAD)" != "$(git rev-parse origin/master)" ]]; then
    printf 'HEAD must match origin/master.\n' >&2
    exit 1
fi

if [[ -f VERSION ]]; then
    version_file=$(<VERSION)
    version_file="${version_file#"${version_file%%[![:space:]]*}"}"
    version_file="${version_file%"${version_file##*[![:space:]]}"}"
    if [[ $version_file != "$version" ]]; then
        printf 'VERSION must contain %s.\n' "$version" >&2
        exit 1
    fi
fi

if git show-ref --verify --quiet "refs/tags/$version"; then
    printf 'Local tag already exists: %s\n' "$version" >&2
    exit 1
fi

remote_tags=$(git ls-remote --tags origin "refs/tags/$version")
if [[ -n $remote_tags ]]; then
    printf 'Remote tag already exists: %s\n' "$version" >&2
    exit 1
fi

git tag -a "$version" -m "Release $version" HEAD
git push origin "refs/tags/$version"
