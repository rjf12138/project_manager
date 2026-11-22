#!/usr/bin/env bash
#
# build.sh - 当前项目（project_manager）构建 / 清理脚本
#
# 只操作“当前项目”自身：
#   build      编译当前项目，生成可执行产物到 output/<mode>/bin/pm
#   rebuild    先清理再编译
#   clean      清除当前项目生成的产物（可执行文件 + build 目录）
#   clean-all  清空整个产物目录 output/ 与 build/
#   test       运行单元测试
#   run        编译后运行 pm（-- 之后的参数透传给 pm）
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ---------- 默认配置 ----------
GO_SUBDIR="go"                 # Go 模块所在子目录
BIN_NAME="pm"                  # 可执行文件名
CMD="build"
CMD_SET=0
MODE="debug"
OUTPUT_ROOT="$SCRIPT_DIR/output"
ONLINE=0
THIRDPARTY=1
EXTRA_ARGS=()

# ---------- 日志 ----------
if [[ -t 1 ]]; then
  C_RED=$'\033[31m'; C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'
  C_BLUE=$'\033[34m'; C_DIM=$'\033[2m'; C_RESET=$'\033[0m'
else
  C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_DIM=""; C_RESET=""
fi
info() { printf '%s[INFO]%s %s\n' "$C_BLUE" "$C_RESET" "$*"; }
ok()   { printf '%s[ OK ]%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn() { printf '%s[WARN]%s %s\n' "$C_YELLOW" "$C_RESET" "$*"; }
err()  { printf '%s[FAIL]%s %s\n' "$C_RED" "$C_RESET" "$*" >&2; }

usage() {
  cat <<EOF
project_manager 构建脚本

用法:
  ./build.sh [command] [options] [-- extra...]

命令:
  build        构建当前项目（默认）
  rebuild      先清理再构建
  clean        清除当前项目生成的产物
  clean-all    清空整个 output/ 与 build/ 目录
  thirdparty   只编译并打包 third_party/ 下的内置第三方包
  test         运行单元测试
  run          构建后运行 pm，-- 之后的参数透传给 pm
  help         显示本帮助

选项:
  -d, --debug          debug 模式（默认，保留符号便于 gdb 调试）
  -r, --release        release 模式（-s -w 剥离符号，体积更小）
  -o, --output DIR     指定产物根目录（默认 ./output）
      --online         允许联网拉取 Go 依赖（默认离线）
      --no-thirdparty  构建时跳过内置第三方包（加快本地迭代）
  -h, --help           显示本帮助

产物路径:
  <output>/<mode>/bin/pm

示例:
  ./build.sh                    # debug 构建 -> output/debug/bin/pm
  ./build.sh -r                 # release 构建 -> output/release/bin/pm
  ./build.sh rebuild -r         # 清理后 release 构建
  ./build.sh clean              # 清除当前项目的生成产物
  ./build.sh clean-all          # 清空整个 output/ 与 build/
  ./build.sh test               # 运行全部单元测试
  ./build.sh thirdparty         # 只重建内置第三方包
  ./build.sh --no-thirdparty    # 跳过内置包，快速构建 pm
  ./build.sh run -- list        # 构建并执行 pm list
EOF
}

# ---------- 参数解析 ----------
while [[ $# -gt 0 ]]; do
  case "$1" in
    build|rebuild|clean|clean-all|test|run|thirdparty|help)
      CMD="$1"; CMD_SET=1; shift ;;

    -d|--debug)
      MODE="debug"; shift ;;
    -r|--release)
      MODE="release"; shift ;;

    -o|--output)
      if [[ -z "${2:-}" ]]; then
        err "选项 $1 缺少参数"
        exit 2
      fi
      OUTPUT_ROOT="$2"; shift 2 ;;
    --output=*)
      OUTPUT_ROOT="${1#*=}"; shift ;;

    --online)
      ONLINE=1; shift ;;

    --no-thirdparty)
      THIRDPARTY=0; shift ;;
    --thirdparty)
      THIRDPARTY=1; shift ;;

    -h|--help)
      CMD="help"; CMD_SET=1; shift ;;

    --)
      shift
      EXTRA_ARGS+=("$@")
      break ;;

    -*)
      err "未知选项: $1"
      usage
      exit 2 ;;

    *)
      if [[ "$CMD_SET" -eq 0 ]]; then
        err "未知命令: $1"
        usage
        exit 2
      fi
      EXTRA_ARGS+=("$1"); shift ;;
  esac
