# Ticket 2: Remote ZIP Central Directory Peeking via HTTP Range Requests

- **Type**: `wayfinder:research`
- **Status**: Closed
- **Assignee**: Closed by Research Subagent
- **Blocked By**: None (Frontier)

## Question
What is the optimal implementation for reading the ZIP Central Directory over HTTP Range requests using Go standard library (`archive/zip`) or lightweight packages, specifically handling GitHub Release asset redirects, authentication headers, and caching?

## Resolution
Standard Go `archive/zip.Reader` supports `io.ReaderAt` backed by HTTP `Range: bytes=start-end` requests:
- **EOCD & Central Directory**: Reads only the last ~64KB of the remote file via HTTP Range to parse the End of Central Directory record and index of files without downloading the body (<100KB transferred).
- **GitHub Redirects**: Follows `302 Found` to AWS S3/Azure Blob CDN. Go's `net/http` automatically strips the GitHub `Authorization` header across domain boundaries, preventing S3 400 errors while preserving `Range` headers.
- **Connection Optimization**: Use a shared `http.Client` with Keep-Alives to prevent socket reconnect overhead.
