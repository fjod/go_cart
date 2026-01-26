# test-all.ps1 - Run all tests across all Go modules in the workspace

param(
    [switch]$Verbose,
    [switch]$Short,        # Skip integration tests
    [switch]$Cover,        # Show coverage
    [int]$Timeout = 300    # Timeout in seconds (default 5 min for integration tests)
)

$modules = @("cart-service", "product-service", "api-gateway", "inventory-service", "checkout-service")
$failed = @()
$passed = @()

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Running tests for all modules" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

foreach ($mod in $modules) {
    if (-not (Test-Path "$mod/go.mod")) {
        Write-Host "Skipping $mod (no go.mod found)" -ForegroundColor Yellow
        continue
    }

    Write-Host "Testing $mod..." -ForegroundColor Cyan

    # Build test arguments
    $testArgs = @("./$mod/...", "-timeout", "${Timeout}s")
    if ($Verbose) { $testArgs += "-v" }
    if ($Short) { $testArgs += "-short" }
    if ($Cover) { $testArgs += "-cover" }

    go test @testArgs

    if ($LASTEXITCODE -eq 0) {
        $passed += $mod
        Write-Host "PASS: $mod" -ForegroundColor Green
        Write-Host ""
    } else {
        $failed += $mod
        Write-Host "FAIL: $mod" -ForegroundColor Red
        Write-Host ""
    }
}

# Summary
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Test Summary" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

if ($passed.Count -gt 0) {
    Write-Host "Passed: $($passed -join ', ')" -ForegroundColor Green
}
if ($failed.Count -gt 0) {
    Write-Host "Failed: $($failed -join ', ')" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "All tests passed!" -ForegroundColor Green
