@echo off
setlocal enabledelayedexpansion

REM ������Բ���
set "BASIC_DIR=E:/VScode_project/work_2/targets/targets_middle_basic/"
set "RESULTS_DIR=E:/VScode_project/work_2/results/results_middle_basic"
set "SCENES=sequential_blocks sequential_transactions random_transactions batch_transactions"
set "RATES=100 500 1000"
set "DURATION=300s"

REM �������Ŀ¼
if not exist "%RESULTS_DIR%" mkdir "%RESULTS_DIR%"

REM ִ�в���
for %%s in (%SCENES%) do (
    set "TARGET_FILE=%BASIC_DIR%targets_%%s.txt"
    if exist "!TARGET_FILE!" (
        for %%r in (%RATES%) do (
            echo ���ڲ��Գ���: %%s, ����: %%r requests/sec
            vegeta attack -duration=%DURATION% -rate=%%r -targets="!TARGET_FILE!" | vegeta report > "%RESULTS_DIR%\%%s_%%r.txt"
        )
    ) else (
        echo ����: Ŀ���ļ� !TARGET_FILE! �����ڣ��������� %%s
    )
)

echo ULTRA ������ɣ���������� %RESULTS_DIR%