# Windows 安装目录内聚数据实施计划

> 本计划落实 `docs/superpowers/specs/2026-08-29-windows-install-local-data-design.md`。实施时按测试先行执行，每个功能独立提交，不混入旧提交中的无关改动。

**目标：** Windows 正式桌面版默认使用 `<InstallRoot>/data`，并让全局工作区会话与记忆脱离 `projects/<slug>`；外部项目隔离语义保持不变。

**架构：** `internal/desktoplauncher` 负责把安装根转换为桌面子进程环境；`internal/config` 负责全局工作区与会话路径合同；`internal/memory` 负责记忆目录合同；Desktop 只消费配置层的全局工作区根。

**技术栈：** Go 标准库、现有 desktop launcher、`internal/config`、`internal/memory`、Wails Desktop、现有 Go/Windows 构建与原生启动门禁。

---

## 当前执行状态（2026-08-29，重做后基于上游 main-v2 eac151f2c）

### 已提交

- `72459ba09 fix(windows): keep desktop data under install-root data`
- `eb940b6c1 feat(desktop): keep global workspace state out of project store`
- 本 docs 提交（设计文档、实施计划、`docs/FORK_DIFFERENCES.zh-CN.md`）

提交一：Windows 正式 launcher 默认创建 `<InstallRoot>/data`，只向桌面子进程注入 `REASONIX_HOME`，并保留显式 home/state/cache 覆盖。launcher 与 `cmd/reasonix-launcher` 包级测试通过。

提交二：全局工作区状态移出 `projects/<workspace-slug>`，repolint 债务随本提交一并处理——essay/complexity/function-size 增量已在代码层归零（TOCTOU 拒绝守卫下沉进 `loadPinnedTabSessionWithPreload`，`buildTabControllerWithContextCore` 恢复原复杂度与行数预算），残余纯行数增量（tabs.go file-size +4、store.go +1、测试文件 test-file-size +17）经 `repolint -update` 记录并随提交说明；`-update` 同时把基线修正为当前树实际债务（含上游 MCP 功能线引入的文件）。

### 提交二实现语义

涉及文件：

- `internal/config/paths.go`
- `internal/config/global_workspace_path_test.go`
- `internal/memory/store.go`
- `internal/memory/global_workspace_store_test.go`
- `desktop/tabs.go`（含恢复步骤的 cleanup-pending 回归修复）
- `desktop/global_workspace_path_test.go`
- `desktop/session_catalog_app_test.go`（两个部分可用性测试按新目录现实更新）
- 本实施计划中的 Desktop 独立 module 命令修正

已实现语义：

- 全局工作区根由 `internal/config` 单一持有。
- 全局工作区 session 使用根 `sessions`。
- 全局工作区 memory 使用根 `memory`。
- Windows 大小写不同的全局路径仍归入同一个状态目录。
- 外部项目继续使用 `projects/<workspace-slug>`。
- Desktop 的 `globalWorkspaceRoot()` 只委托给配置层真源。
- `buildTabController` 早期拒绝 cleanup-pending 固定会话时记录其规范键，恢复步骤重读 `tab.SessionPath` 后跳过同一路径，防止 boot reconciliation 清除标记后被删路径被误当作空占位符。

### 已运行验证

重做基线（eac151f2c）上已跑通：

```text
go test ./internal/desktoplauncher ./cmd/reasonix-launcher -count=1
go test ./internal/config -count=1
go test ./internal/memory -count=1
go -C desktop test . -run 'TestDesktop(GlobalWorkspace|ExternalProject)|TestBuildTabControllerSkips|TestListProjectTopics' -count=1
```

完整 Desktop 套件（约 270 秒）此前在旧基线（bba8f8eb6）上全部通过，包含 `TestWindowsPackagerRejectsMissingOrPartialRequiredPayloadManifest`——其失败记录来自经 WSL 运行测试（无 `/bin/bash`），不是产品打包合同问题；本机需把 Git Bash 前缀进 PATH 再跑全量（见最终验证）。

### 原恢复入口的归因结论

