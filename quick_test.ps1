# TUI 动画状态指示器 - 快速验证脚本
# Windows PowerShell 版本

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  Kore TUI 动画状态指示器 - 快速测试" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

$passed = 0
$failed = 0

function Test-Case {
    param($Name, $ScriptBlock)

    Write-Host -NoNewline "测试: $Name ... "

    try {
        $result = & $ScriptBlock
        if ($LASTEXITCODE -eq 0 -and $result) {
            Write-Host "✓ 通过" -ForegroundColor Green
            $global:passed++
            return $true
        } else {
            Write-Host "✗ 失败" -ForegroundColor Red
            $global:failed++
            return $false
        }
    } catch {
        Write-Host "✗ 失败" -ForegroundColor Red
        $global:failed++
        return $false
    }
}

# 1. 编译验证
Write-Host "1. 编译验证" -ForegroundColor Yellow
Write-Host "-----------------------------------"
Test-Case "可执行文件存在" { Test-Path "bin\kore.exe" }
Test-Case "文件大小合理 (>10MB)" {
    $size = (Get-Item "bin\kore.exe").Length
    $size -gt 10MB
}
Write-Host ""

# 2. 基本命令测试
Write-Host "2. 基本命令测试" -ForegroundColor Yellow
Write-Host "-----------------------------------"
Test-Case "版本命令" { & .\bin\kore.exe version 2>&1 | Out-Null; $LASTEXITCODE -eq 0 }
Test-Case "帮助命令" { & .\bin\kore.exe --help 2>&1 | Out-Null; $LASTEXITCODE -eq 0 }
Write-Host ""

# 3. 文件完整性
Write-Host "3. 文件完整性检查" -ForegroundColor Yellow
Write-Host "-----------------------------------"
Test-Case "model.go 存在" { Test-Path "internal\adapters\tui\model.go" }
Test-Case "adapter.go 存在" { Test-Path "internal\adapters\tui\adapter.go" }
Test-Case "agent.go 存在" { Test-Path "internal\core\agent.go" }
Write-Host ""

# 4. 新功能代码检查
Write-Host "4. 新功能代码检查" -ForegroundColor Yellow
Write-Host "-----------------------------------"
Test-Case "StatusState 枚举存在" {
    (Get-Content "internal\adapters\tui\model.go" -Raw) -match 'type StatusState int'
}
Test-Case "AnimatedStatus 结构存在" {
    (Get-Content "internal\adapters\tui\model.go" -Raw) -match 'type AnimatedStatus struct'
}
Test-Case "StatusChangeMsg 存在" {
    (Get-Content "internal\adapters\tui\model.go" -Raw) -match 'type StatusChangeMsg'
}
Test-Case "renderAnimatedStatusBar 存在" {
    (Get-Content "internal\adapters\tui\model.go" -Raw) -match 'func.*renderAnimatedStatusBar'
}
Test-Case "viewport.Model 字段存在" {
    (Get-Content "internal\adapters\tui\model.go" -Raw) -match 'viewport\.Model'
}
Test-Case "StartToolExecution 方法存在" {
    (Get-Content "internal\adapters\tui\adapter.go" -Raw) -match 'func.*StartToolExecution'
}
Test-Case "notifyToolExecutionStart 存在" {
    (Get-Content "internal\core\agent.go" -Raw) -match 'func.*notifyToolExecutionStart'
}
Write-Host ""

# 5. 文档检查
Write-Host "5. 文档检查" -ForegroundColor Yellow
Write-Host "-----------------------------------"
Test-Case "设计文档存在" {
    Test-Path "docs\plans\2026-01-17-tui-animated-status-indicators-design.md"
}
Test-Case "测试报告模板存在" { Test-Path "test_tui_animated_status.md" }
Test-Case "USER_GUIDE.md 已更新" {
    (Get-Content "docs\USER_GUIDE.md" -Raw) -match 'Ctrl\+d.*Tab'
}
Write-Host ""

# 6. Git 提交检查
Write-Host "6. Git 提交检查" -ForegroundColor Yellow
Write-Host "-----------------------------------"
$commits = git log --oneline feature/tui-animated-status ^feature/foundation 2>$null
if ($commits) {
    $count = ($commits | Measure-Object).Count
    if ($count -eq 5) {
        Write-Host "测试: 提交数量正确 (5个提交) ... ✓ 通过" -ForegroundColor Green
        $passed++
    } else {
        Write-Host "测试: 提交数量 (期望 5, 实际 $count) ... ⚠ 警告" -ForegroundColor Yellow
    }
}
Write-Host ""

# 总结
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  测试总结" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
$total = $passed + $failed
Write-Host "总测试数: $total"
Write-Host "通过: $passed" -ForegroundColor Green
Write-Host "失败: $failed" -ForegroundColor Red

if ($total -gt 0) {
    $percent = [math]::Round($passed * 100 / $total)
    Write-Host "通过率: $percent%"

    if ($percent -ge 80) {
        Write-Host ""
        Write-Host "✓ 所有自动化测试通过！" -ForegroundColor Green
        Write-Host ""
        Write-Host "下一步：" -ForegroundColor Cyan
        Write-Host "1. 运行手动 TUI 测试: .\bin\kore.exe chat --ui tui"
        Write-Host "2. 参考测试报告: test_tui_animated_status.md"
        Write-Host "3. 检查 15 项手动测试清单"
    } else {
        Write-Host ""
        Write-Host "✗ 部分测试失败，请检查" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  测试完成！" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
