# Agent Home Context Design

**Date:** 2026-04-28  
**Status:** Draft  
**Scope:** Agent 独立目录、人格文件、分级上下文、外部 memory/MCP 接入边界

## 背景

cc-connect 的核心定位仍然是平台桥接、会话路由和 agent 编排，不应内置完整的长期记忆系统或公司知识库。但 agent 在长期使用中确实需要一个稳定的、可写的独立目录，用来保存人格文件、会话资料、工程经验和外部 memory/MCP 配置。

这个目录不应该是 agent 安装目录，也不应该直接污染 Codex/Claude 等 agent 的私有状态目录。建议由 cc-connect 在自己的状态目录下为每个 agent 管理一个独立空间。

## 目标

1. 每个 agent 拥有独立的 `Agent Home`。
2. 每条会话拥有可写的 `Session Workspace`。
3. 明确区分人格、工程经验、相处记忆和临时会话资料。
4. 支持外部 memory/MCP 读取这些文件，但 cc-connect 不负责实现向量库或自动知识库。
5. 支持从会话资料中人工提升内容到更高层级，避免自动污染长期上下文。

## 目录模型

```txt
Project Workspace
/root/cc-connect-fresh
  代码工作区，agent 真正修改代码、运行测试和构建的地方。

Agent Home
~/.cc-connect/agents/codex/
  Codex 在 cc-connect 下的独立上下文目录。

Session Workspace
~/.cc-connect/agents/codex/sessions/<project>/<session>/
  某个 agent + 项目 + 会话的可写上下文目录。
```

示例：

```txt
~/.cc-connect/agents/codex/
├── personas/
│   ├── default.md
│   ├── engineer.md
│   └── reviewer.md
├── memory/
│   ├── engineering.md
│   ├── relationship.md
│   └── preferences.md
├── projects/
│   └── cc-connect-fresh-bafdcc8a/
│       ├── engineering.md
│       ├── decisions.md
│       └── references.md
├── mcp/
│   └── default.json
├── sessions/
│   └── cc-connect-fresh-bafdcc8a/
│       └── webnew-web-admin/
│           ├── PERSONA.md
│           ├── SESSION.md
│           ├── NOTES.md
│           ├── DECISIONS.md
│           ├── mcp.json
│           └── scratch/
└── state.json
```

## 分级上下文

### 1. 人格记忆

人格记忆定义 agent 的稳定行为方式，不记录具体项目事实。

位置：

```txt
~/.cc-connect/agents/<agent>/personas/*.md
~/.cc-connect/agents/<agent>/sessions/<project>/<session>/PERSONA.md
```

内容示例：

```md
# Persona

- 默认使用中文沟通。
- 先读代码再判断，不凭空假设。
- 修改代码前说明要改哪些文件。
- 不主动改参考工作区。
- 遇到会影响运行服务的操作，先说明风险。
```

规则：

- 默认只由用户或管理员修改。
- cc-connect 不自动写入人格记忆。
- 会话级 `PERSONA.md` 可以覆盖 agent 默认 persona。
- 不把项目事实、历史决策、用户隐私写入 persona。

### 2. 工程经验

工程经验记录可复用的工程判断、项目约定、排障经验和架构决策。

位置：

```txt
~/.cc-connect/agents/<agent>/memory/engineering.md
~/.cc-connect/agents/<agent>/projects/<project>/engineering.md
~/.cc-connect/agents/<agent>/projects/<project>/decisions.md
```

内容示例：

```md
# Engineering Experience

## cc-connect

- core 包不能导入 agent/* 或 platform/*。
- 新平台应通过 core.RegisterPlatform 注册。
- Web 多入口统一使用 webnew 作为平台身份，web2/web3/web5 只是 route。
```

规则：

- 可由用户手动维护。
- 也可由 agent 在得到明确确认后写入。
- 会话中的临时结论先写入 `Session Workspace/DECISIONS.md`，确认后再提升到项目工程经验。
- 工程经验可以跨会话复用，但不一定跨 agent 共享。

### 3. 相处记忆

相处记忆记录用户与 agent 的协作偏好，不记录公司机密或项目事实。

位置：

```txt
~/.cc-connect/agents/<agent>/memory/relationship.md
~/.cc-connect/agents/<agent>/memory/preferences.md
```

内容示例：

```md
# Relationship Memory

- 用户偏好直接、务实的中文回答。
- 设计讨论时先厘清边界，再谈实现。
- 用户不希望 cc-connect 内置完整记忆系统，更倾向开放外部 memory/MCP 接入。
- 用户希望保留 upstream 对齐空间，避免占用 webadmin 命名。
```

规则：

- 默认不自动写入。
- 写入前应让用户确认。
- 需要可查看、可删除、可导出。
- 不应与项目工程经验混在一起。

### 4. 会话资料

会话资料是当前会话的临时上下文、草稿、任务清单和阶段性结论。

