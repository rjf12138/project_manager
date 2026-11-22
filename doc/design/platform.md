# 平台维度设计（OS / 架构）

> 现状：`debug`/`release` 是唯一的二元维度，`.pm` 包、项目 `lib/`、构建输出**都没有平台概念**。
> 结果是一个包只能装下「构建它的那台机器」的二进制，无法在 linux-x86_64 与 linux-arm64、
> windows 之间共享。本文补齐 `platform = <os>-<arch>` 这一维。
>
> 相关文档：[`package.md`](../package.md)（本地包管理）、[`registry.md`](registry.md)（远端仓库）。

---

## 1. 问题定位

| 位置 | 现状 | 缺失 |
|---|---|---|
| `PackageManifest` | 只有 `lib_type` | 无 `platforms` |
| `.pm` 归档 | `lib/debug`、`lib/release` | 无 `lib/<platform>/` 层 |
| `PackageTarget` | `LibDebug` / `LibRelease` | 无目标平台 |
| `Install` | 无条件复制 `lib/<mode>` | 不判断平台是否匹配 |
| `ProjectConfig` | 只有 `BuildMode` | 无 `target_platform` |
| `ProjectDirInfo` | `lib/debug`、`output/debug/bin` | 多平台会互相覆盖 |
| `GenerateCMakeLists` | `link_directories(lib/<mode>)`、`-march=native` | 无平台路径；且产物不可移植 |
| `index.json` | 无平台字段 | 客户端无法在下载前过滤 |

一个具体的后果：同一台 linux x86_64 机器上给 arm64 板子交叉编译，`lib/debug/` 会被覆盖；
把包发到仓库，别人在 aarch64 上装下来的是 x86_64 的 `.a`，只会在链接/运行时报错。

---

## 2. 平台标识规范

统一用 `<os>-<arch>`，小写，沿用 Go 工具链的词汇（`runtime.GOOS`/`GOARCH`），避免自造词表。

| 维度 | 取值 |
|---|---|
| `os` | `linux`、`windows`、`darwin`、`android`、`ios`、`freebsd` |
| `arch` | `amd64`、`arm64`、`arm`、`386`、`riscv64`、`loong64` |
| 特殊值 | `any` —— 平台无关（纯头文件包、或作者明确声明可跨平台） |

**别名归一化**（兼容常见写法，解析后一律转成规范值）：

| 输入 | 归一化 |
|---|---|
| `x86_64`、`x64`、`win64` | `amd64` |
| `aarch64`、`armv8` | `arm64` |
| `armv7`、`armhf` | `arm` |
| `i686`、`i386`、`win32` | `386` |
| `macos`、`osx`、`mac` | `darwin` |
| `win`、`win32` | `windows` |

**宿主默认**：`runtime.GOOS + "-" + runtime.GOARCH`。

**ABI 后缀（阶段 2，可选）**：`linux-arm64-musl`、`windows-amd64-mingw`。
linux 上 glibc/musl、Windows 上 MSVC/MinGW 的库不能混链，但多数内网场景统一工具链，
所以先不做，占位设计为「第三段可选」：`<os>-<arch>[-<abi>]`，匹配时 `linux-arm64` 可匹配
`linux-arm64-musl`（宽松）或有精确项时优先精确项。

---

## 3. 包格式

```
greeter/
├── manifest.json
├── include/                              # 头文件通常与平台无关，保持单份
└── lib/
    ├── linux-amd64/
    │   ├── debug/libgreeter.a
    │   └── release/libgreeter.a
    ├── linux-arm64/
    │   ├── debug/libgreeter.a
    │   └── release/libgreeter.a
    └── windows-amd64/
        ├── debug/greeter.lib
        └── release/greeter.lib
```

**向后兼容**：`lib/debug`、`lib/release`（没有平台层）视为 `platform = any`，旧包无需重打。
新包推荐统一用带平台层的结构，即使只提供一个平台——语义更明确。

> 头文件若确实需要按平台区分（如不同的 `config.h`），允许 `include/<platform>/`，
> 安装时优先该目录、否则回退到 `include/`。这条是可选能力，多数包用不到。

---

## 4. manifest / index 字段

### manifest.json 新增

```json
{
  "name": "greeter",
  "version": "1.0.0",
  "platforms": ["linux-amd64", "linux-arm64", "windows-amd64"],
  "link_libs": ["greeter"],
  "platform_link_libs": {
    "windows-amd64": ["ws2_32"],
    "linux-amd64": ["pthread"]
  }
}
```

| 字段 | 缺省 | 说明 |
|---|---|---|
| `platforms` | `["any"]` | 本包提供的平台；缺省即旧包语义 |
| `platform_link_libs` | 空 | 分平台的额外链接库，与 `link_libs` **取并集** |

