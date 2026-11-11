# 模型指纹

管理端侧栏“模型指纹”，路由 `/admin/model-fingerprint`。

## 用法与判断边界

1. 点击“下载 / 更新参考数据”。页面打开、刷新和历史查询不会自动下载。
2. 选择直接持有 API Key 的 OpenAI 兼容或 Anthropic 账号，填写请求模型 ID。
   模型输入支持账号配置的映射建议，后端固定该账号并按映射后的模型发送。
3. 选择自动协议，或者指定 Chat Completions、Responses、Anthropic Messages。
4. 开始验证。会发送三条 292–332 个数字的挑战，可能产生实际上游费用。
5. 查看有效样本数、候选排名、库内相对概率，展开每轮原始文本。
   历史可按账号 ID、请求模型精确筛选，支持分页、详情与 JSON 导出。

首版不支持 OAuth、继承凭据、Gemini 原生接口和 Antigravity 账号。
自动模式默认 Chat Completions；Anthropic 账号默认 Messages，配置了
Responses 的账号默认 Responses。使用其他兼容协议时请手动选择。

结果来自 `lmfpd-sdk 0.2.0` 的 shared-detector-v2 数字指纹算法，沿用参考项目
的多语言挑战模板、序列形状检查、频率/位置特征与闭集校准。它提供排查证据，
**不能单独证明模型身份**。模型不在参考库、上游改写提示、推理策略变化等都会
影响排名。HTTP 成功率（记录中的 `score`）不代表身份置信度。

上游返回模型名与实际请求模型不一致时，保存 `route_mismatch` 并停止评分。
合法别名、日期版本名也可能触发此状态，需要人工判断。样本不足、序列异常、
部分样本与采样失败分别展示，不会仅因为模型名一致就显示“验证通过”。

## 数据与实现边界

新增迁移 `252_model_fingerprint.sql`，只创建本功能的表/索引，不改账号结构：

- `model_fingerprint_records`：请求账号/名称快照、模型与映射目标、端点、状态、
  各轮提示/回答/错误/响应模型/响应 ID/数量/耗时、候选结果和参考库版本/哈希。
- `model_fingerprint_reference`：最近一次显式下载的参考库 manifest 和检测器二进制。

请求前创建记录，每轮完成后保存已收到的样本。取消/超时后另用短数据库上下文
保存最终状态；进程中断时留下“执行中 / 未完成”及已保存样本。账号被删除后
历史仍保留。没有自动清理、删除历史或账号状态变更。

参考源与参考项目相同：`lm.ikale.io/data/` 和
`cdn.jsdmirror.com/npm/lmfpd-data@latest/`。仅向参考源发送参考文件下载请求，
不发送账号、模型、密钥、提示或回答。显式下载失败不会覆盖已有缓存。
检测器 SHA-256 与 manifest 匹配后才使用；这用于校验文件一致性，不是独立发布者签名。
每次验证固定使用启动时加载的快照，中途更新不会改变该次排名。

上游请求使用已有 URL 策略、账号代理与 HTTPUpstream，禁止重定向；不进行账号
池调度或重试。单轮超时 110 秒、总采样超时 360 秒；每账号同一进程最多一个
任务，全进程最多三个任务。接口沿用管理员认证、审计和合规中间件，受限管理
密钥未获得新接口权限。返回内容会屏蔽被测账号的 API Key。

新增实现主要位于 `backend/internal/modelfingerprint/`、各层的
`model_fingerprint*.go` 和 `frontend/src/features/model-fingerprint/`；已有文件
仅增加 Wire、路由、菜单及中英文聚合接线。`wire_gen.go` 由 Wire 重新生成。
第三方许可见 `backend/internal/modelfingerprint/LICENSE` 和 `NOTICE.md`。

## 容器化验证

使用独立数据库，不读取部署数据库，不删除测试记录或卷：

```sh
docker compose -p sub2api-fingerprint-check -f deploy/docker-compose.fingerprint-test.yml up -d --wait database
docker compose -p sub2api-fingerprint-check -f deploy/docker-compose.fingerprint-test.yml run --rm backend
docker compose -p sub2api-fingerprint-check -f deploy/docker-compose.fingerprint-test.yml run --rm frontend
```

前端检查容器使用工作树已有 `frontend/node_modules`，需先准备对应依赖。
Go 检查容器复用本地的 `sub2api-go-mod-cache`、`sub2api-go-build-cache` 外部卷。
数据库记录保存在本检查项目的专用卷中，停止时不要使用 `down -v`。

包含参考 SDK 固定夹具排名/概率对照、序列检查、编码与校验、三协议请求、
账号固定与映射、失败/取消记录、数据库迁移幂等与读写筛选，以及页面/API 测试。
离线测试不消耗真实上游额度；真实模型效果需要在管理页选择账号后主动验证。

若缓存卷不存在，首次运行前执行 `docker volume create sub2api-go-mod-cache` 和
`docker volume create sub2api-go-build-cache` 即可。安装依赖不需要更改生产服务。
