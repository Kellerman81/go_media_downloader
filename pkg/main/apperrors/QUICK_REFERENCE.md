# Error Classification Quick Reference

## Import

```go
import cerrors "github.com/Kellerman81/go_media_downloader/pkg/main/errors"
```

## Error Classes

```go
cerrors.ErrClassConfig       // Configuration errors
cerrors.ErrClassDatabase     // Database errors
cerrors.ErrClassAPI          // External API errors
cerrors.ErrClassNetwork      // Network errors
cerrors.ErrClassValidation   // Validation errors
cerrors.ErrClassFileSystem   // File system errors
cerrors.ErrClassParsing      // Parsing errors
cerrors.ErrClassDownload     // Download errors
cerrors.ErrClassUnknown      // Unknown errors
```

## Common Patterns

### Wrap Existing Error
```go
if err != nil {
    return cerrors.Wrap(cerrors.ErrClassDatabase, "QueryMovies", err)
}
```

### Create New Error
```go
return cerrors.New(cerrors.ErrClassValidation, "ParseInput", "invalid format")
```

### Add Context
```go
err.WithContext("url", apiURL).WithContext("retry", 3)
```

## Guidelines

1. **Choose appropriate class** - Use the most specific error class
2. **Name the operation** - Use descriptive operation names (e.g., "LoadConfig", "QueryMovies")
4. **Add context** - Include relevant debugging information
5. **Check nil** - Helpers handle nil automatically, no need to check

## Examples by Package

### Config
```go
cerrors.Wrap(cerrors.ErrClassConfig, "LoadConfigFile", err)
cerrors.Wrap(cerrors.ErrClassValidation, "ValidateConfig", err)
```

### Database
```go
cerrors.Wrap(cerrors.ErrClassDatabase, "Connect", err)
cerrors.Wrap(cerrors.ErrClassDatabase, "QueryTimeout", err)
```

### API External
```go
cerrors.Wrap(cerrors.ErrClassNetwork, "FetchMetadata", err).
    WithContext("provider", "tmdb")
```

### Downloader
```go
cerrors.Wrap(cerrors.ErrClassDownload, "DownloadFile", err).
    WithContext("url", downloadURL)
```

### Parser
```go
cerrors.Wrap(cerrors.ErrClassParsing, "ParseFileName", err).
    WithContext("file", filename)
```

### Scanner
```go
cerrors.Wrap(cerrors.ErrClassFileSystem, "ScanDirectory", err).
    WithContext("path", dirPath)
```
