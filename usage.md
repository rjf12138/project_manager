# pm 使用手册

`pm` 是一个 C/C++ 项目管理工具。本文按「先跑起来 → 再查细节」的顺序组织；
包格式与配置字段的完整参考见 [doc/package.md](doc/package.md) 和 [doc/config.md](doc/config.md)。

- [1. 安装](#1-安装)
- [2. 快速开始](#2-快速开始)
- [3. 项目目录约定](#3-项目目录约定)
- [4. 命令参考](#4-命令参考)
- [5. 包管理使用](#5-包管理使用)
- [6. TUI 操作](#6-tui-操作)
- [7. 典型工作流](#7-典型工作流)
- [8. 常见问题](#8-常见问题)

---

## 1. 安装

`pm` 是单个可执行文件，没有运行时依赖。

```bash
./build.sh                 # 构建 → output/debug/bin/pm（含内置第三方包）
export PATH="$PWD/output/debug/bin:$PATH"
pm --help
```

首次构建会编译 `third_party/` 下的内置包（gtest 约 10 秒），之后走缓存；
不需要联网（源码都在仓库里）。只想快速构建 pm 本体时用 `./build.sh --no-thirdparty`。

`./build.sh --help` 可查看全部构建选项（`-r` release、`test`、`clean`、`run` 等）。

---

## 2. 快速开始

```bash
pm init demo .        # 在当前目录创建 demo 项目
cd demo
pm build              # 生成 CMakeLists.txt 并编译
pm run                # 运行 → Hello from demo!
```

`pm init` 会自动生成一个可编译的 `main/main.cc`，所以**新建后立刻就能构建**，不需要先补源码。

注册与切换项目：

```bash
pm list               # 列出已注册项目（* 标记活动项目）
pm load ~/work/other  # 注册已有项目，并设为活动项目
pm remove demo        # 从列表移除（不删除任何文件）
```

> 项目列表保存在 `~/.project_manager/solution.json`。列表里指向的目录如果被删掉，
> 下次启动会自动跳过，不会报错。

---

## 3. 项目目录约定

`pm init` 生成的项目骨架：

```
<project>/
├── config/                 # 配置文件
│   └── .pmproject          # 项目元数据（JSON）
├── doc/                    # 文档
├── extern_inc/             # 外部依赖头文件（包管理器写入）
├── inc/                    # 本项目对外提供的头文件
├── lib/                    # 外部库文件（包管理器写入）
│   ├── debug/              # debug 版库文件
│   └── release/            # release 版库文件
├── main/                   # 可执行文件入口源文件
├── output/                 # 编译结果
│   ├── debug/{bin,lib}/
│   └── release/{bin,lib}/
└── src/                    # 项目内部实现
```

约定：

1. 内部头文件和源文件放在一起；要对外提供的头文件放 `inc/`；依赖的其它项目头文件放 `extern_inc/`。
2. `inc/`、`extern_inc/`、以及各源文件目录**默认就是头文件搜索路径**，不需要写进配置；
   除此之外额外创建的目录才需要配置。
3. `lib/<mode>/`（`debug` / `release`）**默认就是库搜索路径**，不需要写进配置；
   其它库目录才需要配置。
4. `main/` 不能作为程序源文件目录写进 `source_dirs`（入口文件通过 `main_file` 指定）。
5. 不给外部引用的头文件（如 `hello.h` 只在旁边 `hello.cc` 用）和源文件放在一起；需要被别的项目引用的才放 `inc/`。

> 约定 2、3 正是「装包后无需手工配置」的原因：包管理器把文件放到 `extern_inc/` 和 `lib/<mode>/`，
> 而生成器已经无条件包含这两个路径。只有**非标准布局**的包才会往 `deps.inc_dirs` / `deps.lib_dirs` 里加条目。

---

## 4. 命令参考

### 4.1 项目管理

| 命令 | 说明 |
|---|---|
| `pm init <name> [parent-path]` | 创建项目骨架。`parent-path` 省略时为当前目录；也可用 `-p/--path` |
| `pm load <path>` | 注册已有项目并设为活动项目 |
| `pm list` | 列出已注册项目（`*` = 活动项目，显示构建模式） |
| `pm remove <name\|path>` | 从注册列表移除，**不删除文件** |

### 4.2 构建与运行

| 命令 | 说明 |
|---|---|
| `pm build [name] [-t debug\|release] [-r]` | 生成 `CMakeLists.txt` 并构建；`-t` 切换并保存构建模式，`-r/--rebuild` 先清理 |
| `pm clean [name]` | 删除 `build/` 与 `output/` 下的产物（随后重建空的 `output/<mode>/{bin,lib}`） |
| `pm run [name] [-- args...]` | 运行当前模式下的可执行文件；`--` 之后的参数会透传并**保存**到配置的 `run_args` |

`[name]` 省略时使用活动项目；也可以直接传项目路径（会自动注册）。

### 4.3 项目依赖

| 命令 | 说明 |
|---|---|
| `pm deps install [name]` | 安装配置里声明的全部依赖 |
| `pm deps add <pkg>[@ver] [-p project]` | 添加并安装依赖，自动写入配置 |
| `pm deps remove <pkg> [-p project] [--keep-files]` | 移除依赖声明**并清理已安装的文件**；`--keep-files` 只删声明 |
| `pm deps list [name]` | 查看依赖及安装状态 |

`pm deps list` 的状态含义：

| 状态 | 含义 |
|---|---|
| `installed` | 头文件和库文件都在 |
| `headers-only` | 只有 `extern_inc/<pkg>/` |
| `libs-only` | 只有库文件 |
| `missing` | 两者都没有 |

### 4.4 包仓库（本地）

| 命令 | 说明 |
|---|---|
| `pm pkg create <dir> [-o out.pm] [-f]` | 把目录打包成 `.pm`；同名同版本已存在时报错，`-f/--force` 才覆盖 |
| `pm pkg install <pkg>[@ver] [-p project]` | 安装包到项目（含递归依赖） |
| `pm pkg remove <pkg>[@ver]` | 从仓库删除包 |
| `pm pkg list` | 列出仓库中的包 |
| `pm pkg search [keyword]` | 按名称/描述搜索；省略关键字则列出全部 |
| `pm pkg dir` | 打印仓库目录路径 |

子命令支持缩写：`create`=`build`、`install`=`i`、`remove`=`rm`、`list`=`ls`。

### 4.5 快捷命令

作用于**活动项目**（等价于同名子命令不带项目名）：

| 命令 | 等价于 |
|---|---|
| `pm -r` | `pm build`（增量） |
| `pm -c` | `pm clean` |
| `pm -cr` | 先清理再构建 |
| `pm -l` | 打开 TUI 项目列表 |
| `pm -cfg` | 打开 TUI 配置页 |
| `pm tui` | 完整 TUI |

### 4.6 通用约定

- **参数顺序随意**：`pm build -r App` 与 `pm build App -r` 等价。
- **`--` 分隔**：`pm run App -- --input a.txt` 中 `--` 之后的都是程序参数。
- **退出码**：成功 `0`；命令执行失败（如无活动项目、构建失败）`1`；未知命令 `2`。
- **配置位置**：项目配置 `<项目>/config/.pmproject`；全局状态 `~/.project_manager/`。

---

## 5. 包管理使用

### 5.1 安装一个包

```bash
pm pkg install greeter -p Demo      # 不写版本 → 取仓库里版本号最大的
pm pkg install greeter@1.0.0 -p Demo
```

安装会自动完成（**无需手工改配置**）：

| 配置项 | 内容 |
|---|---|
| `dependencies` | `name` / `version` / `lib_type` |
| `deps.libs` | 包声明的 `link_libs` → `target_link_libraries` |
| `deps.inc_dirs` | 包贡献的额外头文件目录（仅非标准布局） |
| `deps.lib_dirs` | 包贡献的额外库目录（仅非标准布局） |

装完直接 `pm build` 即可 `#include` 和链接。

`pm pkg list` 会标注包的来源：来自内置层的会显示 `[builtin]`。

> 同样的操作也可以在 TUI 里完成：`Configure → Dependencies → a`（见 [6. TUI 操作](#6-tui-操作)）。

### 5.6 内置包（开箱可用）

`pm` 自带一批常用的第三方依赖，装完就能用，不需要自己编译：

```bash
pm pkg list                 # 会看到 gtest  [builtin]
pm pkg install gtest -p app # 直接安装，零等待
```

工作方式：

- 仓库里只保存**源码**（`third_party/`），构建 pm 时编译并打包，随二进制分发；
- 运行时释放到 `~/.project_manager/builtin/`，作为**只读仓库层**，用户仓库优先；
- 预编译产物只按 OS/Arch 比对；**构建环境不匹配时自动用内嵌源码本机构建**（约 10 秒），
  产物缓存进用户仓库，之后秒装；
- 内置包不能被 `pm pkg remove` 删除；用户在用户仓库放同名同版本包可以覆盖它。

gtest 的用法（注意 `gtest_main` 自带 `main()`，与项目默认的 `main/main.cc` 冲突）：

```cpp
// main/main.cc —— 自己提供 main()
#include <gtest/gtest.h>

TEST(MathTest, Add) { EXPECT_EQ(3, 1 + 2); }

int main(int argc, char** argv) {
    ::testing::InitGoogleTest(&argc, argv);
    return RUN_ALL_TESTS();
}
```

想看/新增内置包，见 [doc/third_party.md](doc/third_party.md)。

### 5.2 升级 / 重装

直接装新版本，不需要先卸载：

```bash
pm pkg install greeter@2.0.0 -p Demo
```

安装过程会先清理旧版本残留：新版本删掉的头文件、改名的旧库文件、失效的链接库名和目录条目。

### 5.3 卸载

```bash
pm deps remove greeter -p Demo              # 删声明 + 清已安装文件
pm deps remove greeter -p Demo --keep-files # 只删声明，文件保留
```

仍被其它依赖引用的库文件会保留；某个剩余依赖的元数据取不到时会保守处理，不删库文件。

### 5.4 打包自己的库

```
mylib/
├── manifest.json
├── include/            # 对外头文件
└── lib/
    ├── debug/libmylib.a
    └── release/libmylib.a
```

```json
{
  "name": "mylib",
  "version": "1.0.0",
  "description": "example library",
  "lib_type": "static",
  "link_libs": ["mylib"]
}
```

```bash
pm pkg create ./mylib        # 打包进仓库 ~/.project_manager/repo/
pm pkg list                  # 确认
```

`manifest.json` 缺失时会以目录名 + `0.1.0` 兜底。**同一 `name@version` 只允许一份内容**，
重复打包会报错——要覆盖需显式 `-f`（这样依赖才能复现）。

非标准布局（头文件不在 `include/`、库不在 `lib/debug|release/`）用 `include_dirs` / `lib_dirs` 声明，
安装时会自动复制并登记到项目配置。字段细节见 [doc/package.md](doc/package.md)。

### 5.5 完整性

每个 `.pm` 都带一个 `<name>-<version>.pm.sha256` 边车文件。安装前会校验摘要，
不匹配直接拒绝。没有边车的旧包会跳过校验（向后兼容）。

---

## 6. TUI 操作

```bash
pm tui        # 完整界面
pm -l         # 直接进项目列表（q/esc 退出）
pm -cfg       # 直接进配置页（q/esc 退出）
```

### 主菜单

| 按键 | 作用 |
|---|---|
| `↑` `↓` | 移动光标 |
| `enter` | 执行选中项 |
| `q` | 退出 |
| `ctrl+c` | 随时退出（任何页面都生效） |

菜单项：Load / Open Project、Create New Project，以及已有项目时的
Build、Rebuild、Run、Configure、Clean、List Projects、Quit。

### 各页面

| 页面 | 按键 |
|---|---|
| Create Project | `↑↓`/`tab` 切换字段，`enter` 创建，`esc` 返回 |
| Load Project | 输入项目路径（支持 `~`），`enter` 加载，`esc` 返回 |
| Build | `↑↓` 选择，`enter` 执行，`d` 切 Debug，`r` 切 Release，`l` 查看上次构建日志，`esc` 返回 |
| Build Log | `↑↓`/`j k` 滚动，`pgup`/`pgdn` 翻页，`g`/`G` 到首/尾，`esc` 返回 |
| Configure | `↑↓` 移动，`b` 构建模式，`g` 生成类型，`f` 切换 main 文件，`s` 保存退出，`c`/`esc` 取消；`enter` 进入带 `[enter]` 标记的子页 |
| Dependencies | `↑↓` 选择，`a` 从本地仓库安装，`r` 重装选中，`u` 全部重装，`d` 卸载，`esc` 返回 |
| Install Package | `↑↓` 选择仓库中的包，`enter` 安装到当前项目，`esc` 返回 |
| Libraries / Lib Dirs / Include Dirs / Source Dirs | `↑↓` 选择，`a` 新增，`d` 删除，`enter` 编辑；输入时 `enter` 保存、`esc` 取消、`ctrl+u` 清空 |
| Projects 列表 | `↑↓` 选择，`enter` 切换活动项目，`esc` 返回 |

> 配置页里的 `Libraries` / `Lib Dirs` / `Include Dirs` 由 `pm pkg install` 自动填充，
> 一般不需要手动维护；需要额外目录时再手工添加（相对路径按项目根解析，也可写绝对路径）。

### 依赖管理页

`Configure → Dependencies` 可以看到当前项目安装了哪些包，以及**每个包的安装状态**：

```
Dependencies

Project: demo   (lib/debug)

 ▶ mylib              1.0.0      installed
   missinglib         2.0.0      missing

↑↓ select • a add • r reinstall • u reinstall all • d remove • esc back
```

| 状态 | 含义 |
|---|---|
| `installed` | 头文件和库文件都在 |
| `headers-only` | 只有 `extern_inc/<pkg>/`，库文件缺失 |
| `libs-only` | 只有库文件，头文件缺失 |
| `missing` | 两者都没有（非 `installed` 状态会以红色显示） |

在这个页面可以完成依赖的全部日常操作：

| 按键 | 作用 |
|---|---|
| `a` | 打开本地仓库的包列表，`enter` 安装选中的包（已装的会标 `[installed]`，可升级的标 `[upgrade from x.y.z]`） |
| `r` | 重装选中的依赖（修复文件被误删，或按配置恢复指定版本） |
| `u` | 按配置重装全部依赖 |
| `d` | 卸载选中的依赖（删声明 + 清理已安装文件） |

安装 / 卸载会直接写入项目配置并落盘文件；如果配置页里有未保存的改动，会先自动保存再执行，
所以不用担心改动被覆盖。操作结果会显示在页面底部。

构建与运行在界面上是异步的：构建时显示进度动画，结束后进入构建日志页；
运行会把终端交给程序，退出后自动回到界面。

---

## 7. 典型工作流

### 7.1 新建项目并构建

```bash
pm init myapp .
cd myapp
# 按需修改 src/ 与 main/main.cc
pm build
pm run
```

### 7.2 把自己的库给另一个项目用

```bash
# 1) 构建库并整理成包目录
cd ~/work/mylib
#    确保有 include/ 和 lib/{debug,release}/
pm pkg create .                      # 入本地仓库

# 2) 在目标项目里安装
cd ~/work/myapp
pm pkg install mylib -p myapp        # 配置自动更新
pm build                             # 直接可用

# 3) 源码里引用
#    #include <mylib/mylib.h>
```

### 7.3 用 TUI 管理

```bash
pm tui
# 主菜单 → Configure → 修改构建模式 / main 文件 / 库目录 → s 保存
# 主菜单 → Build → Rebuild
# 主菜单 → List Projects → 切换活动项目
```

---

## 8. 常见问题

**`no active project; specify a project name...`**
没有活动项目。用 `pm load <path>` 注册，或在命令里带上项目名（`pm build myapp`）。

**构建失败，提示找不到源文件**
检查 `src/`、`main/` 下是否有源文件，以及 `config/.pmproject` 里的 `main_file` 是否指向存在的文件。
`pm init` 生成的项目自带 `main/main.cc`，不应出现该问题。

**`package X@Y already exists in repository`**
同一个 `name@version` 已存在。**推荐升版本号**；确实要覆盖时用 `pm pkg create -f`
（覆盖后其它项目无法复现旧内容，慎用）。

**`package verification failed: checksum mismatch`**
包归档与摘要不符（多为传输/复制损坏）。重新 `pm pkg create` 生成。

**`dependency X not found in project Y`**
该依赖没写在项目配置里。用 `pm deps list Y` 查看，或 `pm deps add` 添加。

**依赖装了但编译时找不到头文件**
- 标准布局包：确认头文件在 `extern_inc/<pkg>/` 下，源码里用 `#include <pkg/xxx.h>`。
- 非标准布局包：确认包 manifest 里声明了 `include_dirs`，安装后 `deps.inc_dirs` 里应有对应目录
  （可在 TUI 配置页查看）。

**改了配置但构建没变化**
`CMakeLists.txt` 在每次 `pm build` 时重新生成，配置改动会生效；
但如果 CMake 缓存里有旧的编译选项，用 `pm build -r` 强制重建。

**想清空所有编译产物**
`pm clean <name>` 删除 `build/` 与 `output/` 下的产物（`output/<mode>/{bin,lib}` 空目录会重建）；
本地仓库与项目配置都不受影响。
