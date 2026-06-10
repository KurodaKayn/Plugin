# 插件化执行系统设计说明

## 1. 整体架构设计说明

### 1.1 架构边界

系统分两层：

- 主程序：负责读取输入、扫描插件、校验清单、管理状态、调度执行、汇总结果
- 插件：负责具体业务处理。插件不被主程序 import，只以独立可执行文件形式存在

核心边界是 JSON 协议。主程序只认识插件清单和 stdin/stdout 数据格式，不认识插件内部代码

### 1.2 模块划分

```
cmd/executor        CLI 入口：list、reload、unload、run
cmd/sample-plugin   Go 示例插件
internal/config     默认插件目录、默认超时
internal/contract   主程序和插件共享协议类型
internal/manager    插件扫描、清单校验、状态管理、依赖校验、reload/unload
internal/runner     插件进程启动、超时控制、输出解析、失败降级
plugins/            运行时插件目录
```

模块关系：

```
executor -> manager -> plugin.json
executor -> runner  -> plugin executable
runner   -> contract -> request/response/result
```

### 1.3 执行流程

1. 主程序读取插件目录，默认 `plugins/`
2. `manager` 扫描每个插件子目录
3. 每个插件目录必须有 `plugin.json`
4. 主程序校验名称、版本、入口文件、启用状态、依赖关系
5. 合法且启用插件进入可执行列表；非法插件保留错误状态
6. `runner` 按依赖顺序启动插件进程
7. 主程序通过 stdin 写入请求 JSON
8. 插件通过 stdout 返回响应 JSON
9. 主程序解析结果，处理失败、超时、非法输出、fallback
10. 主程序汇总所有插件结果

## 2. 插件系统核心设计思路

### 2.1 插件单元

插件 = 一个目录 + 一个清单 + 一个可执行入口。

```text
plugins/sample/
├── plugin.json
└── sample-plugin
```

`plugin.json` 描述插件元信息：

```json
{
  "name": "sample",
  "version": "1.0.0",
  "entry": "./sample-plugin",
  "enabled": true,
  "timeout_ms": 3000,
  "dependencies": [
    {
      "name": "base",
      "version": ">=1.0.0"
    }
  ],
  "failure_policy": {
    "fallback_data": {
      "message": "fallback response"
    }
  }
}
```

字段含义：

- `name`：插件唯一名
- `version`：插件版本
- `entry`：插件可执行文件，相对插件目录
- `enabled`：是否启用
- `timeout_ms`：单插件超时
- `dependencies`：依赖插件名和版本约束
- `failure_policy.fallback_data`：失败降级数据

### 2.2 通信协议

主程序传给插件：

```json
{
  "request_id": "req-001",
  "data": {
    "message": "hello"
  },
  "context": {
    "source": "executor"
  }
}
```

插件返回给主程序：

```json
{
  "ok": true,
  "data": {
    "message": "hello from plugin"
  },
  "error": ""
}
```

协议只要求 JSON，可执行文件语言不限。Go、Python、Node.js 都可接入，只要能读 stdin、写 stdout

### 2.3 加载与状态管理

加载阶段做这些事：

- 插件目录扫描。
- `plugin.json` 读取和 JSON 解析
- 插件名去重
- 入口路径限制在插件目录内
- 入口文件存在且可执行
- 禁用插件标记为 `disabled`
- 非法插件标记为 `invalid`，保留错误原因
- 依赖缺失、禁用、版本不满足、循环依赖 -> `invalid`

运行状态：

- `enabled`：可执行
- `disabled`：清单禁用
- `invalid`：加载或依赖校验失败
- `success`：执行成功
- `failed`：执行失败
- `timeout`：执行超时
- `degraded`：执行失败但命中 fallback

`Reload()` 重新扫描插件目录。`Unload(name)` 从当前管理器中移除插件。CLI 暴露 `reload` 和 `unload --name` 方便验证

### 2.4 执行与失败隔离

每个插件独立进程执行。主程序不共享插件内存，不链接插件代码

失败处理：

- 插件非零退出 -> `failed`
- 插件 stdout 为空 -> `failed`
- 插件 stdout 非 JSON -> `failed`
- 插件返回 `ok=false` -> `failed`
- 插件超时 -> `timeout`，进程被终止
- 配置 fallback -> 状态改为 `degraded`，返回 `fallback_data`，同时保留原始错误

单个插件失败不影响后续插件执行

## 3. 关键实现选择与取舍说明

### 3.1 为什么不用 Go 标准库 `plugin`

不用 `plugin` 包。原因：

- 平台限制明显，Windows 不支持
- 主程序和插件必须 Go 版本、依赖、构建参数兼容
- 插件 panic 更容易影响主进程
- 只能加载 Go 符号，不适合跨语言插件协议

当前选择独立进程模型。结果：

- 稳定性更好：插件崩溃不拖垮主程序
- 扩展性更好：任意语言可接入
- 部署更简单：插件独立构建、独立替换
- 代价：每次执行要启动进程，性能低于内存调用

### 3.2 为什么不用现成插件框架

题目要求主程序不依赖现成插件系统或流程编排框架。实现只用 Go 标准库完成：

- `os.ReadDir` / `os.ReadFile`：扫描和读取清单
- `encoding/json`：协议序列化
- `os/exec`：启动插件进程
- `context.WithTimeout`：控制超时
- `flag`：处理 CLI 参数

而且任务非常简单,用这些框架反而是大炮打蚊子了

### 3.3 依赖与执行顺序取舍

依赖只做加载合法性和执行顺序约束：

- 依赖插件必须存在
- 依赖插件必须启用且合法
- 版本约束支持 `=`, `==`, `>`, `>=`, `<`, `<=`
- 循环依赖直接标记非法
- 执行时依赖插件先运行

不做插件间数据流水线。所有插件收到同一份输入。原因：题目重点是插件化执行系统，不是工作流编排系统。这样设计更小、更稳、更容易验证

### 3.4 安全与资源隔离取舍

当前隔离边界是进程边界 + JSON 协议 + 超时终止

已做：

- 插件入口必须是相对路径
- 入口路径不能逃出插件目录
- 入口文件必须存在且可执行
- 单插件有超时
- 非法输出不会污染主程序

未做：

- OS 级文件系统沙箱
- CPU / 内存限额
- 网络权限控制
- 常驻插件进程池
- 文件监听式热加载

原因：这些属于运行环境治理，不是核心插件协议。后续可在 `runner` 进程启动层接入 sandbox、容器、cgroup 或权限配置，不需要改插件协议

## 4. 如何运行

### 4.1 构建示例插件

先生成 Go 示例插件可执行文件：

```bash
go build -o plugins/sample/sample-plugin ./cmd/sample-plugin
```

`plugins/python-echo` 是 Python 示例插件，默认禁用。需要验证跨语言插件时，把它的 `plugin.json` 里 `enabled` 改成 `true`，并确保本机有 `python3`。

### 4.2 查看和管理插件

```bash
go run ./cmd/executor list
go run ./cmd/executor reload
go run ./cmd/executor unload --name sample
```

- `list`：查看插件名称、版本、启用状态、加载状态
- `reload`：重新扫描插件目录
- `unload --name`：从当前管理器结果中移除插件

### 4.3 执行插件

直接传 JSON：

```bash
go run ./cmd/executor run --input '{"message":"hello"}'
```

从文件读取 JSON：

```bash
go run ./cmd/executor run --file input.json
```

指定插件目录：

```bash
go run ./cmd/executor run --plugins plugins --input '{"message":"hello"}'
```

### 4.4 验证

```bash
go test ./...
golangci-lint run
```
