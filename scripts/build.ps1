$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path dist | Out-Null
$env:CGO_ENABLED = "0"
$goos = if ($env:GOOS) { $env:GOOS } else { go env GOOS }
$goarch = if ($env:GOARCH) { $env:GOARCH } else { go env GOARCH }
$output = "dist/shim-$goos-$goarch"
if ($goos -eq "windows") { $output += ".exe" }
go build -trimpath -ldflags="-s -w" -o $output ./cmd/shim
Write-Output $output
