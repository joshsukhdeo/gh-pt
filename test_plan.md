The goal is to increase the code coverage of the `release` package to at least 80%, using hermetic mocking and isolated tests. Currently, coverage in `release` is around 33.3%.

Key functions to test in `release.go`:
- `installMac`
- `installWindows`
- `findChecksumFile`
- `verifyChecksum`
- `GetLatestRelease`
- `Install` (the large function)
- `extractPackageName`
- `PtermPrompter.Confirm` and `PtermPrompter.Input`
- `resolveDestinationPath` (more edge cases)
- `installBinary` (error cases)

Key functions to test in `virustotal.go`:
- `VerifyHashWithVirusTotal` (the remaining branches)
- `CalculateSHA256`
- `doVTRequestWithRetry`

We also need to use `var execCommand = exec.Command` as an execution seam, which is already in `release.go` and being used by `helperCommand`.
We will write additional test cases in `release_test.go` and `virustotal_test.go` and add a new test file if necessary, ensuring we hit >80%.
