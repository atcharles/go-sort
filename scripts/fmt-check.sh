#!/bin/sh
# 在临时副本中格式化，避免检查失败时修改工作区。
set -eu
check_dir=$(mktemp -d)
trap 'rm -rf "$check_dir"' EXIT HUP INT TERM
tar -cf - --exclude=./.git --exclude=./.agent-bus --exclude=./.codex --exclude=./vendor --exclude=./bin . | tar -xf - -C "$check_dir"
make -C "$check_dir" fmt
find . -type d \( -name .git -o -name .agent-bus -o -name .codex -o -name vendor -o -name bin -o -name testdata \) -prune -o -type f -name '*.go' -print > "$check_dir/source-list"
while IFS= read -r source; do
    diff -u "$source" "$check_dir/$source"
done < "$check_dir/source-list"
