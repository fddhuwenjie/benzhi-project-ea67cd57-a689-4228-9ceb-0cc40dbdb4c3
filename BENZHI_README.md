# BENZHI_README

## 项目说明
- 项目：benzhi-project-ea67cd57-a689-4228-9ceb-0cc40dbdb4c3
- 项目用途：面向历史地图数字化团队的地理配准质量审定工作台，完整实现底图基线冻结、控制点维护、确定性仿射求解、残差整改、独立复核、摘要链审计和不可变发布清单校验。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：map-registration-gate
- 项目介绍：面向历史地图数字化团队的地理配准质量审定工作台，将底图基线、控制点、仿射求解、残差整改、独立复核和发布清单冻结串成一条可追溯流程。项目根目录提供简体中文 README.md，说明用途以及标准构建、运行和测试方式。
- 项目概述：面向历史地图数字化团队的地理配准质量审定工作台，将底图基线、控制点、仿射求解、残差整改、独立复核和发布清单冻结串成一条可追溯流程。项目根目录提供简体中文 README.md，说明用途以及标准构建、运行和测试方式。
- 核心工作流：数字化员创建配准任务并冻结历史底图与目标坐标系基线，录入分布合格的控制点后执行确定性仿射求解；残差或空间分布不达标时任务进入待整改状态，数字化员替换问题点并重新求解，达标后提交独立复核；复核员检查确定性抽取的叠加样本并作出结论，拒绝则退回定向整改，批准则生成不可变发布清单并将任务冻结为已发布状态。
- 对外接口：Go 服务直接提供原生 HTML、CSS 和 JavaScript 单页工作台及仅供该页面使用的同源 JSON 端点；页面包含任务状态栏、底图与控制点表格、Canvas 叠加预览、残差整改队列、复核面板和发布清单校验视图，无需 Node 构建链。HTTP 监听支持 -addr=127.0.0.1:<port>，也支持 PORT 端口环境变量并绑定 127.0.0.1，默认地址为 127.0.0.1:19081，绝不默认绑定 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -selftest -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-ea67cd57-a689-4228-9ceb-0cc40dbdb4c3-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-ea67cd57-a689-4228-9ceb-0cc40dbdb4c3-arm64 linux/arm64

docker run -it benzhi-project-ea67cd57-a689-4228-9ceb-0cc40dbdb4c3-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selftest -addr=127.0.0.1:19081`
