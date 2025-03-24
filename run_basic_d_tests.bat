@echo off
setlocal enabledelayedexpansion

REM 定义测试参数
set "BASIC_DIR=E:/VScode_project/work_2/targets/targets_basic/"
set "RESULTS_DIR=results_dy_basic"
set "SCENES=sequential_blocks random_blocks batch_blocks sequential_transactions random_transactions batch_transactions"
set "DURATION=60s"
set "MAX_RATE=5000"
set "STEP_TIME=10s"


REM 创建结果目录
if not exist "%RESULTS_DIR%" mkdir "%RESULTS_DIR%"

REM 执行动态负载测试
for %%s in (%SCENES%) do (
    set "TARGET_FILE=%BASIC_DIR%targets_%%s.txt"
    echo 正在测试场景: %%s 动态负载
    vegeta attack -duration=%DURATION% -rate=%MAX_RATE%/%STEP_TIME% -targets="!TARGET_FILE!" | vegeta report > "!RESULTS_DIR!\%%s.txt"

)

echo ULTRA 动态测试完成！结果保存在 %RESULTS_DIR%