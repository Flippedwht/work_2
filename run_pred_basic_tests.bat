@echo off
setlocal enabledelayedexpansion

REM ������Բ���
set "BASIC_DIR=E:/VScode_project/work_2/targets/targets_pred_basic/"
set "RESULTS_DIR=E:/VScode_project/work_2/results/results_pred_basic_100000"
set "OUTPUT_FILE_PREFIX=results_pred_basic_"

REM �������Ŀ¼
if not exist "%RESULTS_DIR%" mkdir "%RESULTS_DIR%"

REM ����������Բ���
set "TEST_GROUP_1_RATE=500"
set "TEST_GROUP_1_DURATION=60s"
set "TEST_GROUP_2_RATE=1000"
set "TEST_GROUP_2_DURATION=120s"
set "TEST_GROUP_3_RATE=2000"
set "TEST_GROUP_3_DURATION=240s"

REM ִ���������
REM ��һ�����
set "TARGET_FILE=%BASIC_DIR%targets_pred_basic.txt"
if exist "!TARGET_FILE!" (
    echo ����ִ�е�һ�����: ���� %TEST_GROUP_1_RATE% requests/sec, ����ʱ�� %TEST_GROUP_1_DURATION%
    vegeta attack -duration=%TEST_GROUP_1_DURATION% -rate=%TEST_GROUP_1_RATE% -targets="!TARGET_FILE!" | vegeta report > "%RESULTS_DIR%\%OUTPUT_FILE_PREFIX%%TEST_GROUP_1_RATE%rate.txt"
) else (
    echo ����: Ŀ���ļ� !TARGET_FILE! �����ڣ�������һ�����
)

REM �ڶ������
set "TARGET_FILE=%BASIC_DIR%targets_pred_basic.txt"
if exist "!TARGET_FILE!" (
    echo ����ִ�еڶ������: ���� %TEST_GROUP_2_RATE% requests/sec, ����ʱ�� %TEST_GROUP_2_DURATION%
    vegeta attack -duration=%TEST_GROUP_2_DURATION% -rate=%TEST_GROUP_2_RATE% -targets="!TARGET_FILE!" | vegeta report > "%RESULTS_DIR%\%OUTPUT_FILE_PREFIX%%TEST_GROUP_2_RATE%rate.txt"
) else (
    echo ����: Ŀ���ļ� !TARGET_FILE! �����ڣ������ڶ������
)

REM ���������
set "TARGET_FILE=%BASIC_DIR%targets_pred_basic.txt"
if exist "!TARGET_FILE!" (
    echo ����ִ�е��������: ���� %TEST_GROUP_3_RATE% requests/sec, ����ʱ�� %TEST_GROUP_3_DURATION%
    vegeta attack -duration=%TEST_GROUP_3_DURATION% -rate=%TEST_GROUP_3_RATE% -targets="!TARGET_FILE!" | vegeta report > "%RESULTS_DIR%\%OUTPUT_FILE_PREFIX%%TEST_GROUP_3_RATE%rate.txt"
) else (
    echo ����: Ŀ���ļ� !TARGET_FILE! �����ڣ��������������
)

echo ���в�����ɣ���������� %RESULTS_DIR%

endlocal