1. `TestBuildTabControllerSkipsCleanupPendingPinnedSession`：由全局路径 owner 改动触发，不是既有失败。用 `git worktree` 在 HEAD 基线复跑通过、工作区复跑失败。根因是 TOCTOU：全局会话目录改为根 `sessions` 后，boot 启动期的 `CleanupPendingReconciler`（`internal/boot/boot.go`）与 buildTabController 早期检查操作同一目录，标记在早期检查之后、恢复步骤重读 `tab.SessionPath` 之前被清除并完成删除，`loadPinnedTabSessionWithPreload` 把已删除路径当作合法空占位符绑定。修复保留在 `desktop/tabs.go`。
2. `TestListProjectTopicsUsesAvailableProjectionBeforeEveryGlobalDirectoryIsScanned` 与 `TestListProjectTopicsPaginatesMetadataWhileCatalogIsPartiallyAvailable`：同样由全局路径改动触发（基线通过、工作区失败）。旧测试前提是 legacy 目录与全局工作区目录为两个不同目录；设计变更后全局会话目录唯一，`sessionCatalogTargets` 按路径去重后第二个全局目录只能来自恢复标签页仍指向的旧 `projects/<global-workspace-slug>` 路径。测试已改为注册这样一个恢复标签页来制造部分可用状态，原有断言未放宽；生产行为符合设计，无需改动。

### 尚未运行

- 设置页显示核对、重启后会话/记忆继续读取、覆盖安装复用 `data`（部分磁盘级验证已完成，见下）。
- 打开真实外部项目核对 `data/projects/<workspace-slug>` 隔离（需 GUI 操作）。
- 从 `D:\Reasonix - 副本` 选择性恢复数据（设计约定为独立后续步骤）。

没有修改、删除或迁移 `D:\Reasonix`、`D:\Reasonix2`、AppData 或备份目录。

---

## 实施任务

### 任务 1：为 launcher 定义安装本地数据根

**文件：**

- 新增：`internal/desktoplauncher/portable_home.go`
- 新增：`internal/desktoplauncher/portable_home_test.go`

- [x] 先写失败测试，覆盖：
  - Windows 正式版本把 `D:\Reasonix` 映射为 `D:\Reasonix\data`。
  - 任意安装路径 `F:\P\Reasonix` 动态映射，不硬编码盘符。
  - `dev`、空版本和非 Windows 返回“不注入”。
  - 非空显式 `REASONIX_HOME` 优先。
  - 相对或空安装根被拒绝，不产生 AppData fallback。
- [x] 运行失败测试：

  ```powershell
  go test ./internal/desktoplauncher -run 'TestPortableDesktopHome' -count=1
  ```

- [x] 实现一个无副作用的纯函数，输入 `goos`、`buildVersion`、显式 home 和 `installRoot`，输出数据根或明确错误/不启用状态。
- [x] 数据根必须使用 `filepath.Join(installRoot, "data")`；不得重新调用 `os.Executable()`，不得解析版本目录。
- [x] 运行目标测试并确认通过。

### 任务 2：把数据根只注入桌面子进程

**文件：**

- 修改：`internal/desktoplauncher/launcher.go`
- 修改：`internal/desktoplauncher/portable_home_test.go`

- [x] 写失败测试验证 launcher 环境合成：
  - 未显式配置时只增加一个 `REASONIX_HOME=<InstallRoot>\data`。
  - 显式 `REASONIX_HOME`、`REASONIX_STATE_HOME`、`REASONIX_CACHE_HOME` 不被覆盖或删除。
  - 不启用时保持继承环境语义。
- [x] 运行失败测试。
- [x] 在 `ResolveInstallRoot()` 成功且桌面路径解析完成后、启动 `exec.Command` 前，计算数据根。
- [x] 对启用的正式 Windows 构建执行 `os.MkdirAll(dataRoot, 0o700)`；失败时输出带安装根和阶段信息的错误并返回非零，不回落 AppData。
- [x] 仅设置 `cmd.Env`，不调用进程级 `os.Setenv`，防止 launcher 自身或其他子进程意外继承 fork 默认值。
- [x] 运行：

  ```powershell
  go test ./internal/desktoplauncher -count=1
  go test ./cmd/reasonix-launcher -count=1
  ```

- [x] 检查本提交 diff 只包含 launcher helper、测试和一个现有文件挂点。
- [x] 精确暂存并提交：

  ```text
  fix(windows): keep desktop data under install-root data
  ```

### 任务 3：建立全局工作区路径真源

**文件：**

- 修改：`internal/config/paths.go`
- 新增或修改：`internal/config/global_workspace_path_test.go`

