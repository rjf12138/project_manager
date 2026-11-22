# 项目配置参考（`config/.pmproject`）

每个项目在 `config/.pmproject` 保存一份 JSON 配置。它由 `pm init` 生成，
之后可以用 TUI 的 Configure 页面修改，也可以直接用编辑器改（改完 `pm build` 会重新生成 `CMakeLists.txt`）。

**位置**：`<project>/config/.pmproject`
**生成**：`pm init` 按默认值创建；`pm build` 读取它生成 `<project>/CMakeLists.txt`

---

## 1. 字段一览

### 基本信息

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `name` | string | 项目名 | 同时作为默认构建目标名 |
| `version` | string | `"0.1.0"` | 传给 CMake 的 `project(... VERSION ...)` |
| `path` | string | 创建时的绝对路径 | 加载项目时自动写入，**不用手改** |
| `uuid` | string | 自动生成 | 项目唯一标识 |
| `build_system` | string | `"cmake"` | 预留字段 |
| `cmake_minimum_version` | string | `"3.10"` | `cmake_minimum_required` |
| `language` | string | `"CXX"` | CMake `project(... LANGUAGES ...)` |
| `cxx_standard` | int | `17` | 生成 `CMAKE_CXX_STANDARD`（`language` 为 `CXX` 且 > 0 时） |

### 目录（相对项目根，生成时会加 `${CMAKE_SOURCE_DIR}/` 前缀）

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `source_dirs` | []string | `["src"]` | 递归收集其中的 `.cc/.cpp/.cxx/.c`，同时作为头文件搜索路径 |
| `include_dirs` | []string | `["inc"]` | 本项目对外头文件目录 |
| `extern_inc_dir` | string | `"extern_inc"` | 外部依赖头文件目录（包管理器写入） |
| `lib_dir` | string | `"lib"` | 外部库根目录，实际使用 `lib/<build_mode>/` |
| `output_dir` | string | `"output"` | 产物根目录，实际使用 `output/<build_mode>/` |

> 这几个字段**必须写相对路径**：生成器一律拼成 `${CMAKE_SOURCE_DIR}/<值>`。
> 需要绝对路径时用下面的 `extra_include_dirs` / `extra_lib_dirs`。

### 构建

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `build_mode` | string | `"debug"` | `debug` 或 `release`，决定 `output/<mode>/`、`lib/<mode>/` 与编译选项 |
| `main_file` | string | `"main.cc"` | `main/` 目录下的入口文件名 |
| `run_args` | string | 空 | `pm run` 传给程序的参数（`pm run App -- args` 会写回这里） |
| `generate_type` | string | `"executable"` | `executable` / `static_lib` / `share_lib`；**单目标或未配置 `targets` 时决定生成类型** |
| `build_options` | object | 见下 | 按构建模式的宏与优化选项 |
| `targets` | []Target | 一个可执行目标 | 多目标配置；单目标时其 `type` 由 `generate_type` 同步，见下 |

`build_options` 默认值：

```json
{
  "debug":   { "defines": ["DEBUG"],  "optimization": "-O0" },
  "release": { "defines": ["NDEBUG"], "optimization": "-O3" }
}
```

生成时取**当前 `build_mode` 对应**的那一组，`defines` 展开成 `-D<名字>`，
追加到内置的 `-g -ggdb`（debug）或 `-O3`（release）之后。

`targets[]`：

| 字段 | 说明 |
|---|---|
| `name` | 目标名 |
| `type` | `executable`、`static_lib`（别名 `library`）、`share_lib` |
| `source_files` | 额外源文件（相对项目根）；为空时用 `main_file` |
| `link_libraries` | 该目标额外链接的库 |

`targets` 为空时会用 `name` 生成一个默认可执行目标。

### 依赖

| 字段 | 类型 | 说明 |
|---|---|---|
| `dependencies` | []Dependency | 声明的依赖：`name` / `version` / `lib_type` |
| `deps.libs` | []string | 链接库名 → `target_link_libraries` |
| `deps.inc_dirs` | []string | 依赖提供的额外头文件目录 |
| `deps.lib_dirs` | []string | 依赖提供的额外库目录 |
| `deps.src_dirs` | []string | 预留字段，当前未使用 |
| `extra_include_dirs` | []string | 额外头文件目录，**按原样写入**（可写绝对路径） |
| `extra_lib_dirs` | []string | 额外库目录，**按原样写入**（可写绝对路径） |

`deps.*` 由包管理器维护，`pm pkg install` / `deps add` / `deps remove` 会自动增删；
也可以在 TUI 的 `Include Dirs` / `Lib Dirs` 页面手工编辑。
`extra_*` 则是纯手工字段，包管理器不会动它。

---

## 2. 完整示例

