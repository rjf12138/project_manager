# 包管理（.pm）说明

## 1. 包格式

`.pm` 文件本质是一个 **tar.gz** 归档，解包后的结构：

```
<pkg-name>/
├── manifest.json        # 包元数据（必需）
├── include/             # 对外头文件（安装到项目的 extern_inc/<pkg-name>/）
└── lib/
    ├── debug/           # debug 版库文件（.a / .so）
    └── release/         # release 版库文件（.a / .so）
```

> **平台维度（设计中，尚未实现）**：当前的 `lib/debug`、`lib/release` 不区分目标平台，
> 同一个包无法同时提供 linux-x86_64 与 linux-arm64 的二进制；
> 计划改为 `lib/<os>-<arch>/{debug,release}/` 并在 `manifest.json` 声明 `platforms`，
> 旧格式视为平台 `any` 继续兼容。完整设计见 [`platform.md`](design/platform.md)。

`manifest.json` 字段：

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | 是 | 包名，同时也是安装后头文件目录名 |
| `version` | 是 | 版本号，用于归档文件命名与仓库检索 |
| `description` | 否 | 描述，供 `pm pkg search` 匹配 |
| `author` | 否 | 作者 |
| `lib_type` | 否 | `static` 或 `shared` |
| `link_libs` | 否 | 安装时自动写入项目配置的链接库名（生成 `target_link_libraries`） |
| `deps` | 否 | 递归依赖，格式 `[{"name":"a","version":"1.0.0"}]` |
| `include_dirs` | 否 | 包内的头文件目录（相对包根）。**第一个**的内容平铺到 `extern_inc/<pkg>/`；**其余**保持相对结构复制，并自动登记为 include 目录。缺省按约定探测 `include/`、`inc/` |
| `lib_dirs` | 否 | 包内的库目录（相对包根）。目录下有 `debug/`、`release/` 时按构建模式安装到 `lib/<mode>/`；否则整目录搬到 `lib/<pkg>/` 并自动登记为库目录。缺省为 `["lib"]` |
| `platforms` | 否 | *（设计中）* 提供的平台列表（`<os>-<arch>`，如 `linux-amd64`）；缺省为 `["any"]`。见 [`platform.md`](design/platform.md) |
| `platform_link_libs` | 否 | *（设计中）* 分平台的额外链接库，与 `link_libs` 取并集，如 `{"windows-amd64":["ws2_32"]}` |

示例：

```json
{
  "name": "greeter",
  "version": "1.0.0",
  "description": "simple greeting library",
  "lib_type": "static",
  "link_libs": ["greeter"],
  "deps": []
}
```

> 若源目录缺少 `manifest.json`，`pm pkg create` 会以目录名作为包名、`0.1.0` 作为版本自动生成。

## 2. 本地仓库

仓库目录：`~/.project_manager/repo/`（与 `solution.json` 同根，可用 `pm pkg dir` 查看）。

查找分两层，**用户仓库优先**：

| 层 | 位置 | 可写 |
|---|---|---|
| 用户仓库 | `~/.project_manager/repo/` | ✅ |
| 内置层 | `~/.project_manager/builtin/<key>/` | ❌ 只读，随 pm 分发（见 [third_party.md](third_party.md)） |

用户仓库里的同名同版本包会覆盖内置包；内置包无法用 `pm pkg remove` 删除。

包文件按 `<name>-<version>.pm` 命名，例如 `greeter-1.0.0.pm`，并附带摘要边车
`greeter-1.0.0.pm.sha256`（sha256sum 兼容格式）。

> **版本不可变**：同一个 `name@version` 只允许存在一份内容。重复 `pm pkg create` 会报错，
> 需要显式加 `-f/--force` 才会覆盖。这样依赖才能复现。
>
> **定位以 manifest 为准**：仓库检索按归档内的 `manifest.json` 匹配 name/version，
> 不从文件名反推，因此 `1.0.0-rc1`、`1.2.0-beta.1` 这类版本号可以正常安装。
>
> **远端共享仓库**的设计见 [`registry.md`](design/registry.md)，但**目前暂缓实施**——
> 当前只使用本地仓库 `~/.project_manager/repo/`，没有远端存储。

## 3. 安装流程

`pm pkg install <pkg>[@ver]` / `pm deps add` 会：

1. 在本地仓库定位包文件（未指定版本时取**版本号最大**的一个）。
2. **校验归档 sha256**（读取同名 `.sha256` 边车；没有边车的旧包跳过校验），
   校验失败直接拒绝安装，不会把半包/被篡改的包装进项目。
3. 解压到临时目录，读取 `manifest.json`（解压带路径穿越防护）。
4. 清空 `extern_inc/<pkg-name>/` —— 这个目录由该包独占，先删干净才能避免旧版本残留。
5. 安装头文件：主目录内容平铺到 `extern_inc/<pkg-name>/`；额外目录保持结构复制。
6. 安装库文件：`debug/`、`release/` 分别复制到 `lib/debug/`、`lib/release/`
   （若某一种缺失，则用另一种补齐，保证两种构建模式都能链接）；
   没有模式子目录的库目录整目录搬到 `lib/<pkg-name>/`。
7. 递归安装 `deps` 中声明的依赖（按 `name@version` 去重，避免循环）。
8. **自动写入项目配置**，无需手工添加任何目录或库：

   | 配置项 | 写入内容 |
   |---|---|
   | `dependencies` | 每个已装包的 `name`/`version`/`lib_type` |
   | `deps.libs` | 各包 `link_libs` 的并集 → `target_link_libraries` |
   | `deps.inc_dirs` | 各包额外头文件目录（如 `extern_inc/<pkg>/vendor/nested`） |
   | `deps.lib_dirs` | 各包非约定库目录（如 `lib/<pkg>/prebuilt`） |

