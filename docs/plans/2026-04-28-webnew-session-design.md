# Webnew Session Design

**Date:** 2026-04-28  
**Status:** Draft  
**Scope:** Web 多入口会话归一、上下文恢复、管理端兼容

## 背景

当前 cc-connect 的 Web 管理端可以从不同入口访问，例如原始 `web`、复制出的 `web2`、后续可能出现的 `web3`、`web5`。这些入口的本意是多通道转发或多前端入口，而不是创建不同的对话上下文。

实际问题是：当前 Web 前端把入口名写进了 `session_key`。

```txt
bridge:web-admin:<project>
web2:web-admin:<project>
web3:web-admin:<project>
web5:web-admin:<project>
```

后端按 `session_key -> Session -> AgentSessionID` 恢复 agent 会话。入口一变，`session_key` 就变，Engine 会命中另一个 Session，进而拿不到原来的 Codex/Claude 会话 ID。用户感知就是“切回去之后之前的对话没了”。

这不是 agent 本身丢上下文，而是 cc-connect 把“入口/通道”误当成了“会话身份”。

## 参考原则

借鉴 OpenClaw 的会话模型：

- 会话身份稳定，代表上下文。
- 消息通道可变，只代表从哪里进入、回哪里去。
- 直聊默认可以折叠到主会话。
- 群、频道、平台用户仍可以隔离。
- `channel`、`origin`、`deliveryContext`、`lastChannel` 等作为投递元数据保存，不作为主会话身份。

落到 cc-connect，应避免继续让 `web2`、`web3`、`web5` 决定上下文。它们应只是 Web 入口或卡位。

## 目标

1. 保留现有管理入口和页面结构，不重命名、不占用 `webadmin` 语义，便于继续跟踪原始 cc-connect 仓库。
2. 新增一个 Web 消息平台身份：`webnew`。
3. 所有 Web 管理入口共享同一个项目上下文。
4. QQ、Telegram、Feishu、Slack 等真实消息平台继续保持平台隔离。
5. 旧会话不删除，提供 alias/fallback 兼容，避免已有历史丢失。
6. 管理员视图可以看到同一项目下所有平台会话，而不是强制合并所有平台。

## 命名

```txt
web / webadmin  原始管理端语义，保留给 upstream 对齐
web2/web3/web5  Web 前端入口、卡位、实验 UI
webnew          新的统一 Web 消息平台
bridge          旧 Web bridge key，作为 legacy 兼容
```

`webnew` 是平台身份，不是新的管理端页面名。

```txt
入口: web2
平台: webnew
会话: webnew:web-admin:<project>
```

## 目标模型

### 当前模型

```txt
session_key = <入口平台>:web-admin:<project>
```

示例：

```txt
web2:web-admin:cc-connect-fresh-bafdcc8a
web3:web-admin:cc-connect-fresh-bafdcc8a
bridge:web-admin:cc-connect-fresh-bafdcc8a
```

这些 key 会被后端当作不同用户/会话桶。

### 新模型

```txt
session_key = webnew:web-admin:<project>
route_key   = <web入口>:web-admin:<project>
```

示例：

```txt
session_key = webnew:web-admin:cc-connect-fresh-bafdcc8a
route_key   = web2:web-admin:cc-connect-fresh-bafdcc8a
route_key   = web3:web-admin:cc-connect-fresh-bafdcc8a
route_key   = bridge:web-admin:cc-connect-fresh-bafdcc8a
```

`session_key` 用于：

- SessionManager 查找/创建本地 Session
- Engine 复用 interactive state
- AgentSessionID 保存和恢复
- Codex/Claude resume
- `/sessions`、`/new`、`/switch` 等会话命令

`route_key` 用于：

- 标记来源入口
- WebSocket 回复路由
- 最后活跃入口识别
- 管理端调试展示
- legacy 兼容定位

## Bridge 消息形态

短期可以不修改协议字段，只让 Web 前端发送稳定的 `session_key`：

