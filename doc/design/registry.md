# 远端包仓库设计（Registry）

> **状态：暂缓实施（设计稿）**
> 当前项目**只使用本地仓库** `~/.project_manager/repo/`，没有远端存储的需求。
> 本文仅作为将来需要共享包时的参考，**代码中没有任何远端实现**——
> `pm repo add/serve`、`pm pkg publish/yank`、`pm cache` 等命令都不存在，
> `~/.project_manager/config.json` 也没有引入。阅读时请勿当作待办计划。
>
> 目标（将来）：把「只在本地仓库里的包」扩展为**可共享的远端仓库**——
> 项目能把自己的库打包上传，其它项目能按名字/版本拉取、校验并自动写入配置参与构建。
>
> 当前已可用的本地流程见 [`package.md`](../package.md)；
> 平台维度设计见 [`platform.md`](platform.md)（同样是设计稿）。

---

## 1. 现状与差距

| 能力 | 现状 | 位置 |
|---|---|---|
| 打包 `.pm` | ✅ | `PackageManager.CreatePackageWithOptions` |
| 本地仓库 | ✅ `~/.project_manager/repo/` | `PackageManager.Init` |
| 归档摘要 | ✅ `<file>.pm.sha256` 边车 + `VerifyPackage` | `go/core/package.go` |
| 目录/版本解析 | ✅ 以 manifest 为权威（不依赖文件名） | `findPackageFile` |
| 安装到项目 + 写配置 | ✅ `extern_inc/`、`lib/<mode>/`、`dependencies`、`deps.libs` | `InstallToProject` |
| CMake 自动生效 | ✅ `include_directories(extern_inc)`、`link_directories(lib/<mode>)`、`target_link_libraries` | `GenerateCMakeLists` |
| **远端仓库** | ❌ | 本文 |
| **上传 / 下载** | ❌ | 本文 |
| 本地缓存 | ❌ | 本文 |

**关键结论**：从「拿到一个本地 `.pm` 文件」到「项目能 `#include` 并链接」这一段**已经完整实现**。
远端仓库只需要负责一件事：**把 `<name>@<version>` 解析成本地归档路径**（必要时下载并校验）。

---

## 2. 术语

| 术语 | 含义 |
|---|---|
| 包（package） | 一个 `.pm` 归档（tar.gz），含 `manifest.json` + `include/` + `lib/{debug,release}/` |
| 仓库（registry） | 提供索引与包下载/上传的服务，也可是静态文件站点或共享目录 |
| 本地仓库 | 已安装包的落盘位置 `~/.project_manager/repo/`（与远端同名，注意区分） |
| 缓存（cache） | 从远端下载回来的 `.pm` 副本，`~/.project_manager/cache/` |
| 项目依赖 | `.pmproject` 里的 `dependencies[]` + `deps.libs[]` |

---

## 3. 数据模型

### 3.1 manifest.json（包内，已有）

见 `doc/package.md`。本文建议**新增两个可选字段**（向后兼容，旧包不受影响）：

| 字段 | 说明 |
|---|---|
| `license` | 许可证标识，便于内网合规审查 |
| `homepage` | 项目主页 |

> 注意：**不能**把包自身的 sha256 写进 `manifest.json`——摘要由归档内容决定，写进去会形成循环依赖。
> 摘要一律放在索引（`index.json`）或边车文件里。

### 3.2 index.json（仓库侧，新增）

仓库的权威索引。客户端搜索、版本解析、完整性校验都基于它，**不再从 `.pm` 文件名反推元数据**。

