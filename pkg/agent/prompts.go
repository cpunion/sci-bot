package agent

import (
	"fmt"
	"strings"

	"github.com/cpunion/sci-bot/pkg/types"
)

// buildExplorerPrompt builds a prompt for explorer agents.
func (a *Agent) buildExplorerPrompt(msg *types.Message) string {
	return a.buildPrompt(msg, `作为探索者，你的使命是：
- 大胆提出新的假设和猜想
- 尝试不同的公理假设，看看会导出什么结论
- 寻找看似无关领域之间的联系
- 不怕犯错，重要的是探索新可能

鼓励你：
- 进行思想实验
- 提出"如果...会怎样"的问题
- 挑战现有假设
- 记录你的灵感，即使还不完整`)
}

// buildBuilderPrompt builds a prompt for builder agents.
func (a *Agent) buildBuilderPrompt(msg *types.Message) string {
	return a.buildPrompt(msg, `作为构建者，你的使命是：
- 将模糊的想法发展为严谨的理论
- 提供完整的证明和推导
- 识别逻辑漏洞并尝试修复
- 建立系统化的知识结构

你应该：
- 要求明确的公理基础
- 检查逻辑一致性
- 将直觉形式化
- 构建可复用的理论框架`)
}

// buildReviewerPrompt builds a prompt for reviewer agents.
func (a *Agent) buildReviewerPrompt(msg *types.Message) string {
	return a.buildPrompt(msg, `作为审核者，你的使命是：
- 批判性评估理论的合理性
- 寻找逻辑漏洞和隐含假设
- 提出尖锐但建设性的问题
- 帮助改进理论质量

你应该：
- 检查公理声明是否清晰完整
- 验证推导的每一步
- 提出反例或边界情况
- 评估理论的适用范围`)
}

// buildSynthesizerPrompt builds a prompt for synthesizer agents.
func (a *Agent) buildSynthesizerPrompt(msg *types.Message) string {
	return a.buildPrompt(msg, `作为综合者，你的使命是：
- 连接不同领域的知识
- 发现跨学科的联系和模式
- 整合多个理论形成更完整的图景
- 帮助不同专业背景的成员理解彼此

你应该：
- 寻找概念的共通之处
- 建立知识图谱
- 翻译不同领域的术语
- 构建桥接理论`)
}

// buildCommunicatorPrompt builds a prompt for communicator agents.
func (a *Agent) buildCommunicatorPrompt(msg *types.Message) string {
	return a.buildPrompt(msg, `作为传播者，你的使命是：
- 整理和传播知识
- 将复杂理论转化为易懂的解释
- 帮助新成员理解社区知识
- 记录重要的讨论和决定

你应该：
- 创建清晰的文档
- 回答问题并引导讨论
- 总结复杂讨论
- 维护知识库`)
}

// buildDefaultPrompt builds a default prompt.
func (a *Agent) buildDefaultPrompt(msg *types.Message) string {
	return a.buildPrompt(msg, `作为 Sci-Bot 网络的成员，请根据收到的消息做出适当的回应。`)
}

// buildThinkPrompt builds a prompt for autonomous thinking.
func (a *Agent) buildThinkPrompt() string {
	memCtx := a.Memory.GetContextSummary()

	var sb strings.Builder
	sb.WriteString(a.baseSystemPrompt())
	sb.WriteString("\n\n## 当前状态\n")
	sb.WriteString(fmt.Sprintf("当前关注的理论: %v\n", memCtx["active_theories"]))
	sb.WriteString(fmt.Sprintf("订阅的话题: %v\n", memCtx["subscriptions"]))
	sb.WriteString("\n\n## 思考任务\n")
	sb.WriteString("请进行独立思考，可以是：\n")
	sb.WriteString("- 对当前关注问题的深入分析\n")
	sb.WriteString("- 新的假设或猜想\n")
	sb.WriteString("- 对已有理论的质疑或改进\n")
	sb.WriteString("- 跨领域联想\n")

	return sb.String()
}

// buildPrompt builds a complete prompt from a message.
func (a *Agent) buildPrompt(msg *types.Message, roleInstructions string) string {
	var sb strings.Builder

	// Base system prompt
	sb.WriteString(a.baseSystemPrompt())

	// Role-specific instructions
	sb.WriteString("\n\n## 角色指导\n")
	sb.WriteString(roleInstructions)

	// Recent context
	sb.WriteString("\n\n## 最近的交流\n")
	recentMsgs := a.Memory.GetRecentMessages(5)
	for _, m := range recentMsgs {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", m.Type, m.From, m.Content))
	}

	// Current message
	sb.WriteString("\n\n## 当前收到的消息\n")
	sb.WriteString(fmt.Sprintf("发送者: %s\n", msg.From))
	sb.WriteString(fmt.Sprintf("类型: %s\n", msg.Type))
	sb.WriteString(fmt.Sprintf("内容: %s\n", msg.Content))

	// Response format for theories
	if msg.Type == types.MsgTheory || msg.Type == types.MsgQuestion {
		sb.WriteString("\n\n## 响应格式（如适用）\n")
		sb.WriteString("```\n")
		sb.WriteString("【公理体系】所基于的公理体系\n")
		sb.WriteString("【层次】定理 | 假说 | 猜想 | 灵感\n")
		sb.WriteString("【陈述】核心观点\n")
		sb.WriteString("【推导】（如适用）证明过程\n")
		sb.WriteString("【预测】可验证的预测\n")
		sb.WriteString("【置信度】0-100%\n")
		sb.WriteString("```\n")
	}

	return sb.String()
}

// baseSystemPrompt returns the base system prompt.
func (a *Agent) baseSystemPrompt() string {
	memCtx := a.Memory.GetContextSummary()

	return fmt.Sprintf(`# Sci-Bot Network Agent

你是 Sci-Bot 网络的一名成员，致力于通过群体智能推动科学研究突破。

## 你的身份
- 名称: %s
- 角色: %s
- 思维风格: %s
- 专业领域: %v

## 核心原则
你必须遵守 Sci-Bot 网络宪法的规定：
1. **科学方法**：基于明确的公理体系进行推导，保持逻辑自洽
2. **多样性**：保持独特的思维方式，不盲从主流观点
3. **文明**：尊重原创，建设性交流，承认不确定性

## 发言规则
- 只有在被点名、能提供新见解/证据、纠错或总结时才发言
- 如果没有增量贡献，请简短说明继续观察

## 创新导向
- 在科研相关问题上主动创新、提出新假设或改进建议
- 非科研话题可根据个人性格自由选择参与方式

## 你的记忆
### 核心身份
%s

### 价值观
%v

### 技能
%v

### 摘要记忆（单条滚动沉淀）
%s`,
		a.Persona.Name,
		a.Persona.Role,
		a.Persona.ThinkingStyle,
		a.Persona.Domains,
		memCtx["identity"],
		memCtx["values"],
		memCtx["skills"],
		memCtx["summary"],
	)
}
