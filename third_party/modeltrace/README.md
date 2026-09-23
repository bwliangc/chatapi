# ModelTrace 集成与上游更新

这是 ChatAPI 的原生管理员功能，不经过插件管理。运行时只需要现有 Go 后端和 Vue 前端。

算法及参考库来自 [xqy2006/ModelTrace](https://github.com/xqy2006/ModelTrace)，保留 MIT 许可。`upstream.json` 记录来源提交、路径和 SHA-256。`fingerprint-core.mjs` 保留上游 JavaScript 原文（仅改扩展名），Go 移植放在 `backend/internal/pkg/modeltrace`，业务适配放在 `backend/internal/service/model_detection_*`。

## 设置页在线更新

系统设置 → 功能开关 → 模型一致性检测，下方提供「检查更新」「更新指纹库」「回滚上一版」。无需打开检测开关即可管理版本，也不会调用模型账号。

检查更新锁定 ModelTrace `main` 的提交，更新时重新下载该提交的算法与指纹库。只有算法 SHA-256 与当前 Go 程序适配版本完全一致、数据结构和数值校验通过，才允许切换。上游算法发生任何变化都会提示先升级系统，不会在线运行下载的 JavaScript。

当前版本和上一版本保存在数据库设置项 `model_detection_bank_state_v1` 中，重启后保留，多实例共享；原子比较更新防止并发覆盖。下一次检测立即使用新库，正在执行的任务继续使用开始时的版本。回滚只恢复上一版并消耗该回滚记录。下载或校验失败保留原库；数据库请求结果不确定时页面会刷新状态供确认。服务器需要访问 `api.github.com` 和 `raw.githubusercontent.com`，网络故障、GitHub 限流会显示错误。

## 构建镜像时自动同步

Release 工作流（包括完整发布、简化发布和 dry run）增加 `prepare-modeltrace`：

1. 从固定 ModelTrace 仓库拉取 `main`，锁定一次提交。手动发布可填写 `modeltrace_ref`，标签发布也可通过仓库变量 `MODELTRACE_REF` 固定分支、标签或 SHA。
2. 先核对算法 SHA-256，再运行库结构校验、Go 测试和 JS/Go 评分对照。算法改变或任一步失败时停止构建，不会发布含未适配算法的镜像。
3. 把验证后的库、来源记录及对照数据作为 `modeltrace-snapshot` 构建产物传给所有架构，确保各平台包含同一版本。每个二进制内嵌提交和文件哈希，设置页可查看版本；Actions 保留快照产物 7 天。

本地 `bash deploy/build_image.sh` 同样自动同步，每次传入刷新参数避免复用旧下载缓存。可用 `MODELTRACE_REF=<完整提交 SHA> bash deploy/build_image.sh` 固定版本。普通构建和 `deploy/Dockerfile` 均内置同步步骤；直接执行 Docker 时使用：

```sh
docker build --build-arg MODELTRACE_REF=main --build-arg MODELTRACE_REFRESH="$(date -u +%s)" -t sub2api:latest .
```

移动分支必须改变 `MODELTRACE_REFRESH`（或关闭构建缓存）才会重新拉取；固定完整 SHA 可复现同一指纹库。构建仅在 builder 中安装 Git、Python、Node，最终运行镜像仍不需要它们。自动构建只更新兼容的指纹数据；算法变化须先在源码中适配 Go 实现。需要暂缓上游变化时，将构建参数固定到已验证的旧 SHA，不会自动吞掉失败继续发布。

镜像携带的是**内置基线库**。已有数据库在线版本在算法仍兼容时继续优先使用，避免镜像升级覆盖管理员选择；要切到最新远程库，在设置页检查并更新。若新程序算法与保存的库不兼容，会回退到新镜像内置库并显示提示。构建不改运行数据库，也不修改应用仓库的远程配置。

## 两个上游独立更新

ChatAPI 的 `origin`、`upstream` 和 `custom/local-mods` 分支保持现有用法。继续使用：

```sh
git fetch upstream
git merge upstream/main  # 也可选择已确认的上游 tag/提交
```

ModelTrace 使用独立的 `modeltrace` remote，不把它的应用服务合入 ChatAPI 根目录。新克隆此仓库时先配置（Git remote 配置不随克隆继承）：

```sh
git remote add modeltrace https://github.com/xqy2006/ModelTrace.git
```

查看 ModelTrace 更新（只拉取 Git 对象和比较版本，不改业务代码）：

```sh
python3 scripts/modeltrace/update.py
```

审阅上游变更后，指定完整提交应用：

```sh
python3 scripts/modeltrace/update.py --ref <commit-sha> --apply
```

更新脚本需要 Python 3.9+、Node 和 Go，仅开发时需要，生产环境不需要它们。脚本会：

1. 拉取指定 ModelTrace 分支/标签/提交，锁定完整 SHA。
2. 检查受管理快照文件没有未提交改动。
3. 在临时 Go 包内放入新指纹库，用上游 JS 对相同合成样例重新评分。
4. 运行 Go 测试，验证候选得分/概率/排序与 JS 相符（误差 1e-9），并检查库结构。
5. 仅在验证通过后更新算法参考、指纹库、许可、对照样例及来源记录。最后由维护者审阅 diff、运行完整检查并提交。

如果上游算法改动不再兼容 Go 移植，脚本退出，原来的运行时快照不变；先根据上游修改 Go 实现再重跑。检查无法代替真实模型准确率验证，不能保证未知模型识别或门槛误报率。上游提示词变化也需要人工同步评估。

已有 Git 对象时可离线检查：

```sh
python3 scripts/modeltrace/update.py --no-fetch --ref <commit-sha>
```

设置页仅在线更新兼容的指纹数据。算法改造仍走源码审阅、测试、构建和部署流程；开发用 `--apply` 会运行指定版本的 JS 评分器，应先审阅该版本。构建使用 `--build --apply`，在执行 JS 前严格核对算法哈希。`source.json` 由脚本与来源快照一起维护。

## 使用与范围

1. 系统设置 → 功能开关 → 模型一致性检测，开启并保存。
2. 管理员菜单 → 模型检测，选择平台、账号和模型。
3. 开始检测并观察进度；可以取消，离开页面也会中止连接和上游请求。

目前支持 OpenAI / Anthropic 的 API Key 和 OAuth 文本账号，不支持影子、Bedrock、Vertex 或合成 UI 账号。所有采样固定同一账号，复用代理、认证和模型映射，不经过账号池调度；存在 OpenAI 传输插件时仍遵循该账号现有传输路径。每轮重新检查开关、账号可调度状态和模型映射，并在正常并发额度中占一个账号槽。关闭功能后阻止新任务和后续采样，当前上游调用仍受 90 秒时限约束。

目标三份有效回答，最多六次（仅数字不足重采样），单次 90 秒、总计 10 分钟。网络错误/上游错误/截断立即失败，不自动切换账号或偷偷重试。API Key 请求使用最多 4096 输出 token；Codex OAuth 不接受此参数，使用时间、响应字节上限控制，因此不能承诺固定 token 消耗上限。OAuth 必需的客户端身份前缀和模型规范化会保留，可能影响指纹分布。

结果在当前页面显示，不保存原始回答或长期检测历史；进程重启不恢复检测。阈值固定为候选概率 0.8、前两名差值 0.2，是启发式门槛。比对对象是管理员请求的模型名，映射后的模型及上游报告名称分别展示；别名或未收录名称不进行一致性判定。上游模型标签本身不是身份凭据。

管理员接口：`GET /api/v1/admin/model-detection`（库元数据），`POST /api/v1/admin/accounts/:id/model-detection`（JSON `{ "model": "..." }`，SSE 进度和结果）。功能默认关闭，关闭时上述两端点均返回 404。指纹库管理接口 `/api/v1/admin/model-detection/bank`（GET）及其 `/check`、`/update`、`/rollback`（POST）独立于检测开关，始终要求管理员权限；更新和回滚沿用系统二次认证策略。API Key、Token 和原始回答不进入浏览器事件或结果。

## 验证

```sh
cd backend
go test ./internal/pkg/modeltrace
go test ./internal/service ./internal/handler/admin -run TestModelDetection -count=1
```

没有为开发验证调用真实付费账号。真实账号准确率、代理注入、模型版本和采样参数变化仍需部署后小范围验收。

原生检测初次集成验证：后端 `go test ./...`、设置 API 契约、检测专项 race 和 vet 通过；相关前端 82 项测试、TypeScript 检查和生产构建通过。更新脚本完成真实 Git 拉取检查，并在隔离仓库中验证相同版本的应用流程。尚未用真实模型账号检验识别准确率。
