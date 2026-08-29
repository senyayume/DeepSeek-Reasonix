# Windows 安装目录内聚数据与全局工作区状态设计

日期：2026-08-29
状态：已批准，待实现计划
适用分支：`upstream-sync-merge`（基于上游 `main-v2` 的 `eac151f2c`）

## 目标

在长期维护的 fork 中，以两个相互独立的提交恢复并更新以下产品行为：

1. Windows 正式桌面版自动把稳定安装根下的 `data` 子目录作为 Reasonix home。
2. 全局工作区的会话与项目记忆直接使用全局状态目录，不再映射到 `projects/<workspace-slug>`。

安装位置不是固定值。若安装根为 `D:\Reasonix`，数据根为 `D:\Reasonix\data`；若安装根为 `F:\P\Reasonix`，数据根为 `F:\P\Reasonix\data`。

## 非目标

- 不修改“存储与路径”设置页，不增加可编辑路径控件。
- 不写入 Windows 用户或系统环境变量；只为 launcher 启动的桌面子进程设置环境。
- 不迁移测试安装 `D:\Reasonix2` 的 AppData 数据。
- 不自动迁移旧 `D:\Reasonix`。用户会保留 `D:\Reasonix - 副本`，删除旧安装目录并进行全新安装；数据恢复在新版本验收后单独执行。
- 不重新提交 `929110ce0`，因为上游已经包含等价的 NSIS UTF-8 修复。
- 不照搬 `e8036d511` 和 `13f30cfb2` 中与当前目标无关或已过时的改动。
- 不改变外部项目的隔离存储合同。真实外部项目仍使用 `data/projects/<workspace-slug>` 保存 Reasonix 内部状态。
- 不使用 `.gitattributes` 或自定义 merge driver 静默保留 fork 代码。
- 不支持用户直接运行 `versions/<version>/reasonix-desktop.exe` 作为正式入口；正式入口是 `Reasonix.exe`、开始菜单快捷方式或 `reasonix-launcher.exe`。

## 目录合同

以 `<InstallRoot>` 表示用户选择的稳定安装根：

```text
<InstallRoot>/
├─ Reasonix.exe
├─ reasonix-launcher.exe
├─ current.json
├─ versions/
├─ uninstall.exe
└─ data/
   ├─ config.toml
   ├─ .env
   ├─ archive/
   ├─ cache/
   ├─ desktop/
   │  └─ topic-state-v1.sqlite
   ├─ global-workspace/
   ├─ memory/
   ├─ mcp-state/
   ├─ plugins/
   ├─ projects/
   ├─ sessions/
   └─ skills/
```

`global-workspace` 与 `projects` 必须保持同级，因为职责不同：

- `global-workspace` 是 Agent 可以读取和修改的真实工作目录。
- `projects` 是 Reasonix 为外部项目保存会话、记忆和元数据的内部状态目录。

两者不得互相嵌套，避免工作区索引、Git、搜索工具或状态清理逻辑误处理另一类数据。

少量不属于用户数据的系统运行面可以继续使用操作系统目录，例如 WebView2 运行缓存以及为了跨配置实例收敛而故意放在 OS cache 下的安全锁。配置、凭据、会话、记忆、扩展、技能及 Reasonix 管理的缓存必须位于 `data`。

## 提交一：Windows 安装根内聚数据

建议提交信息：

```text
fix(windows): keep desktop data under install-root data
```

### Owner 与挂点

`internal/desktoplauncher` 已经负责解析稳定安装根、读取 `current.json` 并启动活动桌面二进制，因此它是该行为的唯一 owner。

预计改动：

- 新增 `internal/desktoplauncher/portable_home.go`：计算并验证桌面子进程的数据根。
- 新增 `internal/desktoplauncher/portable_home_test.go`：覆盖平台、版本、安装路径和显式覆盖。
- 修改 `internal/desktoplauncher/launcher.go`：在创建桌面子进程后注入环境，保留一个小挂点。

不修改 `desktop/main.go`。launcher 已经掌握稳定安装根，桌面二进制不应从 `versions/<version>` 反向猜测安装根。

### 行为规则

1. 仅当目标平台为 Windows 且 launcher 是正式发行版时启用。
2. `buildVersion` 为空或为 `dev` 时保持上游开发行为。
3. 若继承环境中已有非空 `REASONIX_HOME`，显式值优先，不注入 fork 默认值。
4. 否则计算 `<InstallRoot>/data`，创建目录后把 `REASONIX_HOME` 只传给桌面子进程。
5. 不清除 `REASONIX_STATE_HOME` 或 `REASONIX_CACHE_HOME`；用户显式设置的高级覆盖继续优先。
6. 无法解析安装根或创建数据根时启动失败并返回具体错误，不允许静默退回 AppData。

### 数据流

```text
Reasonix.exe / reasonix-launcher.exe
  -> desktoplauncher.ResolveInstallRoot()
  -> <InstallRoot>/data
  -> child env: REASONIX_HOME=<InstallRoot>/data
  -> reasonix-desktop.exe
  -> internal/config 现有路径解析
```

现有 `internal/config` 会自然得到：

- home、配置、凭据、默认 state root：`<InstallRoot>/data`
- cache：`<InstallRoot>/data/cache`
- plugins：`<InstallRoot>/data/plugins`

## 提交二：全局工作区状态不进入项目存储

建议提交信息：

```text
feat(desktop): keep global workspace state out of project store
```

