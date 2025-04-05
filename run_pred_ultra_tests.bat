@echo off
setlocal enabledelayedexpansion

REM 定义测试参数
set "ULTRA_DIR=E:/VScode_project/work_2/targets/targets_pred_ultra/"
set "RESULTS_DIR=E:/VScode_project/work_2/results/results_pred_ultra_100000"
set "OUTPUT_FILE_PREFIX=results_pred_ultra_"

REM 创建结果目录
if not exist "%RESULTS_DIR%" mkdir "%RESULTS_DIR%"

REM 定义三组测试参数
set "TEST_GROUP_1_RATE=500"
set "TEST_GROUP_1_DURATION=60s"
set "TEST_GROUP_2_RATE=1000"
set "TEST_GROUP_2_DURATION=120s"
set "TEST_GROUP_3_RATE=2000"
set "TEST_GROUP_3_DURATION=240s"

REM 执行三组测试
REM 第一组测试
set "TARGET_FILE=%ULTRA_DIR%targets_pred_ultra.txt"
if exist "!TARGET_FILE!" (
    echo 正在执行第一组测试: 速率 %TEST_GROUP_1_RATE% requests/sec, 持续时间 %TEST_GROUP_1_DURATION%
    vegeta attack -duration=%TEST_GROUP_1_DURATION% -rate=%TEST_GROUP_1_RATE% -targets="!TARGET_FILE!" | vegeta report > "%RESULTS_DIR%\%OUTPUT_FILE_PREFIX%%TEST_GROUP_1_RATE%rate.txt"
) else (
    echo 警告: 目标文件 !TARGET_FILE! 不存在，跳过第一组测试
)

REM 第二组测试
set "TARGET_FILE=%ULTRA_DIR%targets_pred_ultra.txt"
if exist "!TARGET_FILE!" (
    echo 正在执行第二组测试: 速率 %TEST_GROUP_2_RATE% requests/sec, 持续时间 %TEST_GROUP_2_DURATION%
    vegeta attack -duration=%TEST_GROUP_2_DURATION% -rate=%TEST_GROUP_2_RATE% -targets="!TARGET_FILE!" | vegeta report > "%RESULTS_DIR%\%OUTPUT_FILE_PREFIX%%TEST_GROUP_2_RATE%rate.txt"
) else (
    echo 警告: 目标文件 !TARGET_FILE! 不存在，跳过第二组测试
)

REM 第三组测试
set "TARGET_FILE=%ULTRA_DIR%targets_pred_ultra.txt"
if exist "!TARGET_FILE!" (
    echo 正在执行第三组测试: 速率 %TEST_GROUP_3_RATE% requests/sec, 持续时间 %TEST_GROUP_3_DURATION%
    vegeta attack -duration=%TEST_GROUP_3_DURATION% -rate=%TEST_GROUP_3_RATE% -targets="!TARGET_FILE!" | vegeta report > "%RESULTS_DIR%\%OUTPUT_FILE_PREFIX%%TEST_GROUP_3_RATE%rate.txt"
) else (
    echo 警告: 目标文件 !TARGET_FILE! 不存在，跳过第三组测试
)

echo 所有测试完成！结果保存在 %RESULTS_DIR%

endlocal