> 标准布局（`include/` + `lib/{debug,release}/`）的包**连登记都不需要**：
> `extern_inc` 与 `lib/<mode>` 本来就在 CMake 的搜索路径上，装完直接能 `#include` 和链接。
> 只有非标准布局的包才会往 `deps.inc_dirs` / `deps.lib_dirs` 里加条目。
>
> `deps.inc_dirs` / `deps.lib_dirs` 同时可以在 TUI 的「Include Dirs / Lib Dirs」页面手工增删，
> 生成器会一并使用（相对路径按项目根解析，绝对路径原样使用）。

### 升级 / 重装

同一个包从旧版本升到新版本时，安装过程会先清理旧版本留下的内容，避免半新半旧：

- `extern_inc/<pkg>/` 整体重建 → 新版本删掉的头文件不会残留；
- 旧版本声明、新版本不再声明、且其它依赖也不需要的库文件会被删除，并从 `deps.libs` 移除；
- 旧版本登记、新版本不再提供的 `deps.inc_dirs` / `deps.lib_dirs` 条目及其目录会被移除。

因此升级只需一条命令，不需要先卸载：

```bash
pm pkg install mylib@2.0.0 -p App
```

### 卸载

`pm deps remove <pkg> [-p project]` 会同时移除依赖声明**和**安装时复制进项目的文件：

- 删除 `<project>/extern_inc/<pkg>/`；
- 删除该包 `link_libs` 对应的库文件（`lib/debug`、`lib/release`）；
- 删除该包登记的 `deps.inc_dirs` / `deps.lib_dirs` 条目及其目录；
- 仍被其它依赖声明的库会保留；某个剩余依赖的元数据已不可获取时，保守起见不删任何库文件。

只想丢掉声明、保留文件时加 `--keep-files`（或 `-k`）。

```bash
pm deps remove greeter -p Demo              # 声明 + 已安装文件一起清理
pm deps remove greeter -p Demo --keep-files # 只删声明
```

## 4. 命令一览

### 包仓库

| 命令 | 说明 |
|------|------|
| `pm pkg create <dir> [-o out.pm] [-f]` | 将目录打包为 `.pm`，默认输出到仓库；同 `name@version` 已存在时报错，`-f` 才覆盖 |
| `pm pkg install <pkg>[@ver] [-p project]` | 安装包到项目（含递归依赖） |
| `pm pkg remove <pkg>[@ver]` | 从仓库删除包 |
| `pm pkg list` | 列出仓库中的包 |
| `pm pkg search [keyword]` | 按名称/描述搜索 |
| `pm pkg dir` | 打印仓库目录 |

### 项目依赖

| 命令 | 说明 |
|------|------|
| `pm deps install [project]` | 安装项目已声明的全部依赖 |
| `pm deps add <pkg>[@ver] [-p project]` | 添加并安装依赖 |
| `pm deps remove <pkg> [-p project] [--keep-files]` | 移除依赖声明并清理已安装文件 |
| `pm deps list [project]` | 查看依赖及安装状态 |

`pm deps list` 的状态按**该依赖自己的文件**判断，而不是只看目录是否非空：

| 状态 | 含义 |
|------|------|
| `installed` | 头文件与库文件都在 |
| `headers-only` | 只有 `extern_inc/<pkg>/`，缺库文件 |
| `libs-only` | 只有库文件，缺头文件 |
| `missing` | 两者都没有 |

> 项目参数可省略，默认使用当前活动项目；flag 与位置参数顺序随意（`pm build App -r` 与 `pm build -r App` 等价）。

### 项目管理与构建

| 命令 | 说明 |
|------|------|
| `pm init <name> [parent-path]` | 创建项目骨架 |
| `pm load <path>` | 加载/注册已有项目 |
| `pm list` | 列出已注册项目（`*` 标记活动项目） |
| `pm remove <name\|path>` | 从列表移除（不删除文件） |
| `pm build [name] [-t debug\|release] [-r]` | 构建（`-r` 先清理） |
| `pm clean [name]` | 清理编译产物 |
| `pm run [name] [-- args...]` | 运行可执行文件 |

## 5. 完整示例

```bash
# 1) 准备一个静态库包
mkdir -p greeter/include greeter/src greeter/lib/debug greeter/lib/release

cat > greeter/manifest.json <<'JSON'
{ "name": "greeter", "version": "1.0.0", "lib_type": "static", "link_libs": ["greeter"] }
JSON

cat > greeter/include/greeter.h <<'HDR'
#pragma once
#include <string>
std::string greet(const std::string &who);
HDR

cat > greeter/src/greeter.cc <<'SRC'
#include "greeter.h"
std::string greet(const std::string &who) { return "Hello, " + who + "!"; }
SRC

g++ -std=c++17 -Igreeter/include -c greeter/src/greeter.cc -o /tmp/greeter.o
ar rcs greeter/lib/debug/libgreeter.a /tmp/greeter.o
cp greeter/lib/debug/libgreeter.a greeter/lib/release/libgreeter.a

# 2) 打包并入库
pm pkg create ./greeter

# 3) 创建项目并安装依赖
pm init Demo .
pm pkg install greeter -p Demo

# 4) 在 main/main.cc 中使用
cat > Demo/main/main.cc <<'MAIN'
#include <greeter/greeter.h>
#include <iostream>
int main() { std::cout << greet("pm") << std::endl; return 0; }
MAIN

# 5) 构建并运行
pm build Demo
pm run Demo        # 输出: Hello, pm!
```