### Owner 与挂点

路径合同由 `internal/config` 和 `internal/memory` 持有，Desktop 只消费结果。

预计改动：

- 修改 `internal/config/paths.go`：
  - 提供单一的全局工作区根定义 `<ReasonixHome>/global-workspace`。
  - 提供规范化、Windows 大小写不敏感的全局工作区判断。
  - `ProjectSessionDir(globalWorkspaceRoot)` 返回 `SessionDir()`。
- 修改 `internal/memory/store.go`：
  - 全局工作区的项目作用域记忆目录为 `<state-root>/memory`。
  - 真实外部项目继续使用 `<state-root>/projects/<workspace-slug>/memory`。
- 修改 `desktop/tabs.go`：现有 `globalWorkspaceRoot()` 只委托给配置层真源。
- 新增或扩充路径与记忆测试；测试放在风险所属包，不把全部断言堆进 Desktop。

因为采用全新安装，本提交不增加启动迁移器，不修改 `desktop/app.go`，也不创建自动搬运旧目录的逻辑。

### 行为规则

全局工作区：

```text
workspace: <ReasonixHome>/global-workspace
sessions:  <state-root>/sessions
memory:    <state-root>/memory
```

工作区目录固定在 Reasonix home 下；`REASONIX_STATE_HOME` 只改变 sessions/memory 所在的 state 根，不改变工作区目录位于 home 的合同（默认 state 根即 home，两者一致）。此处 `<ReasonixHome>` 即目录合同中的 `<InstallRoot>/data`。

真实外部项目 `D:\Projects\Unholy_Maiden`：

```text
workspace: D:\Projects\Unholy_Maiden
sessions:  <state-root>/projects/d--projects-unholy_maiden/sessions
memory:    <state-root>/projects/d--projects-unholy_maiden/memory
```

全局工作区不得创建对应的 `projects/<global-workspace-slug>`。外部项目的 `projects/<workspace-slug>` 是正常隔离状态，不得删除或与全局记忆混合。

当前上游已把全局 Desktop Topic 元数据放在 `<state-root>/desktop/topic-state-v1.sqlite`；本提交不改变该合同。

## 错误处理与数据安全

- 任何数据根解析或创建失败都必须显式失败，不允许回落到 AppData 后继续运行。
- 路径判断使用规范化后的路径相等语义，不使用字符串前缀判断。
- Windows 路径比较忽略大小写，并复用当前路径解析约定。
- 不删除用户目录，不自动移动旧数据，不覆盖备份。
- 以后从 `D:\Reasonix - 副本` 恢复数据时，先检查新旧格式，再选择性迁移 `memory`、`plugins`、`skills`、配置或会话；不得直接覆盖整个新安装根。

## 验收

### 提交一

单元测试至少覆盖：

1. `D:\Reasonix` 解析为 `D:\Reasonix\data`。
2. `F:\P\Reasonix` 解析为 `F:\P\Reasonix\data`，证明没有硬编码盘符。
3. Windows 正式版本启用。
4. `dev` 版本不启用。
5. 非 Windows 不启用。
6. 显式 `REASONIX_HOME` 保持不变。
7. 显式 state/cache 覆盖不被清除。
8. 数据根创建失败时不回落 AppData。

### 提交二

单元和集成测试至少覆盖：

1. 全局工作区根位于 `<ReasonixHome>/global-workspace`。
2. 全局工作区会话目录等于 `SessionDir()`。
3. 全局工作区记忆目录位于 `<state-root>/memory`。
4. 外部项目仍使用 `projects/<workspace-slug>`。
5. Windows 路径不同大小写仍识别为同一个全局工作区。
6. 创建全局会话和记忆后，不出现全局工作区 slug 目录。
7. 打开真实外部项目后，项目专属状态仍正常创建和读取。

### 构建与实际安装

1. 运行受影响 Go 包的完整测试。
2. 运行 Desktop 相关测试和 Windows 构建。
3. 用干净目录安装构建产物，启动正式 launcher。
4. 在“存储与路径”确认：
   - 会话与记忆：`<InstallRoot>\data`
   - 缓存：`<InstallRoot>\data\cache`
   - 扩展：`<InstallRoot>\data\plugins`
5. 创建全局对话与记忆，核对真实磁盘路径。
6. 打开外部项目，核对项目状态隔离。
7. 覆盖安装下一构建，确认仍复用同一个 `data`。

只有单元测试、Desktop 测试、Windows 构建及安装后真实启动路径均通过，才能声明功能完成。设置页截图只能证明显示结果，不能替代磁盘路径和重启验证。

## 文档与长期维护

- 两项功能分别提交，测试和相关说明跟随各自提交。
- fork 的长期差异集中记录在单独的新文档中，避免大面积重写上游 README。
- 每次合并上游后，复核 launcher 环境注入、全局工作区路径判断和 memory store 分支；不得假设无文本冲突就代表语义仍正确。
- 不配置自动“保留我方”代码合并规则。上游若改变 launcher、路径 owner 或安装布局，显式复核比静默保留旧逻辑更安全。
- stage 时逐文件添加，不使用 `git add .`。

## 提交顺序

1. `fix(windows): keep desktop data under install-root data`
2. `feat(desktop): keep global workspace state out of project store`

提交一独立建立安装目录内聚数据能力；提交二只改变全局工作区与外部项目的状态归属。两者可以分别审查、测试、撤销或重放。
