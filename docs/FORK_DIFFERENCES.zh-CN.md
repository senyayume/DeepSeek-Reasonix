# Fork 长期差异记录

本文件记录 DeepSeek-Reasonix fork 相对上游 `main-v2` 的长期产品差异。每项差异说明 owner、行为合同、非目标、验收入口，以及合并上游后必须显式复核的位置。上游 README 与路径文档保持上游原样；两者冲突时以本文件为准。

适用分支基线：`upstream-sync-merge`（基于上游 `eac151f2c`）。

## 差异一：Windows 正式桌面版数据内聚在安装根 `data`

提交：`fix(windows): keep desktop data under install-root data`

### 合同

Windows 正式发行版中，launcher 解析稳定安装根 `<InstallRoot>`，创建 `<InstallRoot>/data` 并把它作为 `REASONIX_HOME` 只注入桌面子进程环境。`<InstallRoot>/data` 随即成为配置、凭据、会话、记忆、扩展、技能与缓存的家目录；安装位置不是固定盘符，随用户选择的安装根移动。

### Owner

- `internal/desktoplauncher`（`portable_home.go` 计算/校验数据根，`launcher.go` 注入子进程环境）。
- `internal/config` 既有路径解析消费注入后的 `REASONIX_HOME`，自身不改语义。

### 行为边界

- 仅 Windows 正式构建启用；`buildVersion` 为空或 `dev` 保持上游开发行为。
- 继承环境中非空 `REASONIX_HOME`、`REASONIX_STATE_HOME`、`REASONIX_CACHE_HOME` 显式值优先，不被覆盖或清除。
- 安装根解析或数据根创建失败时启动失败并返回具体错误，不允许静默回落 AppData。
- 不写用户/系统环境变量，不改“存储与路径”设置页。

### 合并上游后的复核点

- `internal/desktoplauncher` 的 `ResolveInstallRoot()` 合同与 `current.json` 解析是否仍成立。
- launcher 组装 `cmd.Env` 的挂点是否仍在启动 `exec.Command` 之前。
- `internal/config` 对 `REASONIX_HOME` 的解析是否仍把 home、默认 state root、cache、plugins 收敛到同一数据根。

## 差异二：全局工作区状态不进入 `projects/<slug>` 项目存储

提交：`feat(desktop): keep global workspace state out of project store`

### 合同

```text
data/global-workspace/   全局工作区真实工作目录（Agent 可读写）
data/sessions/           全局工作区会话
data/memory/             全局工作区记忆
data/projects/<slug>/    真实外部项目的会话与记忆（隔离内部状态）
```

`data/global-workspace` 与 `data/projects` 必须保持同级，不得互相嵌套：前者是工作目录，后者是 Reasonix 为外部项目保存的状态。全局工作区不得产生自己的 `projects/<global-workspace-slug>` 目录；外部项目的 `projects/<workspace-slug>` 是正常隔离状态，不得删除或与全局记忆混合。

Desktop Topic 元数据仍在上游合同位置 `<state-root>/desktop/topic-state-v1.sqlite`，本差异不改变它。

### Owner

- `internal/config/paths.go`：`GlobalWorkspaceRoot()` 单一定义全局工作区根；`IsGlobalWorkspaceRoot()` 做规范化后的路径相等判断（Windows 忽略大小写，不做字符串前缀判断）；`ProjectSessionDir()` 仅对全局根返回 `SessionDir()`。
- `internal/memory/store.go`：`StoreFor()` 仅对全局工作区返回 `<state-root>/memory`，外部项目继续 `projects/<slug>/memory`。
- `desktop/tabs.go`：`globalWorkspaceRoot()` 只委托配置层真源，不持有路径语义。

### 已知回归修复（随本差异维护）

全局会话目录并入根 `sessions` 后，boot 启动期的 cleanup-pending reconciliation 与 `buildTabController` 操作同一目录：早期检查拒绝 cleanup-pending 固定会话后，reconcile 可能在恢复步骤重读 `tab.SessionPath` 前清除标记并完成删除，已删路径会被误当作空占位符绑定。`desktop/tabs.go` 记录被拒会话的规范键，恢复步骤把它作为 `rejectedKey` 传入 `loadPinnedTabSessionWithPreload`，由该函数在重读路径匹配时跳过加载，阻断该 TOCTOU。合并上游改动 `buildTabController`、`loadPinnedTabSessionWithPreload`（含其签名）或 `agent.CleanupPending*` 时必须复核该保护仍然成立。

### 部分可用性测试的前提

全局会话目录唯一后，`desktop/session_catalog_app_test.go` 中两个部分可用性测试通过“恢复标签页仍指向旧 `projects/<global-workspace-slug>` 路径”制造第二个全局目录。就地升级（无迁移器）时这是旧全局目录存活的唯一形态：仅当恢复标签页持有旧路径时进入会话目录索引，重建控制器后标签页绑定新会话。

### 合并上游后的复核点

- `config.ProjectSessionDir` 的全局分支与 `IsGlobalWorkspaceRoot` 的 Windows 大小写语义。
- `memory.StoreFor` 的全局分支未影响 `DirFor`、scope、归档与索引语义。
- `desktop` 侧 `globalWorkspaceRoot()` 仍为单行委托。
- 上述 cleanup-pending TOCTOU 保护。

## 验收入口

```text
go test ./internal/desktoplauncher ./cmd/reasonix-launcher ./internal/config ./internal/memory -count=1
go -C desktop test . -count=1
```

安装级验收（设置页显示、磁盘路径、重启读取、覆盖安装复用 `data`）见 `docs/superpowers/specs/2026-08-29-windows-install-local-data-design.md` 与对应实施计划。

## 数据恢复边界

本 fork 不自动迁移旧目录。从 `D:\Reasonix - 副本` 或其他旧安装恢复数据时，必须先检查新旧目录格式，再选择性迁移 `memory`、`plugins`、`skills`、配置或会话；不得直接覆盖整个新安装根。WebView2 运行缓存与安全锁等系统运行面留在 OS 目录属于预期，不属于用户数据迁移失败。