```json
{
  "schema": 1,
  "generated_at": "2025-09-10T12:00:00Z",
  "base_url": "",
  "packages": [
    {
      "name": "greeter",
      "version": "1.0.0",
      "description": "simple greeting library",
      "author": "alice",
      "license": "MIT",
      "lib_type": "static",
      "link_libs": ["greeter"],
      "deps": [{ "name": "baselib", "version": "1.0.0" }],
      "platforms": ["linux-amd64", "linux-arm64"],
      "filename": "greeter-1.0.0.pm",
      "sha256": "02e0b4db563901de90ae193f66e18ad164535ede8abeb2b6f61d02a4a2b31fd5",
      "size": 2869,
      "uploaded_at": "2025-09-10T11:00:00Z",
      "uploaded_by": "alice",
      "yanked": false
    }
  ]
}
```

- `schema`：索引格式版本，客户端不认识的版本直接报错，避免静默误解析。
- `sha256` / `size`：**必需**。客户端下载后必须校验，不匹配就丢弃。
- `platforms`：该包提供的平台列表（`<os>-<arch>`，或 `["any"]`）。
  客户端**在下载前**就据此过滤候选版本，避免下完才发现平台不匹配。
  缺失时视为 `["any"]`（旧索引兼容）。详见 [`platform.md`](platform.md)。
- `yanked`：撤回标记。`true` 表示「不再被新解析选中，但已锁定的项目仍可下载」，用于替代物理删除。
- `filename`：允许包文件名与 `<name>-<version>.pm` 不一致（当前 `findPackageFile` 已支持）。

### 3.3 仓库目录布局（服务端）

```
<registry-root>/
├── index.json                                  # 索引（可由 index.d/ 或扫描生成）
├── packages/
│   └── greeter/
│       ├── 1.0.0.pm
│       └── 1.0.0.pm.sha256
└── .registry/
    ├── tokens.json                             # 可选：写操作 token（也可用外部鉴权）
    └── audit.log                               # 可选：上传/删除审计
```

`packages/<name>/<version>.pm` 的层级布局避免包名/版本里的 `-` `.` 造成歧义——
这正是本地仓库原来踩过的坑（版本号含 `-` 时无法定位，现已修复）。

---

## 4. HTTP API

Base：`<registry-url>/api/v1`

### 4.1 读接口（默认匿名）

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/index` | 返回 `index.json`；支持 `ETag` / `If-None-Match`（304 省流量） |
| `GET` | `/index?q=<kw>&name=<n>&limit=&cursor=` | 服务端过滤/分页（可选实现，客户端也能本地过滤） |
| `HEAD` | `/packages/{name}/{version}` | 只返回 `Digest` / `Content-Length`，用于判断是否要重新下载 |
| `GET` | `/packages/{name}/{version}` | 下载 `.pm`，响应头带 `Digest: sha-256=<base64>` 与 `ETag` |

### 4.2 写接口（需鉴权）

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/packages` | `multipart/form-data`，字段 `package`；query `force=0\|1` |
| `PUT` | `/packages/{name}/{version}` | 直接以 body 上传单个 `.pm`（便于脚本 `curl -T`） |
| `DELETE` | `/packages/{name}/{version}` | 物理删除（建议优先用 yank） |
| `POST` | `/packages/{name}/{version}/yank` | 标记撤回，不删文件 |

上传时服务端流程：

1. 大小限制（默认 64 MiB，可配置），超限返回 `413`。
2. 解压读取 `manifest.json`，取出 `name`/`version`。
3. **校验请求路径与 manifest 一致**（`PUT` 时），不一致返回 `422` —— 防止把 A 包的内容挂在 B 的名字下。
4. 计算 `sha256`。
5. 冲突检查：已存在同 `name@version`
   - 内容摘要相同 → 幂等成功 `200`；
   - 摘要不同且 `force=0` → `409`，提示「请升版本号，而不是覆盖」；
   - `force=1` 且 token 有 `write:force` 权限 → 覆盖。
6. 写临时文件 → `fsync` → 原子 `rename` 到 `packages/<name>/<version>.pm`。
7. 更新 `index.json`（同样原子替换）。

### 4.3 状态码与错误体

```json
{ "error": { "code": "version_conflict", "message": "greeter@1.0.0 already exists with a different digest" } }
```