最终写入项目 `deps.libs` 的集合 = `link_libs ∪ platform_link_libs[目标平台]`。

### index.json 新增

```json
{
  "name": "greeter",
  "version": "1.0.0",
  "platforms": ["linux-amd64", "linux-arm64"],
  "sha256": "02e0...",
  "filename": "greeter-1.0.0.pm"
}
```

客户端可**在下载前**就过滤掉不含目标平台的版本，避免白下载几十兆才报错。
索引里的 `platforms` 应与归档内 manifest 一致；服务端上传时校验这一点。

---

## 5. 安装解析与回退

目标平台的确定顺序：

1. `pm pkg install --platform <p>`（显式指定）；
2. 项目 `.pmproject` 的 `target_platform`；
3. 宿主平台。

解析规则（`Install` 内部）：

```
1. manifest.platforms 既不含 target 也不含 "any"
      → 报错：该包不支持 <target>，可用：linux-amd64, linux-arm64
2. 找库目录 lib/<target>/<mode>/
      存在            → 用它
3. 否则回退 lib/<mode>/（any / 旧格式）
      存在            → 用它（若 target != 宿主，给出「无法确认二进制平台」警告）
4. 都不存在        → 报错：缺少 <target> 的 <mode> 库（包内只有：linux-amd64/release）
```

**落盘位置**取决于项目是否多平台（见下节），单平台项目路径与现在完全一致。

---

## 6. 项目侧布局与兼容策略

`.pmproject` 新增：

```json
{
  "target_platform": "linux-arm64",
  "platforms": ["linux-amd64", "linux-arm64"]
}
```

| 情况 | 库目录 | 构建输出 | 兼容性 |
|---|---|---|---|
| 未设置 `platforms`（**绝大多数**） | `lib/<mode>/` | `output/<mode>/bin\|lib` | **与 `usage.md` 和现有项目完全一致，零改动** |
| 设置了 `platforms`（多平台交叉编译） | `lib/<platform>/<mode>/` | `output/<platform>/<mode>/bin\|lib` | 新增能力 |

设计意图：**不为不需要跨平台的人改变任何东西**。只有当项目显式声明多平台时，才启用分层目录。

`target_platform` 未设置时等价于宿主；设置后用于选择依赖库与构建输出目录。

`ProjectDirInfo` 相应改为「按平台计算」的访问器：

```go
func (p *Project) LibDir(platform string, mode BuildMode) string
func (p *Project) OutputBinDir(platform string, mode BuildMode) string
// 单平台项目内部仍返回 lib/<mode>、output/<mode>/bin
```

`ResolveExecutable()` 同步按目标平台查找可执行文件。

---

## 7. CMake 生成

单平台：生成结果**保持现状**（`link_directories(lib/<mode>)`、`output/<mode>/bin`）。

多平台：引入 `${PM_TARGET_PLATFORM}`，由构建时传入（`pm build --platform linux-arm64`
或在 `CMakeLists.txt` 里 `set(PM_TARGET_PLATFORM "linux-amd64" CACHE STRING "")`）：

```cmake
link_directories(${CMAKE_SOURCE_DIR}/lib/${PM_TARGET_PLATFORM}/${CMAKE_BUILD_TYPE})
set(EXECUTABLE_OUTPUT_PATH ${CMAKE_SOURCE_DIR}/output/${PM_TARGET_PLATFORM}/${CMAKE_BUILD_TYPE}/bin)
```

交叉编译还需要工具链文件，`pm build --platform` 在平台 ≠ 宿主时自动追加
`-DCMAKE_TOOLCHAIN_FILE=<project>/config/toolchain-<platform>.cmake`（存在才加）。
工具链文件由用户提供，本工具不生成。

> 注意 `${CMAKE_BUILD_TYPE}` 是 `Debug`/`Release`（首字母大写），而目录名是小写 `debug`/`release`，
> 生成时需 `string(TOLOWER ...)` 转换——现有实现是直接把 `cfg.BuildMode` 拼进路径的，
> 多平台分支要注意别踩这个坑。

---

## 8. CLI

```text
pm pkg create <dir> --platform linux-amd64[,linux-arm64]   # 打包并声明支持的平台
pm pkg install <pkg>[@ver] -p proj [--platform linux-arm64]
pm deps install [proj] [--platform ...]
pm pkg list                                                 # 额外显示 platforms 列
pm deps list [proj]                                         # 显示目标平台是否可用
pm build [proj] [--platform linux-arm64]                    # 多平台项目选择构建目标
```

