#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
patch_file="$script_dir/source.patch"
backup_root="/Users/TT/Downloads/tal_downloader-release-assets-backup-20260922"
repo="TT3301/tal_downloader"

cd "$repo_root"

if ! git apply --reverse --check "$patch_file"; then
  printf '%s\n' '回滚安全检查失败：工作流可能已有额外修改，未执行任何回滚。' >&2
  exit 1
fi

for required in \
  "$backup_root/v3.2.0/tal_downloader_macos_arm64_v3.2.0.zip" \
  "$backup_root/v3.2.0/tal_downloader_win64_v3.2.0.exe" \
  "$backup_root/v3.2.1/tal_downloader_linux_amd64_v3.2.1" \
  "$backup_root/v3.2.1/tal_downloader_win64_dc75f9a.exe" \
  "$backup_root/v3.2.1/tal_downloader_win64_v3.2.1.exe" \
  "$backup_root/v3.2.2/tal_downloader_linux_amd64_v3.2.2" \
  "$backup_root/v3.2.2/tal_downloader_win64_5e32ad5.exe" \
  "$backup_root/v3.2.2/tal_downloader_win64_v3.2.2.exe" \
  "$backup_root/v3.2.4/tal_downloader_android_arm64_v3.2.4.apk" \
  "$backup_root/v3.2.4/tal_downloader_android_arm_v3.2.4.apk" \
  "$backup_root/v3.2.4/tal_downloader_linux_amd64_v3.2.4" \
  "$backup_root/v3.2.4/tal_downloader_macos_arm64_v3.2.4.zip" \
  "$backup_root/v3.2.4/tal_downloader_win64_v3.2.4.exe"
do
  if [ ! -f "$required" ]; then
    printf '回滚所需备份不存在：%s\n' "$required" >&2
    exit 1
  fi
done

git apply --reverse "$patch_file"

upload() {
  tag=$1
  file=$2
  gh release upload "$tag" "$backup_root/$file" --repo "$repo" --clobber
}

upload latest v3.2.0/tal_downloader_macos_arm64_v3.2.0.zip
upload latest v3.2.0/tal_downloader_win64_v3.2.0.exe
upload v3.2.1 v3.2.1/tal_downloader_linux_amd64_v3.2.1
upload v3.2.1 v3.2.1/tal_downloader_win64_dc75f9a.exe
upload v3.2.1 v3.2.1/tal_downloader_win64_v3.2.1.exe
upload v3.2.2 v3.2.2/tal_downloader_linux_amd64_v3.2.2
upload v3.2.2 v3.2.2/tal_downloader_win64_5e32ad5.exe
upload v3.2.2 v3.2.2/tal_downloader_win64_v3.2.2.exe
upload v3.2.4 v3.2.4/tal_downloader_android_arm64_v3.2.4.apk
upload v3.2.4 v3.2.4/tal_downloader_android_arm_v3.2.4.apk
upload v3.2.4 v3.2.4/tal_downloader_linux_amd64_v3.2.4
upload v3.2.4 v3.2.4/tal_downloader_macos_arm64_v3.2.4.zip
upload v3.2.4 v3.2.4/tal_downloader_win64_v3.2.4.exe

printf '%s\n' '工作流补丁已回滚，Release 资产已从本地备份恢复。'
git status --short