| 码 | 含义 |
|---|---|
| 400 | 请求格式错误 |
| 401 / 403 | 未认证 / 无权限 |
| 404 | 包或版本不存在 |
| 409 | 版本冲突（摘要不同） |
| 413 | 包体积超限 |
| 422 | manifest 与请求路径不一致 / manifest 缺失 |
| 500 | 服务端错误 |

### 4.4 鉴权

- 写操作：`Authorization: Bearer <token>`。
- token 由服务端 `tokens.json` 或外部反代（OAuth2/OIDC/JWT）提供，服务端只需一个 `Authenticator` 接口。
- 权限粒度：`read` / `write` / `write:force` / `admin`。

---

## 5. 客户端行为

### 5.1 全局配置 `~/.project_manager/config.json`（新增）

```json
{
  "schema": 1,
  "default_registry": "internal",
  "registries": [
    { "name": "internal", "url": "https://pm.example.com", "token_env": "PM_TOKEN" },
    { "name": "mirror",   "url": "https://mirror.example.com" },
    { "name": "shared",   "url": "file:///srv/pm-repo", "kind": "filesystem" }
  ],
  "cache": { "dir": "", "max_size_mb": 2048 },
  "install": { "verify": true }
}
```

- `token_env` 优先于内联 `token`，避免明文写进家目录。
- `kind: filesystem` 时 `url` 是本地/挂载路径（`file://` 或裸路径），实现零服务端共享。
- `cache.dir` 为空时用 `~/.project_manager/cache`。

### 5.2 依赖解析顺序

`Install` 需要一个本地归档路径。解析顺序：

1. `--pkg-file <path>`：调用方显式指定的本地 `.pm`（`findPackageFile` 已支持直接传路径）。
2. **本地缓存** `cache/.../`：命中且 `VerifyPackage` 通过 → 直接用。
3. **本地仓库** `~/.project_manager/repo/`。
4. **远端 registry**（按配置顺序；`default_registry` 优先）：
   - `GET /index`（带 ETag 缓存）→ 找到 `name@version` 且 `platforms` 含目标平台（或 `any`）的条目；
   - `HEAD`/`GET` 下载到 `cache/tmp-xxxx`；
   - `VerifyPackage(tmp, index.sha256)` 校验；
   - 再解压校验 manifest 的 `name`/`version` 与索引一致（防串包）；
   - 原子 `rename` 入缓存。
5. 全部失败 → 报错并列出已尝试的仓库。

> 递归依赖：`manifest.deps` 里的子依赖同样走这套解析，所以远端依赖树可以自动拉全，
> 不需要每个包都在本地仓库里预置。

### 5.3 缓存布局

```
~/.project_manager/cache/
└── greeter/
    └── 1.0.0/
        ├── greeter-1.0.0.pm
        └── greeter-1.0.0.pm.sha256
```

按 `name/version` 分目录、避免同名不同版本互相覆盖；`sha256` 边车用于离线复用与二次校验。
淘汰策略：`pm cache clean [--max-size N]`，默认按最后访问时间 LRU。

### 5.4 安装（复用现有代码）

拿到本地路径后，**完全走现有流程**：

```
registry.Fetch(dep) ──► <cache>/greeter/1.0.0/greeter-1.0.0.pm
        │
        ▼
PackageManager.InstallWithDeps ──► extern_inc/<pkg>/、lib/{debug,release}/
        │
        ▼
InstallToProject ──► 写 .pmproject 的 dependencies + deps.libs
        │
        ▼
pm build ──► CMake 自动 include/link
```

即：**“下载后自动加到配置让项目自己加载”这一环无需新代码**，只需把远端文件喂给
`InstallWithDeps` / `InstallToProject`。

---

## 6. 发布流程（publish）

`pm pkg publish <dir|file.pm> [--registry <name>] [--force]`

