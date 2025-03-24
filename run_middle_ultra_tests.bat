@echo off
setlocal enabledelayedexpansion

REM 定义测试参数
set "ULTRA_DIR=E:/VScode_project/work_2/targets/targets_middle_ultra/"
set "RESULTS_DIR=E:/VScode_project/work_2/results/results_middle_ultra"
set "SCENES=sequential_blocks sequential_transactions random_transactions batch_transactions"
set "RATES=100 500 1000"
set "DURATION=300s"

REM 创建结果目录
if not exist "%RESULTS_DIR%" mkdir "%RESULTS_DIR%"

REM 执行测试
for %%s in (%SCENES%) do (
    set "TARGET_FILE=%ULTRA_DIR%targets_%%s.txt"
    if exist "!TARGET_FILE!" (
        for %%r in (%RATES%) do (
            echo 正在测试场景: %%s, 速率: %%r requests/sec
            vegeta attack -duration=%DURATION% -rate=%%r -targets="!TARGET_FILE!" | vegeta report > "%RESULTS_DIR%\%%s_%%r.txt"
        )
    ) else (
        echo 警告: 目标文件 !TARGET_FILE! 不存在，跳过场景 %%s
    )
)

echo ULTRA 测试完成！结果保存在 %RESULTS_DIR%