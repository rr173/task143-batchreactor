# BENZHI_README — task143-batchreactor

## 业务说明

本项目是一个**间歇反应釜反应动力学与热失控安全引擎**，面向工艺安全工程师与化工生产计划员。它基于反应动力学（Arrhenius 速率、RK4 转化率积分）与热平衡（产热/移热、绝热温升、Stoessel 临界等级、Frank-Kamenetskii TMR）评估液相间歇反应釜投料前的热安全，组织多产品反应釜活动（序列相关清洗 + 热兼容），跟踪批次生命周期（含热失控自动故障与转化率门槛放料约束），并从事件流恢复调度。

主要输入：反应配方（k0、Ea、级数、CA0、ΔH_rx、ρ、Cp、T0、夹套温度、热分解门槛、最小转化率、时长）与反应釜（体积、U、面积、最高许用温度）。
主要输出：转化率、峰值温度、Stoessel 等级、TMR、安全判定（safe/marginal/runaway）、批次生命周期状态与排程计划。

## 标准本地命令

```bash
go build ./...           # 编译
go run . --addr=:8080 --db=batchreactor.db   # 启动 HTTP 服务
go test ./...            # 测试
go run . --smoke-test    # 自检（页面 + 业务 API，运行后退出）
go run . --migrate-only  # 仅建表后退出
```

服务启动后页面 URL：`http://localhost:8080/`（原生 HTML/CSS/JS，经 `//go:embed` 打入二进制；覆盖 建反应釜/配方→跑动力学模拟→建活动→排程→推进批次→放料完成→看热安全结果 的真实读写流程）。

## 前端

- 前端目录：`internal/webfs/web/`（原生 HTML/CSS/JS，无构建工具、无 Node、无 npm）
- 构建方式：无 Node 步骤；`go build ./...` 经 `//go:embed web` 把页面打进二进制
- 构建产物路径：二进制内嵌（无独立产物目录）
- 镜像内验证：`test -s /app/internal/webfs/web/index.html`

## benzhi Docker 构建

构建脚本两参数：镜像名、平台。

```bash
# amd64
bash ./build_benzhi_docker.sh go-task-benzhi:amd64 linux/amd64
docker run --rm go-task-benzhi:amd64 go version

# arm64
bash ./build_benzhi_docker.sh go-task-benzhi:arm64 linux/arm64
docker run --rm go-task-benzhi:arm64 go version
```

进入容器交互：`docker run -it go-task-benzhi:amd64`

## 同时验证页面与业务 API 的 smoke-test

每个架构 benzhi 构建后，在镜像内检查前端产物非空并运行 Go `--smoke-test`：

```bash
# 镜像内：确认前端嵌入文件存在且非空
docker run --rm go-task-benzhi:amd64 test -s /app/internal/webfs/web/index.html
# 镜像内：运行 smoke-test（覆盖模拟、热兼容拒绝、序列清洗、生命周期、热失控故障、转化率门槛、前端页面、重启恢复）
docker run --rm go-task-benzhi:amd64 go run . --smoke-test
```