1. 目录 → 先本地打包（临时文件，不入本地仓库），拿到 `.pm` 与 sha256。
2. 读取全局配置，确定目标 registry 与 token。
3. `HEAD /packages/{name}/{version}`：
   - 不存在 → 上传；
   - 存在且摘要相同 → 幂等，直接提示「已存在且内容一致」；
   - 存在且摘要不同 → **默认报错**，提示升版本号；`--force` 才覆盖（需要权限）。
4. `POST /packages`（multipart）。
5. 服务端返回最终摘要，客户端核对一致后提示成功。

> 建议配套 `pm pkg yank <name>@<version>`：撤回错误版本但保留可复现性，
> 比直接删除安全得多。

---

## 7. 完整性与安全

| 风险 | 对策 | 状态 |
|---|---|---|
| 下载中断 / 半包 | 临时文件 + `sha256` 校验通过后才 `rename` 入缓存 | 校验函数已实现，接入待做 |
| 归档被篡改 | `index.json.sha256` + `VerifyPackage` | 已实现（本地边车，远端复用同一函数） |
| 串包（A 的内容挂 B 的名） | 解压后校验 manifest 的 `name`/`version` 与请求一致 | 设计，待实现 |
| 路径穿越（`../../etc/x`） | `extractArchive` 的 `cleanDest` 前缀检查 | 已实现 |
| 版本被覆盖导致不可复现 | 同 `name@version` 默认拒绝覆盖（本地已实现，服务端沿用同一策略） | 本地已实现 |
| 中间人 | 强制 HTTPS；`file://`/内网自签需显式配置 | 设计 |
| 供应链投毒 | 阶段 3：minisign/ed25519 签名 + 信任列表 | 后续 |

**最小信任模型**：内网可用「HTTPS + 只读镜像 + 写操作 token」。公网场景再叠加签名。

---

## 8. 版本解析

- **当前**：精确版本；`version` 为空时取版本号最大的一个（`compareVersion`）。
- **修复项**：解析一律基于 manifest/索引，不再从文件名反推，因此 `1.0.0-rc1`、`1.2.0-beta.1`
  这类**含 `-` 的版本号已可正常安装**（此前会失败）。
- **后续**：语义化范围（`^1.2.0`、`~1.2`、`>=1.0,<2.0`）在客户端解析成候选集合，
  取满足条件的最高版本；配合 `pm.lock` 固化「name/version/sha256」，保证团队/CI 可复现。
- 预发布版本默认不参与 `^`/`~` 匹配，需显式指定（遵循 semver 惯例）。

---

## 9. CLI 命令设计

```text
# 仓库配置
pm repo add <name> <url> [--token-env VAR] [--default]
pm repo list
pm repo use <name>                  # 设为 default_registry
pm repo remove <name>

# 发布
pm pkg publish <dir|file.pm> [--registry <name>] [--force]
pm pkg yank <name>[@ver] [--registry <name>]

# 安装（复用现有命令，增加来源控制）
pm pkg install <pkg>[@ver] [-p project] [--registry <name>|--offline]
pm deps install [project] [--offline]

# 缓存
pm cache list | clean [--max-size MB]

# 自建仓库（阶段 1）
pm repo serve --root <dir> [--addr :8080] [--token-file <path>] [--read-only]
```

新增的命令都只是薄封装；核心逻辑放在 `core` 包里，便于 TUI 复用。

---

## 10. 代码改动点

| 文件 | 改动 |
|---|---|
| `go/core/package.go` | ✅ 已加 `FileSHA256` / `VerifyPackage` / `ChecksumSidecarPath` / `CreateOptions` / `ManifestOf` / `LibraryFiles` / `HasLibrary`；`findPackageFile` 改为按 manifest 多源定位 |
| `go/core/registry.go`（新） | `Resolver` 接口；`httpRegistry`、`fsRegistry`；`RegistrySet`（多仓库顺序尝试） |
| `go/core/cache.go`（新） | 缓存布局、命中判断、原子落盘、LRU 清理 |
| `go/core/globalcfg.go`（新） | 读写 `~/.project_manager/config.json` |
| `go/core/package.go` | `PackageManager` 增加可选 `resolver`；`Install` 通过 resolver 取包（默认仍只查本地仓库） |
| `go/cmd/cli.go` | `pm repo *`、`pm pkg publish/yank`、`pm cache *`、`--registry/--offline` |
| `go/registryd/main.go`（新） | `pm repo serve` 的实现：索引维护 + 上传/下载 + token 鉴权 |
| `go/core/registry_test.go`（新） | 用 `httptest.Server` 跑 publish → install → build 全链路 |