```json
{
  "name": "myapp",
  "version": "0.1.0",
  "path": "/home/me/work/myapp",
  "uuid": "8e5c1f7a-1d2b-4c3e-9f10-2a3b4c5d6e7f",
  "build_system": "cmake",
  "cmake_minimum_version": "3.10",
  "language": "CXX",
  "cxx_standard": 17,

  "source_dirs": ["src"],
  "include_dirs": ["inc"],
  "extern_inc_dir": "extern_inc",
  "lib_dir": "lib",
  "output_dir": "output",

  "main_file": "main.cc",
  "run_args": "",
  "build_mode": "debug",
  "generate_type": "executable",

  "dependencies": [
    { "name": "greeter", "version": "1.0.0", "lib_type": "static" }
  ],
  "deps": {
    "libs": ["greeter"],
    "inc_dirs": [],
    "lib_dirs": []
  },

  "build_options": {
    "debug":   { "defines": ["DEBUG"],  "optimization": "-O0" },
    "release": { "defines": ["NDEBUG"], "optimization": "-O3" }
  },

  "targets": [
    {
      "name": "myapp",
      "type": "executable",
      "source_files": ["main/main.cc"]
    }
  ],

  "extra_include_dirs": [],
  "extra_lib_dirs": []
}
```

---

## 3. 配置项 → 生成的 CMake

| 配置 | 生成的 CMakeLists.txt |
|---|---|
| `cmake_minimum_version` | `cmake_minimum_required(VERSION ...)` |
| `name` / `version` / `language` | `project(... VERSION ... LANGUAGES ...)` |
| `cxx_standard` | `set(CMAKE_CXX_STANDARD ...)` |
| `build_options[build_mode]` | `set(CXX_FLAGS ...)` → `CMAKE_CXX_FLAGS` |
| `output_dir` + `build_mode` | `EXECUTABLE_OUTPUT_PATH` / `LIBRARY_OUTPUT_PATH` |
| `source_dirs` | `file(GLOB_RECURSE SRC_FILES ...)` |
| `include_dirs` / `extern_inc_dir` / `source_dirs` | `include_directories(...)` + `target_include_directories(...)` |
| `deps.inc_dirs` / `extra_include_dirs` | 同上（前者相对路径加前缀，后者原样） |
| `lib_dir` + `build_mode` | `link_directories(${CMAKE_SOURCE_DIR}/lib/<mode>)` |
| `deps.lib_dirs` / `extra_lib_dirs` | `link_directories(...)` |
| `targets[]` / `generate_type` | `add_executable` / `add_library(STATIC\|SHARED)`；单目标或未配置 `targets` 时以 `generate_type` 为准，多目标时按各自 `type` |
| `deps.libs` | `target_link_libraries` |
| `main_file` | `targets[].source_files` 为空时作为入口源文件 |

另外生成器会固定加入 `set(CMAKE_DEBUG_POSTFIX d)`，并内置一组编译旗标
（debug 为 `-g -ggdb`，release 为 `-O3`）。

---

## 4. 与 TUI 配置页的对应

| TUI 显示项 | 配置字段 | 快捷键 |
|---|---|---|
| Name / Version / Path | `name` / `version` / `path` | —（只读展示） |
| Build Mode | `build_mode` | `b` 切换 |
| Generate Type | `generate_type` | `g` 切换（单目标时决定实际生成类型） |
| C++ Standard | `cxx_standard` | — |
| Main File | `main_file` | `f` 在 `main/` 下扫描到的文件间循环 |
| Dependencies | `dependencies` | `enter` 进入依赖管理页（查看状态 / 安装 / 重装 / 卸载） |
| Libraries | `deps.libs` | `enter` 进入子页面增删改 |
| Lib Dirs | `deps.lib_dirs` | 同上 |
| Include Dirs | `deps.inc_dirs` | 同上 |
| Source Dirs | `source_dirs` | 同上 |

`Libraries` / `Lib Dirs` / `Include Dirs` 三项由 `pm pkg install` 自动填充，一般无需手工维护。

---

## 5. 已知限制

| 项 | 说明 |
|---|---|
| `deps.src_dirs` | 预留字段，代码中未被使用。 |
| `build_system` | 预留字段，目前只支持 `cmake`。 |
| 绝对路径 | 只有 `extra_include_dirs` / `extra_lib_dirs` 支持绝对路径；其余目录字段都会被拼上 `${CMAKE_SOURCE_DIR}/`。 |
| 编译旗标 | `-march=native`、`-rdynamic` 等内置旗标目前硬编码，无法通过配置关闭。 |
| 跨平台 | 配置里没有 OS/架构维度，一个项目的产物只对应构建机；设计稿见 [design/platform.md](design/platform.md)。 |