位置：

```txt
~/.cc-connect/agents/<agent>/sessions/<project>/<session>/
```

内容示例：

```txt
SESSION.md     当前会话摘要
NOTES.md       临时笔记
DECISIONS.md   本会话形成但尚未提升的决策
REFERENCES.md  本会话引用的外部路径、文档、仓库
scratch/       临时文件
```

规则：

- 会话资料可写。
- 它默认只影响当前会话。
- 需要明确动作才提升为工程经验或相处记忆。
- 可以被外部 MCP memory server 索引。

## 注入顺序

启动或恢复 agent 时，cc-connect 可按以下顺序构造上下文：

```txt
1. cc-connect 固定系统指令
2. agent 默认 persona
3. 项目 persona
4. 会话 persona
5. 工程经验摘要
6. 相处偏好摘要
7. 平台格式指令，例如 webnew/Telegram/Feishu
8. 当前用户消息
```

不是所有 agent 都支持原生 system prompt。对于支持的 agent，优先注入 system prompt；对于不支持的 agent，可以通过 agent 自己的 instruction/memory 文件或首轮 prompt 传递。

## 环境变量

cc-connect 应给 agent 注入稳定环境变量：

```txt
CC_PROJECT=<project>
CC_SESSION_KEY=<canonical session key>
CC_ROUTE_KEY=<transport/route key>
CC_PLATFORM=<platform>
CC_USER_ID=<user id>
CC_PROJECT_WORKSPACE=<project work dir>
CC_AGENT_HOME=~/.cc-connect/agents/<agent>
CC_SESSION_WORKSPACE=~/.cc-connect/agents/<agent>/sessions/<project>/<session>
CC_PERSONA_FILE=<selected persona file>
CC_MCP_CONFIG=<selected mcp config>
```

这些变量让外部 memory/MCP 能识别当前项目、会话、用户和 agent。

## 外部 memory/MCP 接入

cc-connect 不内置 memory，但可以提供文件和元数据，让外部系统接入。

### Agent 原生 MCP

用户可以在 Codex/Claude 的 MCP 配置里接入 memory server。memory server 通过 `CC_AGENT_HOME`、`CC_SESSION_WORKSPACE` 或配置文件找到当前上下文。

### 会话 MCP 配置

会话目录可以包含：

```txt
mcp.json
```

用于描述当前会话需要的 MCP server、memory root 或公司知识库 endpoint。

### Persona 指导

persona 可以告诉 agent：

```md
如果 memory 工具可用：

- 开始任务前检索当前 project/session 相关记忆。
- 做出重要设计决策后，先询问用户是否写入长期记忆。
- 不要自动把临时讨论写入相处记忆。
```

## 提升流程

为避免长期上下文污染，建议采用显式提升：

```txt
Session Notes
  ↓ 用户确认
Project Engineering Experience
  ↓ 管理员确认
Agent/Company Shared Experience
```

相处记忆也需要显式确认：

```txt
用户表达偏好
  ↓ agent 询问是否记录
relationship.md
```

## 权限模型

```txt
Project Workspace     writable，代码和测试
Agent Home            writable，agent 自己的上下文
Session Workspace     writable，当前会话资料
Reference Workspace   read-only，参考仓库或文档
Global Persona Library admin-writable，默认只读
```

重要原则：

- agent 安装目录不可写。
- 参考工作区默认只读。
- persona 默认不自动改。
- relationship memory 默认不自动写。
- engineering memory 可以经确认后写。

## 与 webnew 的关系

`webnew` 解决 Web 多入口命中同一会话的问题。

`Agent Home` 解决 agent 拥有独立上下文文件夹的问题。

二者结合后：

```txt
session_key = webnew:web-admin:<project>
route_key   = web2:web-admin:<project>
agent_home  = ~/.cc-connect/agents/codex
session_ws  = ~/.cc-connect/agents/codex/sessions/<project>/webnew-web-admin
project_ws  = /root/cc-connect-fresh
```

这样 Web 入口切换不会丢当前对话，agent 也有自己的可写上下文空间。

## 非目标

- 不实现内置向量数据库。
- 不实现公司知识库管理系统。
- 不自动总结所有会话。
- 不自动写入相处记忆。
- 不把 agent home 放到 agent 安装目录。
- 不把参考工作区当成项目工作区。

## 验收标准

1. 每个 agent 有独立 `Agent Home`。
2. 每条会话有独立 `Session Workspace`。
3. 首次创建会话时自动生成 `PERSONA.md`、`SESSION.md`、`NOTES.md`、`DECISIONS.md`。
4. agent 启动时能收到 `CC_AGENT_HOME` 和 `CC_SESSION_WORKSPACE`。
5. 人格记忆、工程经验、相处记忆、会话资料在目录和语义上清晰分离。
6. 不需要 cc-connect 内置 memory，也能让外部 MCP/memory server 接入。