### 关键接缝

```go
// Resolver 把「依赖」解析成本地 .pm 归档路径（可能触发下载与校验）
type Resolver interface {
    Resolve(dep Dependency) (localPath string, err error)
}

// PackageManager 默认只用本地仓库；注入 registry resolver 后自动具备远端能力。
func (pm *PackageManager) SetResolver(r Resolver)
```

`Install` / `InstallWithDeps` 内部把 `findPackageFile` 换成 `resolver.Resolve`，
其余（解压、复制、写配置、CMake 生成）**完全不变**——这是本设计改动面最小的原因。

---

## 11. 分阶段实施

### 阶段 0：静态托管（零服务端代码，可立即落地）

`index.json` + `packages/**` 放到 nginx / 对象存储 / GitHub Releases，
客户端只发 `GET`（`kind: "static"`）。发布用现成命令：

```bash
pm pkg create ./greeter -o /tmp/greeter-1.0.0.pm
# 把 .pm 与更新后的 index.json 上传到静态站点（rsync / aws s3 cp / gh release upload）
```

**收益**：立刻获得「集中放包 + 其它项目拉取安装」的能力；
**代价**：索引需要脚本生成，没有权限控制。

### 阶段 1：自建服务（推荐起点）

`pm repo serve` + `pm pkg publish` + 客户端缓存与校验。
带 token 的读写、`yank`、`ETag`，一条命令起一个内网仓库。

### 阶段 2：多仓库与治理

多 registry 优先级、镜像/代理缓存（上游不可达时回退）、审计日志、命名空间（`org/name`）。

### 阶段 3：可复现与信任

语义化版本范围、`pm.lock`（name/version/sha256）、包签名与信任列表、CI 集成。

---

## 12. 测试策略

| 层次 | 内容 |
|---|---|
| 单元 | 索引解析、版本比较与范围匹配、缓存命中/淘汰、摘要校验 |
| 集成 | `httptest.Server`：publish → 另一项目 install → `pm build` → 运行输出正确 |
| 异常 | 摘要不匹配、半包（连接中断）、`409` 冲突、manifest 与路径不符、超大包、404 版本 |
| 离线 | `--offline` 只用缓存/本地仓库；缓存缺失时给出明确错误 |
| 兼容 | 无 `.sha256` 边车的旧包仍可安装；无 `index.json` 的目录按本地仓库处理 |

---

## 13. 兼容性与迁移

- `manifest.json` 只做**加法**，旧包无需重新打包即可入库。
- `.pm` 格式不变，本地仓库与远端仓库共用同一套归档与安装逻辑。
- 无 `.sha256` 边车的包：`VerifyPackage` 跳过校验（已实现），不阻塞老用户。
- 客户端未配置任何 registry 时，行为与现在**完全一致**（纯本地）。
- 索引 `schema` 不匹配时明确报错，不做「尽力解析」。

---

## 14. 未决问题

1. 同名包出现在多个 registry 时的优先级：默认第一个命中，还是要求显式指定？
2. 是否需要命名空间（`org/name`）以避免内网重名？
3. 大包（>100 MiB）是否需要分片上传 / 断点续传？
4. 是否提供上游镜像代理（proxy cache），让内网仓库透明代理公共源？
5. `yank` 后是否允许重新发布同版本（建议：不允许，必须升版本）。
6. 是否把依赖树解析结果写入 `pm.lock` 并纳入版本控制（推荐纳入）。