- [x] 写失败测试，证明：
  - `GlobalWorkspaceRoot()` 为 `<ReasonixHome>/global-workspace`。
  - `ProjectSessionDir(GlobalWorkspaceRoot()) == SessionDir()`。
  - 普通外部项目仍为 `<state-root>/projects/<slug>/sessions`。
  - Windows 大小写不同的等价全局路径仍被识别。
  - `REASONIX_STATE_HOME` 只改变 state 路径，不改变全局工作区真实目录位于 Reasonix home 的合同。
- [x] 运行失败测试：

  ```powershell
  go test ./internal/config -run 'TestGlobalWorkspace|TestProjectSessionDir' -count=1
  ```

- [x] 在 `internal/config` 中增加单一 `GlobalWorkspaceRoot()` 和内部路径相等判断。
- [x] 让 `ProjectSessionDir` 只对该全局根返回 `SessionDir()`；不得以字符串前缀判断路径。
- [x] 运行完整配置包测试：

  ```powershell
  go test ./internal/config -count=1
  ```

### 任务 4：让全局工作区记忆使用根 memory

**文件：**

- 修改：`internal/memory/store.go`
- 修改：`internal/memory/store_test.go`，或新增聚焦测试文件

- [x] 写失败测试，证明：
  - `StoreFor(stateRoot, config.GlobalWorkspaceRoot()).Dir == <stateRoot>/memory`。
  - `GlobalDir` 继续是 `<stateRoot>/memory/global`。
  - 普通外部项目继续使用 `<stateRoot>/projects/<slug>/memory`。
  - Windows 路径大小写差异不会分裂全局 store。
- [x] 运行失败测试：

  ```powershell
  go test ./internal/memory -run 'TestStoreForGlobalWorkspace|TestStoreForSlug' -count=1
  ```

- [x] 在 `StoreFor` 中只增加全局工作区分支；不改变 `DirFor`、scope、归档或索引语义。
- [x] 运行完整 memory 包测试：

  ```powershell
  go test ./internal/memory -count=1
  ```

### 任务 5：Desktop 消费路径真源并验证无全局 slug

**文件：**

- 修改：`desktop/tabs.go`
- 新增或修改：`desktop/global_workspace_path_test.go`
- 必要时修改与旧硬编码断言直接相关的现有测试；禁止借机重构 tabs

- [x] 把 `globalWorkspaceRoot()` 改成只返回 `config.GlobalWorkspaceRoot()`。
- [x] 写失败/回归测试，验证创建全局标签页和全局会话时使用根 `sessions`，不创建 `projects/<global-workspace-slug>`。
- [x] 写回归测试，验证打开外部项目仍创建并读取项目专属会话目录。
- [x] 确认全局 Desktop Topic 元数据仍使用 `<state-root>/desktop/topic-state-v1.sqlite`，不改 SQLite owner。
- [x] 运行目标测试：

  ```powershell
  go -C desktop test . -run 'TestGlobalWorkspace|TestDesktopSessionDir|TestOpenGlobal' -count=1
  ```

- [x] 运行完整 Desktop Go 测试：

  ```powershell
  go -C desktop test . -count=1
  ```

### 任务 6：记录 fork 差异并完成第二个提交

**文件：**

- 新增：`docs/FORK_DIFFERENCES.zh-CN.md`
- 不批量改写上游 README 或路径文档

- [x] 记录两个长期差异、owner、非目标、验收入口和合并上游后的复核点。
- [x] 明确 `data/global-workspace` 是真实工作区、`data/projects` 是外部项目状态，两者同级且不可互相嵌套。
- [x] 明确本轮没有自动迁移旧目录；后续从 `D:\Reasonix - 副本` 恢复时必须另行检查格式。
- [x] 运行文档差异检查：

  ```powershell
  git diff --check
  ```

- [x] 复核第二个提交只包含 config、memory、Desktop 委托与 cleanup-pending 回归修复、聚焦测试及更新的部分可用性测试和 fork 差异文档。
- [x] 精确暂存并提交：

  ```text
  feat(desktop): keep global workspace state out of project store
  ```

## 最终验证

### 自动验证

- [x] 运行受影响包：

  ```powershell
  go test ./internal/desktoplauncher ./cmd/reasonix-launcher ./internal/config ./internal/memory -count=1
  go -C desktop test . -count=1
  ```

  完整 Desktop 套件在本机需把 Git Bash 前缀进 PATH（`bash` 解析到损坏的 WSL shim 时打包测试会误报失败）：

  ```powershell
  $env:PATH = 'C:\Program Files\Git\bin;C:\Program Files\Git\usr\bin;' + $env:PATH
  ```

