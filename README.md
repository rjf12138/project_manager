# project_manager (pm)

面向 C/C++ 项目的命令行管理工具：创建项目骨架、自动生成 CMake、构建运行，
并自带一个**本地包管理器**（打包 / 安装 / 升级 / 卸载，带 sha256 校验）和终端交互界面（TUI）。

```
pm init demo .      # 创建项目（自带可编译的 main/main.cc）
pm build demo       # 生成 CMakeLists.txt 并构建
pm run demo         # 运行 → Hello from demo!
```

## 特性

- **配置驱动**：项目用 `config/.pmproject` 描述目录、构建模式、依赖，`CMakeLists.txt` 由工具生成，不用手写。
- **项目生命周期**：`init` / `load` / `list` / `remove` / `build` / `clean` / `run`。
- **本地包仓库**：把库打包成 `.pm`，别的项目一条命令安装，**依赖库、头文件目录、库目录自动写进配置**。
- **完整性校验**：每个包带 sha256 摘要，安装前校验，半包/被篡改的包不会进入项目。
- **依赖生命周期**：升级自动清理旧版本残留，卸载连带清理已复制的文件。
- **TUI**：`pm tui` 图形化操作（主菜单 / 构建 / 配置 / **依赖管理** / 项目列表），也支持 `pm -l`、`pm -cfg` 直达；
  `pm -l` 里回车选择项目会自动切换活动项目并用 VS Code 打开（可用 `PM_EDITOR` 环境变量换成其它编辑器）。
- **内置第三方包**：随二进制自带 GoogleTest 等基础依赖，`pm pkg install gtest` 开箱可用；
  仓库里只放源码，二进制在构建时生成，环境不匹配时自动用内嵌源码本机构建。

## 环境要求

| 依赖 | 版本 | 用途 |
|---|---|---|
| Go | 1.18+ | 构建 `pm` 自身 |
| CMake | 3.10+ | 构建被管理的项目 |
| g++ / make | — | 编译 C/C++ |

## 构建 pm

```bash
./build.sh                   # debug 构建 → output/debug/bin/pm（含内置第三方包）
./build.sh -r                # release 构建 → output/release/bin/pm
./build.sh test              # 运行单元测试
./build.sh thirdparty        # 只重建内置第三方包
./build.sh --no-thirdparty   # 跳过内置包，快速构建 pm
./build.sh clean             # 清理产物（含第三方构建目录）
./build.sh run -- list       # 构建后直接执行 pm list
```

首次 `./build.sh` 会编译 `third_party/` 下的内置包（gtest 约 10 秒），之后按源码哈希缓存。
源码都在仓库里，**整个构建过程不需要联网**。

产物是**单个可执行文件**（可用 `CGO_ENABLED=0` 静态构建），把它放进 `PATH` 即可：

```bash
export PATH="$PWD/output/debug/bin:$PATH"
pm --help
```

## 命令总览

| 类别 | 命令 |
|---|---|
| 项目管理 | `pm init <name> [parent-path]`、`pm load <path>`、`pm list`、`pm remove <name\|path>` |
| 构建运行 | `pm build [name] [-t debug\|release] [-r]`、`pm clean [name]`、`pm run [name] [-- args...]` |
| 项目依赖 | `pm deps install/add/remove/list` |
| 包仓库 | `pm pkg create/install/remove/list/search/dir` |
| 交互界面 | `pm tui`、`pm -l`、`pm -cfg` |
| 快捷操作 | `pm -r`、`pm -c`、`pm -cr`（作用于活动项目） |

完整说明见 [usage.md](usage.md)。

## 文档

| 文档 | 内容 | 状态 |
|---|---|---|
| [usage.md](usage.md) | **使用手册**：目录约定、命令详解、TUI 操作、典型工作流、常见问题 | 与代码同步 |
| [doc/package.md](doc/package.md) | 包管理细节：`.pm` 格式、manifest 字段、安装/升级/卸载流程 | 与代码同步 |
| [doc/config.md](doc/config.md) | `config/.pmproject` 字段参考、生成的 CMake 对应关系 | 与代码同步 |
| [doc/third_party.md](doc/third_party.md) | 内置第三方依赖：机制、`pkg.json` 写法、如何新增一个包 | 与代码同步 |
| [doc/design/registry.md](doc/design/registry.md) | 远端共享仓库（HTTP 协议、索引结构） | **设计稿 · 暂缓实施** |
| [doc/design/platform.md](doc/design/platform.md) | 平台（OS / 架构）维度 | **设计稿 · 未实现** |
| [TODO](TODO) | 最初的整体方案与待办清单 | 历史文档 |

> `doc/design/` 下的两份是**尚未实现**的设计稿，代码里没有对应功能，阅读时请勿当作待办计划。
> 当前只使用本地仓库 `~/.project_manager/repo/`，没有远端存储。

## 实现状态

| 最初规划 | 状态 |
|---|---|
| 项目管理：创建、加载、配置修改、构建 | ✅ 已实现（CLI + TUI） |
| 软件包管理：搜索、添加、删除 | ✅ 已实现（本地仓库，含打包/安装/升级/卸载） |
| 运行单个可执行文件 | ✅ 已实现（`pm run`、TUI 的 Run） |
| 支持 git 管理项目 | ❌ 未实现 |
| 安装（把 pm 装进系统） | ❌ 未实现；当前为单文件二进制，手动放进 `PATH` |
| 远端共享仓库 | ❌ 未实现（设计稿见 `doc/design/registry.md`） |
| 跨平台（OS / 架构）包 | ❌ 未实现（设计稿见 `doc/design/platform.md`） |

## 仓库结构

```
project_manager/
├── build.sh            # 构建/测试/清理脚本
├── usage.md            # 使用手册
├── doc/                # 参考文档与设计稿
│   ├── package.md
│   ├── config.md
│   ├── third_party.md
│   └── design/         # 未实现的设计稿
├── third_party/        # 内置第三方依赖【源码】，构建时编译打包
├── go/                 # Go 源码
│   ├── cmd/            # CLI 入口与子命令
│   ├── core/           # 项目、配置、CMake 生成、包管理
│   └── tui/            # Bubble Tea 终端界面
└── output/<mode>/bin/  # 构建产物
```

## 数据存放位置

| 路径 | 内容 |
|---|---|
| `~/.project_manager/solution.json` | 已注册项目列表与活动项目 |
| `~/.project_manager/repo/` | 本地包仓库（`<name>-<version>.pm` + `.sha256`） |
| `~/.project_manager/builtin/` | 从二进制释放的内置包（只读层，用户仓库优先） |
| `<项目>/config/.pmproject` | 项目配置 |