`pm deps list` 增加一列或在状态里体现平台可用性，例如：

```
Dependencies of Demo (target: linux-arm64):
  greeter   1.0.0   installed
  boost     1.83.0  unsupported (available: linux-amd64)
```

---

## 9. 兼容与迁移

| 对象 | 处理 |
|---|---|
| 旧 `.pm`（`lib/debug`） | 视为 `platforms = ["any"]`，可正常安装；装到非宿主平台时给警告 |
| 旧项目 `.pmproject` | 无 `target_platform` → 宿主；无 `platforms` → 单平台目录，路径不变 |
| 旧 `index.json` | 无 `platforms` → 视为 `["any"]` |
| 本地仓库/缓存 | 归档本身与平台无关（一个包可含多平台），**不需要按平台拆分文件** |
| 已有构建产物 | 不迁移；重新 `pm build` 即可 |

关键点：**平台是「包内容 + 安装选择」的维度，不是仓库文件系统的维度**。
仓库仍是 `<name>/<version>.pm` 一个文件，避免同一版本散落成多个平台文件、难以保证一致性。

---

## 10. 顺带修掉的移植性隐患

`go/core/cmake.go` 目前对所有目标硬编码：

```go
flags = appendUnique(flags, ..., "-march=native", "-rdynamic")
```

- `-march=native`：用本工具构建出的库**绑定构建机 CPU 指令集**，换一台机器可能直接
  非法指令崩溃（SIGILL）。放进共享仓库尤其危险——这正是「包要跨机器使用」场景下的硬伤。
- `-rdynamic`：主要对可执行文件有意义，给库加它没有收益。

建议：

| 目标 | 处理 |
|---|---|
| 库（`static_lib` / `share_lib`） | 去掉 `-march=native`，改用 `-mtune=generic`（或不加） |
| 可执行文件 | 保留可选，但默认关闭；需要时由 `build_options` 显式声明 |
| `-rdynamic` | 只加到可执行目标 |
| 需要极限性能 | 在包的 `build_options` 里按平台显式指定（如 `platform_build_options["linux-amd64"]`） |

这条不在「平台维度」主线里，但和「把包发给别人用」是同一个问题，建议一并处理。

---

## 11. 代码改动点

| 文件 | 改动 |
|---|---|
| `go/core/platform.go`（新） | 平台标识解析/归一化、宿主探测、别名表、`Supported(platforms, target)` |
| `go/core/project.go` | `ProjectConfig` +`TargetPlatform`/`Platforms`；`ProjectDirInfo` 改为按平台的计算方法；`fillDirInfo` 相应调整；`ResolveExecutable` 按平台查找 |
| `go/core/cmake.go` | 多平台时用 `${PM_TARGET_PLATFORM}`；可选工具链文件；`-march=native`/`-rdynamic` 按目标类型区分 |
| `go/core/package.go` | `PackageManifest` +`Platforms`/`PlatformLinkLibs`；`PackageTarget` +`Platform`；`Install` 加平台选择与回退；`InstallToProject`/`InstallProjectDeps` 合并分平台链接库 |
| `go/cmd/cli.go` | `--platform` 参数；`pkg list` / `deps list` 展示平台；`pkg create` 声明平台 |
| `go/core/registry.go`（新，见 registry.md） | 索引按平台过滤候选版本 |
| 测试 | 平台归一化表驱动测试；多平台包安装到指定平台；平台不匹配报错；`any` 回退；多平台项目目录与 CMake 断言 |

---

## 12. 分阶段

1. **阶段 1（纯包侧，不碰项目布局）**：平台标识 + `manifest.platforms` + `lib/<platform>/<mode>/`
   解析与回退 + `--platform` 参数。旧包/旧项目完全不受影响，收益是「一个包能装多平台」。
2. **阶段 2**：项目 `target_platform`、多平台目录、CMake 平台变量与工具链文件。
3. **阶段 3**：ABI 后缀（glibc/musl、MSVC/MinGW）、分平台 `build_options`、交叉编译工具链模板。

---

## 13. 未决问题

1. 多平台项目的 `lib/<platform>/<mode>/` 是否需要一个「当前平台」快捷方式（软链）方便人工 `ls`？
2. 包内头文件按平台区分（`include/<platform>/`）是否值得支持，还是强制要求头文件平台无关？
3. 平台匹配是否需要「兼容族」概念（如 `linux-amd64` 可接受 `linux-386` 的 32 位库）？倾向不做。
4. 交叉编译工具链文件由用户提供还是工具生成模板？
5. `-march=native` 的默认值调整为 `generic` 会让现有项目的产物性能略降，是否需要一次性迁移提示？
