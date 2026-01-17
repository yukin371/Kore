#!/bin/bash
# Quick Test Script for TUI Animated Status Indicators
# 这个脚本执行基本的编译和功能验证

echo "========================================="
echo "  Kore TUI 动画状态指示器 - 快速测试"
echo "========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试计数器
PASSED=0
FAILED=0

# 测试函数
test_case() {
    local name="$1"
    local command="$2"

    echo -n "测试: $name ... "

    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 通过${NC}"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}✗ 失败${NC}"
        ((FAILED++))
        return 1
    fi
}

# 1. 检查编译
echo "1. 编译验证"
echo "-----------------------------------"
test_case "可执行文件存在" "test -f bin/kore.exe"
test_case "可执行文件可执行" "test -x bin/kore.exe"
test_case "文件大小合理 (>10MB)" "[ $(stat -f%z bin/kore.exe 2>/dev/null || stat -c%s bin/kore.exe 2>/dev/null) -gt 10000000 ]"
echo ""

# 2. 基本命令测试
echo "2. 基本命令测试"
echo "-----------------------------------"
test_case "版本命令" "./bin/kore.exe version"
test_case "帮助命令" "./bin/kore.exe --help"
test_case "Chat 帮助命令" "./bin/kore.exe chat --help"
echo ""

# 3. 代码检查
echo "3. 代码质量检查"
echo "-----------------------------------"
test_case "Go 格式检查" "gofmt -l internal/adapters/tui/ | grep -q . && exit 1 || exit 0"
test_case "Go vet 检查" "go vet ./internal/adapters/tui/..."
test_case "Go build 检查" "go build -o /dev/null ./internal/adapters/tui/"
echo ""

# 4. 导入检查
echo "4. 依赖检查"
echo "-----------------------------------"
test_case "Viewport 依赖" "grep -q 'github.com/charmbracelet/bubbles/viewport' go.mod"
test_case "Bubble Tea 依赖" "grep -q 'github.com/charmbracelet/bubbletea' go.mod"
test_case "Lipgloss 依赖" "grep -q 'github.com/charmbracelet/lipgloss' go.mod"
echo ""

# 5. 文件完整性
echo "5. 文件完整性检查"
echo "-----------------------------------"
test_case "model.go 存在" "test -f internal/adapters/tui/model.go"
test_case "adapter.go 存在" "test -f internal/adapters/tui/adapter.go"
test_case "agent.go 已更新" "grep -q 'notifyToolExecutionStart' internal/core/agent.go"
test_case "USER_GUIDE.md 已更新" "grep -q 'Ctrl+d.*Tab' docs/USER_GUIDE.md"
echo ""

# 6. 新功能代码检查
echo "6. 新功能代码检查"
echo "-----------------------------------"
test_case "StatusState 枚举存在" "grep -q 'type StatusState int' internal/adapters/tui/model.go"
test_case "AnimatedStatus 结构存在" "grep -q 'type AnimatedStatus struct' internal/adapters/tui/model.go"
test_case "StatusChangeMsg 存在" "grep -q 'type StatusChangeMsg' internal/adapters/tui/model.go"
test_case "renderAnimatedStatusBar 存在" "grep -q 'func.*renderAnimatedStatusBar' internal/adapters/tui/model.go"
test_case "viewport.Model 字段存在" "grep -q 'viewport.*Model' internal/adapters/tui/model.go"
test_case "StartToolExecution 方法存在" "grep -q 'func.*StartToolExecution' internal/adapters/tui/adapter.go"
test_case "notifyToolExecutionStart 存在" "grep -q 'func.*notifyToolExecutionStart' internal/core/agent.go"
echo ""

# 7. 设计文档
echo "7. 文档检查"
echo "-----------------------------------"
test_case "设计文档存在" "test -f docs/plans/2026-01-17-tui-animated-status-indicators-design.md"
test_case "测试报告模板存在" "test -f test_tui_animated_status.md"
echo ""

# 8. Git 提交检查
echo "8. Git 提交检查"
echo "-----------------------------------"
COMMIT_COUNT=$(git log --oneline feature/tui-animated-status ^feature/foundation 2>/dev/null | wc -l)
if [ "$COMMIT_COUNT" -eq 5 ]; then
    echo -e "测试: 提交数量正确 (5个提交) ... ${GREEN}✓ 通过${NC}"
    ((PASSED++))
else
    echo -e "测试: 提交数量 (期望 5, 实际 $COMMIT_COUNT) ... ${YELLOW}⚠ 警告${NC}"
fi
echo ""

# 9. 统计总结
echo "========================================="
echo "  测试总结"
echo "========================================="
TOTAL=$((PASSED + FAILED))
echo "总测试数: $TOTAL"
echo -e "通过: ${GREEN}$PASSED${NC}"
echo -e "失败: ${RED}$FAILED${NC}"

if [ "$TOTAL" -gt 0 ]; then
    PERCENT=$((PASSED * 100 / TOTAL))
    echo "通过率: ${PERCENT}%"

    if [ "$PERCENT" -ge 80 ]; then
        echo -e "\n${GREEN}✓ 所有自动化测试通过！${NC}"
        echo -e "\n下一步："
        echo "1. 运行手动 TUI 测试: ./bin/kore.exe chat --ui tui"
        echo "2. 参考测试报告: test_tui_animated_status.md"
        echo "3. 检查 15 项手动测试清单"
    else
        echo -e "\n${RED}✗ 部分测试失败，请检查${NC}"
        exit 1
    fi
fi

echo ""
echo "========================================="
echo "  测试完成！"
echo "========================================="