```json
{
  "type": "message",
  "platform": "webnew",
  "session_key": "webnew:web-admin:cc-connect-fresh-bafdcc8a",
  "user_id": "web-admin",
  "user_name": "Web Admin",
  "content": "继续处理丢会话问题",
  "reply_ctx": "opaque-web-reply-context"
}
```

为了保留入口信息，建议在后续协议中增加可选字段：

```json
{
  "type": "message",
  "platform": "webnew",
  "session_key": "webnew:web-admin:cc-connect-fresh-bafdcc8a",
  "transport_session_key": "web2:web-admin:cc-connect-fresh-bafdcc8a",
  "route": "web2",
  "user_id": "web-admin",
  "user_name": "Web Admin",
  "content": "继续处理丢会话问题",
  "reply_ctx": "opaque-web-reply-context"
}
```

兼容要求：

- 老 adapter 没有 `transport_session_key` 时，照旧工作。
- 新 Web 前端用 `webnew` 作为平台身份。
- `reply_ctx` 仍保持不透明，由 adapter/前端解释。

## 会话恢复逻辑

Engine 当前恢复链路是合理的：

```txt
session_key -> active Session -> AgentSessionID -> agent.StartSession(ctx, AgentSessionID)
```

本方案不重写 Engine。关键是保证 Web 多入口进入 Engine 前，使用同一个稳定 `session_key`：

```txt
webnew:web-admin:<project>
```

这样不同 Web 入口最终会命中同一个 Session，也就能恢复同一个 Codex/Claude 会话。

## Legacy 兼容

已有会话不能丢。需要把以下旧 key 识别为同一类 Web 管理入口：

```txt
bridge:web-admin:<project>
web:web-admin:<project>
web2:web-admin:<project>
web3:web-admin:<project>
web4:web-admin:<project>
web5:web-admin:<project>
```

兼容策略：

1. 如果存在 `webnew:web-admin:<project>`，优先使用它。
2. 如果不存在，按以下顺序查找 legacy 会话：
   - 当前项目下 history 最多的 Web/bridge 会话
   - 若 history 相同，选择 `updated_at` 最新的会话
   - 若仍无法判断，优先 `bridge:web-admin:<project>`
3. 第一次命中 legacy 会话时，可以把它作为 `webnew` 的 active session。
4. 不删除 legacy key，不强制合并历史。
5. 管理端展示 legacy 来源，允许后续手动合并、重命名或删除。

以当前 `cc-connect-fresh` 为例，主历史在：

```txt
bridge:web-admin:cc-connect-fresh-bafdcc8a -> s2
AgentSessionID = 019dc4bf-eee7-73e3-a4ed-4b7a20e7aace
```

迁移时应优先把 `webnew:web-admin:cc-connect-fresh-bafdcc8a` 指向这条主会话，而不是新建空会话。

## 管理员视图

`webnew` 只解决 Web 管理入口共享上下文，不等于所有平台合并。

管理员视图应按项目展示所有会话：

```txt
project: cc-connect-fresh-bafdcc8a

- webnew:web-admin:<project>
- telegram:direct:<user>
- telegram:group:<chat>
- qq:direct:<user>
- feishu:<chat/thread>
```

管理员可以看到全部平台会话，但默认不把不同平台合并到同一个上下文。平台隔离仍然有效。

建议管理端增加分组字段：

```txt
project
platform
session_key
route_key
agent_type
agent_session_id
history_count
updated_at
legacy
```

## 配置建议

后续可以增加 session scope 配置，但第一阶段不强依赖。

```toml
[session]
default_scope = "platform_isolated"

[session.scopes]
webnew = "project_shared"
telegram = "platform_isolated"
qq = "platform_isolated"
feishu = "channel_isolated"

[session.admin]
view = "project_all"
can_switch = true
can_rename = true
can_delete = true
```

含义：

```txt
project_shared     同项目共享上下文
platform_isolated  同项目内按平台/用户隔离
channel_isolated   群、频道、线程独立
project_all        管理员能查看项目下所有会话
```