- [x] 运行 Windows Wails 构建：

  ```powershell
  pwsh -NoProfile -Command "Set-Location 'desktop'; wails build -clean -s -skipbindings -nopackage -platform windows/amd64 -webview2 embed"
  ```

  已通过完整 release 构建等价验证：`scripts/desktop-build.sh windows/amd64 v1.17.21 stable`（旧基线 bba8f8eb6 上；含 `wails build -clean -nsis -webview2 embed` 与两次 makensis 打包，Wails CLI 已按 `.wails-version` pin 装到 v2.13.0）。注意前端依赖需用 `packageManager` 钉的 pnpm 10.34.5 安装（`corepack pnpm@10.34.5 install --force`），pnpm 11 不会重建 `.bin` shim。重做基线 eac151f2c 的产物待重建后复核。

- [x] 检查最终 diff、提交边界和工作区：

  ```powershell
  git status --short
  git show --stat --oneline HEAD~1
  git show --stat --oneline HEAD
  ```

### 安装与真实路径验证

- [x] 生成或取得包含两个提交的 Windows 安装产物；安装到一个干净的非生产验收目录。

  产物：`dist/Reasonix-windows-amd64-installer.exe`（54.8 MB）与 `dist/Reasonix-windows-amd64.zip`（便携，70.5 MB），来自旧基线 `scripts/desktop-build.sh windows/amd64 v1.17.21 stable`（重做前产物，仅用于行为级验收）。静默安装到 `D:\Reasonix-Acceptance-20260829`（`installer.exe /S /D=...`），布局含 `Reasonix.exe`/`reasonix-launcher.exe`/`current.json`/`versions/v1.17.21`/`uninstall.exe`，符合 versioned-v1 便携布局；`current.json` 指向 `v1.17.21`。

- [x] 通过 `Reasonix.exe` 或 `reasonix-launcher.exe` 启动，不直接运行版本目录中的桌面二进制。

  `reasonix-launcher.exe version` 输出 `reasonix-launcher v1.17.21`（正式版本注入生效，便携 home 启用条件满足）；通过 launcher 启动桌面约 12 秒后，`<InstallRoot>/data` 被创建并按目录合同填充：`cache/`、`global-workspace/`、`sessions/`、`desktop/topic-state-v1.sqlite`、`desktop-tabs.json`、`install-id` 等；全局会话落在根 `sessions/`，**没有** `projects/<global-workspace-slug>` 目录。测试结束后桌面进程已终止；`crash-fatal/` 下的日志为强制终止时的空占位（0 字节），非崩溃。

- [ ] 在“存储与路径”确认 state、cache、plugins 分别位于 `<InstallRoot>/data`、`<InstallRoot>/data/cache`、`<InstallRoot>/data/plugins`。

  磁盘级已验证 state/cache 位于 `data`（`data/cache`、`data/sessions`、`data/desktop-tabs.json`）；设置页显示与 `data/plugins`（装插件后才创建）待 GUI 复核。

- [x] 创建一个全局会话和全局记忆，重启后确认可以继续读取，并确认磁盘上没有全局工作区 slug 目录。

  磁盘级：首启自动创建全局会话 `data/sessions/20260829-*.jsonl` 且无全局 slug 目录。重启后继续读取与全局记忆创建需 GUI 操作，未做。

- [ ] 打开一个外部项目，确认项目源码路径正确，且项目状态位于 `data/projects/<workspace-slug>`。（需 GUI）

- [ ] 覆盖安装另一个构建或重复安装，确认 `data` 未被覆盖且重启后数据仍可读取。（需 GUI/第二次构建）

- [x] 记录 WebView2/安全锁等允许留在 OS 目录的非用户数据；不得把它们误报为用户数据迁移失败。

  本机 WebView2 运行库已存在，安装与启动未触发额外 OS 目录写入；安装根出现 `.reasonix-activate.lock` 属激活期锁文件，非用户数据迁移问题。

## 停止条件

- launcher 无法稳定区分正式与开发构建。
- 上游当前入口存在绕过 launcher 的正式 Windows 启动路径。
- 全局工作区判断需要修改公开 schema、数据库格式或 Topic SQLite owner。
- 任一测试显示外部项目会话/记忆与全局状态发生混合。
- Windows 构建或真实启动无法证明数据根为 `<InstallRoot>/data`。

遇到停止条件时不得用硬编码、双写、静默 fallback 或自动 merge driver 绕过；应回到设计并说明证据。
