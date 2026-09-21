#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
patch_file="$script_dir/source.patch"

cd "$repo_root"

if ! git apply --reverse --check "$patch_file"; then
  printf '%s\n' '回滚安全检查失败：源码可能已有额外修改，未执行任何回滚。' >&2
  exit 1
fi

git apply --reverse "$patch_file"
printf '%s\n' '课节真实序号修复已回滚；repair-records 记录保留。'
git status --short
