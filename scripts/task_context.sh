#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/task_context.sh write \
    --base-branch <base-branch> \
    --base-worktree <base-worktree-path> \
    --task-branch <task-branch> \
    --task-worktree <task-worktree-path> \
    --slug <slug>
EOF
}

require_arg() {
  local name="$1"
  local value="${2:-}"
  if [[ -z "$value" ]]; then
    echo "missing required argument: $name" >&2
    exit 1
  fi
}

write_context() {
  local base_branch=""
  local base_worktree=""
  local task_branch=""
  local task_worktree=""
  local slug=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --base-branch)
        base_branch="${2:-}"
        shift 2
        ;;
      --base-worktree)
        base_worktree="${2:-}"
        shift 2
        ;;
      --task-branch)
        task_branch="${2:-}"
        shift 2
        ;;
      --task-worktree)
        task_worktree="${2:-}"
        shift 2
        ;;
      --slug)
        slug="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument: $1" >&2
        usage
        exit 1
        ;;
    esac
  done

  require_arg "--base-branch" "$base_branch"
  require_arg "--base-worktree" "$base_worktree"
  require_arg "--task-branch" "$task_branch"
  require_arg "--task-worktree" "$task_worktree"
  require_arg "--slug" "$slug"

  local repo_root
  repo_root="$(git rev-parse --show-toplevel)"
  local common_dir
  common_dir="$(git rev-parse --git-common-dir)"
  local output_dir="$common_dir/intent-driven-delivery/tasks"
  local output_file="$output_dir/$task_branch.json"

  mkdir -p "$(dirname "$output_file")"

  cat >"$output_file" <<EOF
{
  "repo_root": "$repo_root",
  "base_branch": "$base_branch",
  "base_worktree_path": "$base_worktree",
  "task_branch": "$task_branch",
  "task_worktree_path": "$task_worktree",
  "slug": "$slug",
  "created_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF

  echo "$output_file"
}

main() {
  if [[ $# -lt 1 ]]; then
    usage
    exit 1
  fi

  case "$1" in
    write)
      shift
      write_context "$@"
      ;;
    *)
      echo "unknown command: $1" >&2
      usage
      exit 1
      ;;
  esac
}

main "$@"
