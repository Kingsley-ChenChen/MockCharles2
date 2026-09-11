# MockCharles

本机桌面代理与 Mock 工具，按设备、规则、规则集、流量四个模块组织。

当前为 **0.2 开发验收版**：已打通 HTTP/HTTPS 转发与固定 Mock、指定域名解密、本机 CA 生成与公共证书下载。完整产品规划见 [项目计划](docs/project-plan.md)，本阶段验收见 [HTTPS 验收清单](docs/testing/https-certificates.md)，上一阶段见 [HTTP 验收](docs/testing/desktop-foundation.md)。

## 运行

Windows 构建产物：`build/bin/MockCharles.exe`。双击运行，不需要部署服务器。代理默认关闭，手动开启后才监听；默认地址 `0.0.0.0:8888`，可在开启前修改。

配置保存在 `%APPDATA%/MockCharles/config.db`；macOS 使用系统用户配置目录。开发测试可通过 `MOCKCHARLES_DATA_DIR` 指定隔离目录。不要把测试数据目录用于日常配置。

## 已可使用

- 自动发现实际来源 IP；保存设备备注、项目和多个规则集关联；每台设备同时启用一个规则集。
- 创建项目、固定接口规则和有序规则集；接口按方法与完整 HTTP URL 精确匹配，方法 `*` 匹配任意方法。
- 项目通用请求/响应 Header；接口响应 Header 最后覆盖。未绑定设备不改写 Header。
- 真实 HTTP/HTTPS 转发；按规则集顺序使用第一条命中的固定响应。HTTPS 仅指定域名解密，其他目标显示为加密隧道。
- 本机 CA 手动生成、持久复用、指纹/有效期展示、公共证书保存和手机下载；不自动安装信任。
- 本次会话流量的所有 IP / 单个 IP 筛选、时间列表、域名与路径目录、Header 与正文详情。
- SQLite 持久保存配置；应用重启不自动开启代理，不恢复流量。

## 当前边界

- HTTPS 解密域名为精确域名/IP，每行一个，不含协议、端口或通配符。修改设置只影响新连接。设备到代理协商 HTTP/1.1，暂不承诺 HTTP/2-only 客户端、HTTP/3/QUIC 或证书固定的兼容性。
- 未选域名只做隧道，无法应用 Header/Mock 或查看其正文；上游证书仍正常验证，不允许忽略证书错误。
- 顺序执行、随机响应、响应样本/业务记录、删除策略、MCP、目录导出均未交付。本版没有流量导入。
- Header 当前支持设置/覆盖，不支持删除操作。为避免破坏消息长度，禁止配置 `Content-Length` / `Transfer-Encoding`。
- 流量最多保留最近 500 条，每个请求/响应正文最多捕获前 64 KiB；转发正文不因此截断。容量是当前实现上限，待后续容量设计调整。
- 普通请求当前使用 30 秒上限，未解密隧道采用 30 秒空闲超时。停止代理立即关闭 CONNECT 隧道，普通请求最多等待 3 秒。该值是开发版临时实现参数，完整退出策略尚待确认。
- 当前验证 Windows x64；macOS 构建及手机 HTTPS 信任需要后续实机验证。

## 开发与验证

需要 Go、Node/npm、Wails v2 和平台依赖。当前验证 Go 1.27.1、Node 22.20.0、npm 10.9.3、Wails 2.15.0。依赖版本由 `go.mod/go.sum` 与 `frontend/package-lock.json` 固定。

```powershell
cd frontend
npm ci
npm test
npm run build
cd ..
go test ./...
go vet ./...
wails build -skipbindings -s
```

运行 `wails dev` 后，在其输出的开发服务器地址可调试真实 Go 桥接。单独 `npm run dev` 没有 Go 后端，页面会明确提示连接不可用。

浏览器测试：`frontend/ui-smoke.cjs` 只在测试进程注入桥接替身，测试脚本不进入生产包。`frontend/desktop-smoke.cjs` 则使用真实 Wails 桥接和临时 HTTP 上游，要求应用以空的隔离数据目录运行，依次通过 UI 新建项目、规则、规则集，发现设备并验证转发 → Mock → 停用后转发。测试通过 `PLAYWRIGHT_PATH` 和可选 `EDGE_PATH` 指定本机 Playwright 与浏览器。

`frontend/https-smoke.cjs` 使用真实 Wails 桥接生成 CA、通过代理下载公共 DER 并检查指纹、由 Node 建立验证证书的 TLS CONNECT、验证默认 443 端口的 HTTPS Mock 和流量展示。测试不安装系统信任，不获取 CA 私钥。
