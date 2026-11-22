# 内置第三方依赖包

`pm` 可以随二进制自带一批常用的第三方依赖（当前是 GoogleTest），
用户 `pm pkg install gtest` 即可使用，无需自己编译或手工配置目录。

本文说明这套机制怎么工作，以及**怎么再加一个包**。

---

## 1. 设计原则

| 位置 | 内容 | 是否入 git |
|---|---|---|
| `third_party/<pkg>/` | 源码、`pkg.json`、`LICENSE` | ✅ **只有源码与元数据** |
| `go/core/builtin/` | 编译出的 `.pm`、源码包、`index.json` | ❌ 构建时生成，已 gitignore |
| pm 二进制 | 内嵌「预编译 `.pm` + 源码 tar.gz」 | — |
| `~/.project_manager/builtin/<key>/` | 运行时释放出的**只读仓库层** | — |

三条关键约定：

1. **仓库里不保存任何二进制**，`.a` / `.pm` 只在 `./build.sh` 运行时短暂存在。
2. **源码一定内嵌**。预编译产物只是"快路径"；一旦构建环境不匹配，自动回退到用内嵌源码本机构建。
3. **绝不使用 `-march=native`**。第三方库的构建旗标固定为 `-O2 -mtune=generic`，
   否则产物绑死构建机 CPU，换机器可能直接非法指令。

---

## 2. 目录结构

```
third_party/
├── registry.json              # 收录清单（人类可读）
└── gtest/
    ├── pkg.json               # 构建 + 暂存 + 导出规则
    ├── LICENSE                # 会一并打进 .pm
    └── src/                   # 裁剪后的源码（越精简越好）
        ├── CMakeLists.txt
        └── googletest/
            ├── CMakeLists.txt
            ├── include/gtest/
            └── src/
```

`src/build/` 是构建目录，已在 `.gitignore` 中。

---

## 3. `pkg.json` 字段

```json
{
  "name": "gtest",
  "version": "1.14.0",
  "description": "GoogleTest (built-in)",
  "license": "BSD-3-Clause",
  "lib_type": "static",
  "link_libs": ["gtest"],

  "build": {
    "system": "cmake",
    "src": "src",
    "args": ["-DBUILD_GMOCK=OFF", "-DINSTALL_GTEST=OFF",
             "-DCMAKE_POSITION_INDEPENDENT_CODE=ON"],
    "cxxflags": "-O2 -mtune=generic"
  },

  "stage": {
    "include": ["src/googletest/include/gtest"],
    "libs": ["src/build/lib/libgtest.a", "src/build/lib/libgtest_main.a"]
  }
}
```

| 字段 | 说明 |
|---|---|
| `name` / `version` | 包名与版本，决定仓库文件名 `<name>-<version>.pm` |
| `description` / `license` | 元数据；`LICENSE` 文件会一起打进包 |
| `lib_type` | `static` / `shared` |
| `link_libs` | 安装后自动写入项目 `deps.libs` 的链接库名 |
| `build.system` | 目前只支持 `cmake` |
| `build.src` | 源码根（相对包目录），cmake 的 `-S` |
| `build.args` | 传给 cmake 的额外参数 |
| `build.cxxflags` | `CMAKE_CXX_FLAGS`，**必须保持可移植** |
| `stage.include` | 目录**内容会平铺**到包内 `include/` |
| `stage.libs` | 库文件，会被复制到包内 `lib/release/` 与 `lib/debug/` |

### `stage.include` 的平铺规则（最容易踩）

安装时包内 `include/` 的内容会被平铺到 `<项目>/extern_inc/<包名>/`。
所以要让 `#include <gtest/gtest.h>` 生效，源码里的 `include/gtest/` 必须**整体平铺**：

```
stage.include: ["src/googletest/include/gtest"]
  → 包内 include/{gtest.h, gtest-*.h, internal/}
  → 安装后  extern_inc/gtest/{gtest.h, gtest-*.h, internal/}
  → 于是     #include <gtest/gtest.h>  ✓
```

如果写成 `["src/googletest/include"]`，会得到 `extern_inc/gtest/gtest/gtest.h`，
`#include <gtest/gtest.h>` 就会报"找不到头文件"。子目录（如 `internal/`）会保留。

---

## 4. 构建流程

```bash
./build.sh              # 默认：先 thirdparty，再编译 pm
./build.sh thirdparty   # 只重建内置包
./build.sh --no-thirdparty   # 跳过内置包（本地快速迭代用）
./build.sh clean        # 清理产物，保留 PLACEHOLDER.txt
```

`build.sh` 调用 `go run ./cmd/pkgtool`，它做的事：

```
读 third_party/*/pkg.json
  → 算源码树内容哈希，命中缓存则跳过（10.6s → 0.5s）
  → cmake 配置 + 构建（Release）
  → 按 stage 规则暂存出 manifest.json + include/ + lib/ + LICENSE
  → 用 core.CreatePackageWithOptions 打包成 .pm（复用 pm 自己的打包逻辑）
  → 打一个源码 tar.gz（不含 build/）供兜底构建
  → 写 go/core/builtin/index.json（含构建环境元数据）
go build  → go:embed 把上述产物编进 pm
```

> `pkgtool` 直接 import `core`，因此**不依赖已经编译好的 pm 二进制**，
> 避免了"用 pm 打包 pm 自己要内嵌的包"的鸡生蛋问题。

`index.json` 里每个包都记录了构建环境，用于运行时判断能否直接用预编译产物：

```json
"built": {
  "os": "linux", "arch": "amd64",
  "compiler": "c++ (Ubuntu 11.4.0-1ubuntu1~22.04.3) 11.4.0",
  "cxx_abi": 1,
  "flags": "-O2 -mtune=generic",
  "built_at": "2026-09-13T08:17:23Z"
}
```

