# 🤝 贡献指南

感谢你对 Locus 项目的关注！我们欢迎所有形式的贡献。

## 📋 目录

- [行为准则](#行为准则)
- [如何贡献](#如何贡献)
- [开发流程](#开发流程)
- [代码规范](#代码规范)
- [提交规范](#提交规范)
- [测试要求](#测试要求)

---

## 行为准则

参与本项目即表示你同意遵守我们的行为准则。请友善、尊重他人。

---

## 如何贡献

### 🐛 报告 Bug

1. 查看 [现有 Issue](https://github.com/Versifine/locus/issues) 确认问题未被报告
2. 使用 Bug 报告模板创建新 Issue
3. 提供详细的复现步骤和环境信息

### ✨ 提出功能请求

1. 查看 [现有 Issue](https://github.com/Versifine/locus/issues) 避免重复
2. 使用功能请求模板描述你的想法
3. 说明功能的使用场景和价值

### 💻 提交代码

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交你的更改 (`git commit -m 'feat: add some amazing feature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

### 📝 改进文档

文档改进同样重要！包括但不限于：
- 修正错别字
- 改进说明
- 添加示例
- 翻译文档

---

## 开发流程

### 1. 环境准备

```bash
# 克隆仓库
git clone https://github.com/Versifine/locus.git
cd locus

# 安装依赖
go mod download

# 验证环境
go version  # 需要 Go 1.21+
```

### 2. 本地开发

```bash
# 运行程序
go run ./cmd/locus

# 运行测试
go test ./...

# 运行测试（带覆盖率）
go test -v -race -coverprofile=coverage.txt ./...

# 代码检查
go vet ./...
```

### 3. 使用 Linter

我们使用 `golangci-lint` 进行代码检查：

```bash
# 安装 golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 运行 linter
golangci-lint run
```

---

## 代码规范

### Go 代码风格

- 遵循 [Effective Go](https://go.dev/doc/effective_go) 规范
- 使用 `gofmt` 格式化代码
- 所有导出的函数、类型、常量必须有注释
- 保持函数简短，单一职责
- 使用有意义的变量和函数名

### 项目结构

```
locus/
├── cmd/           # 主程序入口
├── internal/      # 内部包（不对外暴露）
│   ├── proxy/     # 代理核心逻辑
│   ├── protocol/  # Minecraft 协议解析
│   ├── llm/       # LLM 集成
│   └── ...
├── configs/       # 配置文件
├── docs/          # 文档
└── tests/         # 测试文件
```

### 注释规范

```go
// Package proxy implements the Minecraft reverse proxy functionality.
//
// The proxy intercepts traffic between Minecraft clients and servers,
// allowing for protocol analysis and AI-driven bot control.
package proxy

// Handler processes incoming Minecraft packets.
//
// It parses the packet, updates world state, and optionally
// forwards it to the backend server.
type Handler interface {
    Handle(packet *Packet) error
}
```

---

## 提交规范

我们使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

### 提交消息格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type 类型

- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式（不影响功能）
- `refactor`: 重构（不是新功能也不是修复）
- `perf`: 性能优化
- `test`: 添加或修改测试
- `chore`: 构建/工具相关
- `ci`: CI/CD 相关

### 示例

```bash
# 新功能
git commit -m "feat(proxy): add packet filtering support"

# Bug 修复
git commit -m "fix(protocol): correct handshake packet parsing"

# 文档
git commit -m "docs: update README with new installation steps"

# 重构
git commit -m "refactor(llm): simplify provider interface"
```

---

## 测试要求

### 单元测试

- 所有新功能必须包含测试
- 测试覆盖率应保持在 70% 以上
- 使用表驱动测试

```go
func TestPacketParser(t *testing.T) {
    tests := []struct {
        name    string
        input   []byte
        want    *Packet
        wantErr bool
    }{
        {
            name:    "valid handshake",
            input:   []byte{0x00, 0x00, ...},
            want:    &Packet{ID: 0x00, ...},
            wantErr: false,
        },
        // 更多测试案例...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParsePacket(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParsePacket() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParsePacket() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### 集成测试

对于复杂功能，添加集成测试：

```bash
# 运行集成测试
go test -tags=integration ./tests/integration/...
```

---

## Pull Request 流程

### 1. 创建 PR 前

- [ ] 代码已通过所有测试
- [ ] 代码已通过 linter 检查
- [ ] 已添加必要的测试
- [ ] 已更新相关文档
- [ ] Commit 消息符合规范

### 2. PR 描述

使用 PR 模板，清晰描述：
- 变更内容
- 相关 Issue
- 测试方法
- 截图/演示（如适用）

### 3. Code Review

- 保持耐心，积极响应反馈
- 解释你的设计决策
- 接受建设性批评

### 4. 合并

- PR 需要至少一个维护者批准
- 所有 CI 检查必须通过
- 解决所有冲突

---

## 获取帮助

遇到问题？

- 💬 [GitHub Discussions](https://github.com/Versifine/locus/discussions)
- 🐛 [Issue Tracker](https://github.com/Versifine/locus/issues)
- 📖 [项目文档](https://github.com/Versifine/locus/blob/master/README.md)

---

## 感谢

感谢所有贡献者让 Locus 变得更好！ 🎉

你的贡献将被记录在 [Contributors](https://github.com/Versifine/locus/graphs/contributors) 页面。
