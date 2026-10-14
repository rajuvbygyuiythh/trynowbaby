@echo off
setlocal
rem netdoctor - direct + proxy connectivity report for any site.
rem Builds the tool once into build\bin\netdoctor.exe, then runs it.
rem Examples:
rem   netdoctor.bat -proxies proxies.txt -targets https://app.chitchat.gg/,https://newsite.com -exitip
rem   netdoctor.bat -targets https://newsite.com -direct
set "ROOT=%~dp0"
set "BIN=%ROOT%build\bin\netdoctor.exe"

where go >nul 2>nul
if errorlevel 1 (
  echo Go was not found. Run install.bat first, or install Go 1.25+.
  exit /b 1
)

if not exist "%BIN%" (
  echo Building netdoctor.exe ...
  pushd "%ROOT%"
  go build -o "%BIN%" .\cmd\netdoctor
  if errorlevel 1 (
    popd
    echo Build failed.
    exit /b 1
  )
  popd
)

"%BIN%" %*
set "CODE=%ERRORLEVEL%"
if not "%CODE%"=="0" (
  echo.
  echo netdoctor exit code %CODE% : 1 = something failed, 2 = wrong arguments.
)
exit /b %CODE%