done

# ---------- 派生路径 ----------
if [[ "$CMD" == "help" ]]; then
  usage
  exit 0
fi

# 相对产物目录按“脚本所在目录”解析，保证在任意 cwd 下结果一致
case "$OUTPUT_ROOT" in
  /*) : ;;
  *) OUTPUT_ROOT="$SCRIPT_DIR/$OUTPUT_ROOT" ;;
esac
OUTPUT_ROOT="${OUTPUT_ROOT%/}"

# 产物目录必须是一个明确的目录层级，避免误删根路径
if [[ -z "$OUTPUT_ROOT" || "$OUTPUT_ROOT" == "/" ]]; then
  err "非法的产物目录: '${OUTPUT_ROOT}'"
  exit 2
fi

BIN_DIR="$OUTPUT_ROOT/$MODE/bin"
BIN_PATH="$BIN_DIR/$BIN_NAME"
GO_DIR="$SCRIPT_DIR/$GO_SUBDIR"
BUILD_DIR="$SCRIPT_DIR/build"
THIRD_PARTY_DIR="$SCRIPT_DIR/third_party"
BUILTIN_DIR="$GO_DIR/core/builtin"

# ---------- 公共函数 ----------

# 拒绝删除危险路径（守卫 clean-all）
guard_path() {
  local p="${1:-}"
  if [[ -z "$p" || "$p" == "/" || "$p" == "${HOME:-}" || ${#p} -lt 4 ]]; then
    err "拒绝删除不安全的路径: '$p'"
    exit 1
  fi
}

# 校验 Go 环境与模块目录
require_go() {
  if ! command -v go >/dev/null 2>&1; then
    err "未找到 go 命令，请先安装 Go 工具链"
    exit 1
  fi
  if [[ ! -f "$GO_DIR/go.mod" ]]; then
    err "未找到 $GO_DIR/go.mod（请把本脚本放在仓库根目录）"
    exit 1
  fi
}

# 配置 Go 网络策略：默认强制离线，避免在无网环境卡住；--online 时使用用户/Go 默认代理
setup_go_env() {
  require_go
  if [[ "$ONLINE" -eq 1 ]]; then
    info "联网模式：GOPROXY=${GOPROXY:-<Go 默认>}"
  else
    export GOPROXY=off
    info "GOPROXY=off（离线构建；如需联网请加 --online）"
  fi
}

# ---------- 命令实现 ----------

# 编译 third_party/ 下的第三方源码并打包成内置 .pm（带内容哈希缓存）
do_thirdparty() {
  setup_go_env

  if [[ ! -d "$THIRD_PARTY_DIR" ]]; then
    warn "未找到 $THIRD_PARTY_DIR，跳过内置包"
    return 0
  fi

  local count
  count="$(find "$THIRD_PARTY_DIR" -mindepth 2 -maxdepth 2 -name pkg.json 2>/dev/null | wc -l)"
  if [[ "$count" -eq 0 ]]; then
    info "third_party/ 下没有包，跳过内置包"
    return 0
  fi

  info "构建内置第三方包: $count 个"
  ( cd "$GO_DIR" && go run ./cmd/pkgtool \
      -third-party "$THIRD_PARTY_DIR" \
      -out "$BUILTIN_DIR" )
}

do_build() {
  setup_go_env

  if [[ "$THIRDPARTY" -eq 1 ]]; then
    do_thirdparty
  else
    info "已跳过内置第三方包（--no-thirdparty）"
  fi

  info "构建模式: $MODE"
  info "源码目录: $GO_DIR"
  info "产物路径: $BIN_PATH"

  mkdir -p "$BIN_DIR"

  local -a args=(build -o "$BIN_PATH")
  if [[ "$MODE" == "release" ]]; then
    args+=(-ldflags "-s -w")
  fi
  args+=(./cmd)

  ( cd "$GO_DIR" && go "${args[@]}" )

  local size
  size="$(du -h "$BIN_PATH" | cut -f1)"
  ok "构建完成: $BIN_PATH ($size)"
}

# 清理内置包产物（保留 PLACEHOLDER.txt）与第三方源码的 build/ 目录
clean_thirdparty_artifacts() {
  local p
  for p in "$BUILTIN_DIR/index.json" "$BUILTIN_DIR/packages" "$BUILTIN_DIR/sources" "$BUILTIN_DIR/.stamps"; do
    if [[ -e "$p" ]]; then
      guard_path "$p"
      rm -rf "$p"
      info "已删除内置包产物 $p"
    fi
  done

  if [[ -d "$THIRD_PARTY_DIR" ]]; then
    while IFS= read -r d; do
      [[ -n "$d" ]] || continue
      guard_path "$d"
      rm -rf "$d"
      info "已删除第三方构建目录 $d"
    done < <(find "$THIRD_PARTY_DIR" -type d -name build -mindepth 3 2>/dev/null)
  fi
}

do_clean() {
  local removed=0

  local m bin
  for m in debug release; do
    bin="$OUTPUT_ROOT/$m/bin/$BIN_NAME"
    if [[ -f "$bin" ]]; then
      rm -f "$bin"
      info "已删除产物 $bin"
      removed=1
    fi
  done

  if [[ -d "$BUILD_DIR" ]]; then
    guard_path "$BUILD_DIR"
    rm -rf "$BUILD_DIR"
    info "已删除构建目录 $BUILD_DIR"
    removed=1
  fi

  clean_thirdparty_artifacts

  if [[ "$removed" -eq 0 ]]; then
    warn "没有找到需要清理的产物"
  else
    ok "清理完成"
  fi
}

do_clean_all() {
  guard_path "$OUTPUT_ROOT"
  guard_path "$BUILD_DIR"

  if [[ -d "$OUTPUT_ROOT" ]]; then
    rm -rf "$OUTPUT_ROOT"
    ok "已清空产物目录 $OUTPUT_ROOT"
  else
    warn "$OUTPUT_ROOT 不存在，跳过"
  fi

  if [[ -d "$BUILD_DIR" ]]; then
    rm -rf "$BUILD_DIR"
    ok "已删除构建目录 $BUILD_DIR"
  else
    warn "$BUILD_DIR 不存在，跳过"
  fi
}

do_test() {
  setup_go_env
  info "运行测试: go test ./... -count=1"
  ( cd "$GO_DIR" && go test ./... -count=1 )
  ok "测试通过"
}

do_run() {
  do_build

  if [[ ${#EXTRA_ARGS[@]} -gt 0 ]]; then
    info "执行: $BIN_NAME ${EXTRA_ARGS[*]}"
    "$BIN_PATH" "${EXTRA_ARGS[@]}"
  else
    info "执行: $BIN_NAME"
    "$BIN_PATH"
  fi
}

# ---------- 分发 ----------
case "$CMD" in
  build)     do_build ;;
  rebuild)   do_clean; do_build ;;
  clean)     do_clean ;;
  clean-all) do_clean_all ;;
  test)      do_test ;;
  thirdparty) do_thirdparty ;;
  run)       do_run ;;
  *)
    err "未实现的命令: $CMD"
    usage
    exit 2 ;;
esac