## 分阶段实施

### Phase 1: 文档和前端止血

目标：不动核心模型，先让 Web 多入口能找回主会话。

- 新增本设计文档。
- Web 前端生成 `webnew:web-admin:<project>` 作为默认 session key。
- 会话选择逻辑增加 legacy fallback。
- 保留 `web2/web3/web5` 作为 route/入口偏好，不再作为默认上下文身份。

影响范围：

```txt
web/src/lib/webPlatform.ts
web/src/pages/Chat/ChatView.tsx
web2/src/lib/webPlatform.ts
web2/src/pages/Chat/ChatView.tsx
```

### Phase 2: Bridge 协议补 route metadata

目标：把入口信息从 `session_key` 中移出，明确放到 route metadata。

- `bridgeMessage` 增加可选 `transport_session_key` / `route`。
- `bridgeReplyCtx` 保存 route 信息。
- 回复仍按当前 `reply_ctx` 返回，不破坏旧 adapter。

影响范围：

```txt
core/bridge.go
web*/src/hooks/useBridgeSocket.ts
docs/bridge-protocol*.md
```

### Phase 3: Session alias/fallback 后端化

目标：后端统一识别 legacy key，避免每个前端重复 fallback 逻辑。

- 增加 Web legacy key resolver。
- `web2/web3/web5/bridge` legacy key 自动解析到 `webnew` 主会话。
- SessionManager 保留 alias 映射或迁移辅助逻辑。
- 增加回归测试，覆盖旧 key 找回主 AgentSessionID。

影响范围：

```txt
core/session.go
core/management.go
core/bridge.go
core/*_test.go
```

### Phase 4: 管理员 project-all 视图

目标：管理端能按项目查看所有平台会话，同时保持平台隔离。

- 会话列表按 project/platform 分组。
- 标记 legacy 会话。
- 提供手动切换、重命名、删除。
- 后续可增加手动合并，但不是第一阶段目标。

## 非目标

- 不把 QQ、Telegram、Feishu 等真实平台合并到 `webnew`。
- 不删除已有 legacy 会话。
- 不重写 Engine 或 Agent adapter。
- 不把 `webnew` 做成新的管理页面名称。
- 不占用 `webadmin` 命名，保留给原始仓库或未来 upstream 语义。

## 风险

1. **回复路由混乱**  
   多个 Web 入口同时打开时，需要明确回复到最后活跃入口，还是广播给所有同项目 Web 入口。第一阶段建议维持当前连接的 `reply_ctx`，不做广播。

2. **legacy 会话选择错误**  
   如果同项目下多个 legacy 会话都有历史，自动选择可能不符合用户预期。第一阶段用“history 最多，其次 updated_at 最新”的规则，并在管理端保留手动切换。

3. **前端和后端认知不一致**  
   如果只改前端，后端仍不知道 route。短期可接受；中期应把 route metadata 写入 Bridge 协议和管理 API。

4. **upstream 合并冲突**  
   避免改名 `webadmin`，使用 `webnew` 降低和原始仓库概念冲突的概率。

## 验收标准

1. 从 9821、9822、web2、web3 进入同一项目，默认看到同一条 Web 管理会话。
2. 切换 Web 入口后，仍恢复原来的 `AgentSessionID`。
3. 当前主 legacy 会话可以被 `webnew` 找回，不新开空会话。
4. Telegram/QQ/Feishu 等平台会话不受影响，仍按原 `session_key` 隔离。
5. 管理端可以区分：
   - 统一 Web 会话：`webnew:web-admin:<project>`
   - legacy Web 会话：`bridge/web2/web3/...`
   - 其他真实平台会话

## 建议结论

采用 `webnew` 作为新 Web 消息平台身份。管理入口继续保留，前端入口继续存在，但不再决定会话上下文。

第一步先做前端止血和 legacy fallback；第二步把 route metadata 后端化；第三步再完善 alias 迁移和管理员 project-all 视图。