---

## 5. 运行时行为

**两层仓库，用户仓库优先：**

```
~/.project_manager/repo/                用户仓库（可写）
~/.project_manager/builtin/<key>/       内置层（只读，首次使用时从二进制释放）
```

- `<key>` 是 `index.json` 的源码树哈希，所以升级 pm 后会自动释放到新目录，不会串版本。
- 释放时会逐个校验 sha256，校验不过就重新释放。
- 内置包**不可删除**：`pm pkg remove gtest` 会明确报错。
- `pm pkg list` 会标注来源（`[builtin]`）。
- 用户在用户仓库里放同名同版本的包，会**覆盖**内置包。

**安装时的解析顺序：**

```
1. 用户仓库 / 内置层里直接命中，且构建环境匹配（OS/Arch）
     → 解压安装，零等待
2. 命中了但环境不匹配（例如 linux 编译的包拿到 darwin 上用）
     → 释放内嵌源码，本机 cmake 构建（约 10 秒）
     → 产物打包后缓存进用户仓库，之后秒装
3. 两边都没有该包
     → 若是内置包清单里的，同样走第 2 步
     → 否则报错
```

> 当前只比对 **OS / Arch**。编译器 / glibc 版本差异属于"软性差异"——
> 可能能链接，也可能出符号问题。这类情况目前不做自动判断，遇到时可以用
> `pm pkg remove <name>` 删掉用户仓库里的缓存副本，再安装即可触发源码兜底重建。

---

## 6. 新增一个内置包

以 header-only 的库（最省事）为例：

```bash
# 1) 建目录，放源码
mkdir -p third_party/mylib/src
cp -r /path/to/mylib/include third_party/mylib/src/
cp /path/to/mylib/LICENSE third_party/mylib/

# 2) 写 pkg.json
cat > third_party/mylib/pkg.json <<'JSON'
{
  "name": "mylib",
  "version": "1.2.0",
  "description": "MyLib (built-in)",
  "license": "MIT",
  "lib_type": "static",
  "link_libs": [],
  "build": { "system": "cmake", "src": "src", "args": [],
             "cxxflags": "-O2 -mtune=generic" },
  "stage": { "include": ["src/include"], "libs": [] }
}
JSON

# 3) 写进收录清单
#    编辑 third_party/registry.json 的 packages 数组，加上 "mylib"

# 4) 构建并验证
./build.sh thirdparty
pm pkg list | grep mylib
```

纯头文件库可以省掉 `build` 步骤（`stage.libs` 留空），但当前 `pkgtool` 仍会跑一次 cmake；
如果源码没有 CMakeLists，把它改成"只暂存不构建"需要给 `pkgtool` 加一个 `"build": {"system": "none"}` 分支。

### 收录标准

| 建议 | 原因 |
|---|---|
| 优先收**头文件库**（nlohmann/json、fmt header-only） | 零构建成本、无 ABI 风险 |
| 只收**体积小、编译快**的 | 内置包会进二进制，影响 clone 与分发体积 |
| 单个源码树控制在 1–2 MB 内 | 当前 gtest 裁剪后 1.2 MB / 47 文件 |
| 内置总量设上限（建议 ~10 MB） | 超了就改成"按需下载"而不是内置 |
| boost / openssl 这类大库**不要内置** | 体积与构建时间都不可接受，应走下载式方案 |

### 裁剪源码

第三方仓库往往包含大量测试、文档、示例，裁剪后再入库：

```bash
# gtest 的例子
rm -rf src/googletest/test src/googletest/samples src/googletest/scripts src/googletest/docs
# 4.5 MB → 1.2 MB
```

裁完**务必重新 `./build.sh thirdparty` 验证仍能构建**。

---

## 7. 注意事项

| 项 | 说明 |
|---|---|
| 许可证 | 分发二进制必须带许可证文本。`LICENSE` 会被打进 `.pm`，`pkg.json` 里的 `license` 字段用于展示 |
| ABI | C++ 静态库绑 libstdc++ 版本与 `_GLIBCXX_USE_CXX11_ABI`。这也是必须保留源码兜底的原因 |
| `gtest_main` | 它自带 `main()`，与项目默认生成的 `main/main.cc` 冲突。所以 `link_libs` 只放 `gtest`；需要时在项目 `targets[].link_libraries` 里手动加 `gtest_main` |
| pthread | glibc 2.34+ 已并入 libc，无需显式链接；更老的系统才需要 |
| 平台维度 | 内置包目前只按 OS/Arch 比对。要发布多平台预编译包，需要先做 [design/platform.md](design/platform.md) |
| 影响二进制体积 | 每加一个包，pm 二进制增加约「.pm + 源码 tar.gz」的大小（gtest 约 +1 MB） |

---

## 8. 故障排查

**`Warning: 内置包释放失败: ... checksum`**
二进制里的内置产物损坏或磁盘写入异常。删掉 `~/.project_manager/builtin/` 重新运行即可。

**安装内置包时现场编译了很久**
说明预编译产物被判为不兼容（OS/Arch 不同），正在走源码兜底。这是预期行为。

**`内置包 X 不适用于当前环境，源码兜底构建也失败`**
兜底构建失败，通常是缺少 cmake / 编译器。先确认 `cmake --version`、`c++ --version` 可用。

**重新构建 pm 后内置包没更新**
`pkgtool` 按源码树哈希缓存。改了 `pkg.json` 或源码会自动失效；也可以 `./build.sh clean` 后重建。

**构建时报 `pattern builtin: no matching files found`**
`go/core/builtin/` 下的 `PLACEHOLDER.txt` 被删了。`go:embed` 要求目录里至少有一个非 `.`/`_` 开头的文件。
