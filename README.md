# ZZZ Multi-Agent Drive Optimizer

《绝区零》本地驱动盘库存管理与基础面板配装器。

## 项目来源

本项目 Fork 并改进自 TonyUB 的原版项目：

- **Upstream / 原版仓库：** [TonyUB/ZZZ_Drive_Disc](https://github.com/TonyUB/ZZZ_Drive_Disc)

感谢原作者提供驱动盘库存、基础面板计算、配装搜索及 WebUI 的初始实现。本分支在原版基础上主要增加了多角色无冲突分配、角色与属性优先级、多个 2 件套候选、独立角色目标存储、界面可读性优化，以及 ZZZ Scanner Next 的自动查找、下载、完整性校验和启动功能。

如需查看原始实现、历史版本或对比本分支改动，请访问上述 Upstream 仓库。

## 当前功能

- 导入 ZZZ Scanner Next、通用驱动盘 JSON 或本软件完整备份。
- 手工录入及驱动盘截图 OCR。
- 卡片/表格库存视图，支持筛选、编辑、归属和占用管理。
- 单角色 4+2 配装，可同时选择多个 2 件套候选并合并排序前 20 套。
- 暴击率、暴击伤害、攻击、生命、防御和异常精通目标及 1～6 级属性优先级。
- 多角色顺序配装：高优先级角色先锁定驱动盘，后续角色不会重复使用。
- 库存与角色要求分文件保存。
- 找不到本地 Scanner 时，可从官方 Latest Release 下载、校验并启动 Windows x64 self-contained 包。

本工具计算角色等级、所选核心技、音擎基础/高级属性、驱动盘主副属性和可静态计算的 2 件套效果。依赖动作、层数、敌人或队伍条件的增益通常只作为实战参考；它不是完整战斗伤害模拟器。

## Scanner

Scanner 使用独立项目 [ZztIsolation/ZZZ-Scanner.Next](https://github.com/ZztIsolation/ZZZ-Scanner.Next)。

点击页面中的“打开驱动盘扫描器”后，程序会先寻找已有 Scanner；没有找到时查询官方 GitHub Latest Release，下载 self-contained 包并按官方 manifest 校验 ZIP 与全部文件，安装到 EXE 同目录的 `scanner` 文件夹后启动。

扫描完成后，在配装器顶部选择“导入 JSON（自动识别）”，导入 Scanner 生成的 `export.json`。

## 数据文件

默认库存：

```text
%APPDATA%\ZZZDriveBuilder\state.json
```

多角色要求单独保存在同目录的：

```text
state-character-targets.json
```

页面可将库存路径改到其他目录。`storage-config.json` 只记录自定义库存路径。

## 从源码构建

要求 Go 1.22 或兼容的新版本：

```powershell
go test ./... -skip '^TestBundledScannerIntegrity$'
go build -o ZZZ_Multi_Agent_Drive_Optimizer_v2.0.0.exe .
```

`TestBundledScannerIntegrity` 只用于检查另行制作的内置 Scanner 发行包；纯源码仓库没有预装 Scanner，因此普通源码测试应跳过该项。

## 目录

- `main.go`：本地服务、存储、Scanner 安装和配装算法。
- `main_test.go`：后端、算法、界面标记和安全性回归测试。
- `web/`：嵌入程序的离线 WebUI 与图片资源。
- `tools/`：前端及数据校验脚本。
- `DRIVE_DISC_INTEROP.md`：通用驱动盘 JSON 互通说明。
- `ZZZ_Multi_Agent_Drive_Optimizer_v2.0.0.exe`：当前 Windows x64 发行版。
- `ZZZ_Multi_Agent_Drive_Optimizer_v2.0.0_使用说明.*`：当前 Markdown 与 HTML 使用说明。
- `ZZZ_Multi_Agent_Drive_Optimizer_v2.0.0_使用说明书.pdf`：可打印的 PDF 说明书。

当前发行版文件直接放在仓库根目录，方便下载和核对；发布 GitHub Release 时也可以复用这些文件作为附件。

## 免责声明

本项目是社区工具，不隶属于或受 HoYoverse 认可。游戏相关素材版权归其权利人所有。Scanner 和配装器的使用风险由使用者自行判断。
