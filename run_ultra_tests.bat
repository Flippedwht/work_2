@echo off
setlocal enabledelayedexpansion

REM ������Բ���
set "ULTRA_DIR=E:/VScode_project/work_2/targets/targets_ultra/"
set "RESULTS_DIR=results_ultra"
set "SCENES=sequential_blocks random_blocks batch_blocks sequential_transactions random_transactions batch_transactions"
set "RATES=100 200 300 400 500"
set "DURATION=30s"

REM �������Ŀ¼
if not exist "%RESULTS_DIR%" mkdir "%RESULTS_DIR%"

REM ִ�в���
for %%s in (%SCENES%) do (
    set "TARGET_FILE=%ULTRA_DIR%targets_%%s.txt"
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