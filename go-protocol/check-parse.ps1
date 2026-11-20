$errs = $null
$null = [System.Management.Automation.Language.Parser]::ParseFile("C:\Users\dhl\Desktop\m3\go-protocol\run-batch-1118.ps1", [ref]$null, [ref]$errs)
if ($errs -and $errs.Count -gt 0) { $errs | ForEach-Object { Write-Output ("PARSE: " + $_.Message + " @line " + $_.Extent.StartLineNumber) } } else { Write-Output "PARSE OK" }
