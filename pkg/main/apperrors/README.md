# Error Classification System

This package provides a structured error classification system for better error handling and observability across the codebase.

## Features

- **Error Classification**: Categorize errors by type (Config, Database, API, Network, etc.)
- **Context Enrichment**: Add contextual information to errors for debugging
- **Logger Integration**: Seamless integration with the logging system
- **Error Unwrapping**: Full support for Go 1.13+ error wrapping

## Error Classes

- `ErrClassConfig` - Configuration-related errors
- `ErrClassDatabase` - Database operation errors
- `ErrClassAPI` - External API errors
- `ErrClassNetwork` - Network-related errors
- `ErrClassValidation` - Input validation errors
- `ErrClassFileSystem` - File system operation errors
- `ErrClassParsing` - Parsing and decoding errors
- `ErrClassDownload` - Download operation errors
- `ErrClassUnknown` - Unclassified errors

## Usage Examples

### Basic Error Wrapping

```go
import (
    cerrors "github.com/Kellerman81/go_media_downloader/pkg/main/errors"
)

// Wrap an existing error
if err != nil {
    return cerrors.Wrap(cerrors.ErrClassConfig, "LoadConfig", err)
}

// Create a new error
return cerrors.New(cerrors.ErrClassValidation, "ParseInput", "invalid input format")
```

### Adding Context

```go
// Add context to provide debugging information
err := cerrors.Wrap(cerrors.ErrClassDatabase, "QueryMovies", dbErr)
err = err.WithContext("query", queryString).
    WithContext("table", "movies").
    WithContext("retry_count", 3)
```

### Logger Integration

```go
import (
    cerrors "github.com/Kellerman81/go_media_downloader/pkg/main/errors"
    "github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// The logger will automatically extract classification metadata
if err != nil {
    logger.Logtype(logger.StatusError, 1).
        Err(err).
        Msg("Operation failed")
}
```

### Extracting Metadata

```go
// Extract error classification metadata
class := cerrors.GetClass(err)        // Returns ErrorClass
operation := cerrors.GetOperation(err) // Returns operation name
context := cerrors.GetContext(err)     // Returns context map
```

## Integration Points

### Config Package

```go
// config/config.go
if err != nil {
    return cerrors.Wrap(cerrors.ErrClassConfig, "LoadConfigFile", err)
}
```

### Database Package

```go
// database/bridge.go
if service == nil {
    return cerrors.New(cerrors.ErrClassDatabase, "Connect", "service not initialized")
}
```

### API External

```go
// apiexternal/metadata/client.go
if err != nil {
    return cerrors.Wrap(cerrors.ErrClassNetwork, "FetchMetadata", err).
        WithContext("url", apiURL).
        WithContext("provider", "tmdb")
}
```

## Best Practices

1. **Use Appropriate Classes**: Choose the error class that best describes the failure
2. **Add Operation Context**: Always specify the operation that failed
4. **Enrich with Context**: Add relevant debugging information
5. **Check Before Wrapping**: Don't wrap nil errors (helpers handle this automatically)

## Error Handling Patterns

### Logging Pattern

```go
if err != nil {
    logger.Logtype(logger.StatusError, 1).
        Err(err).
        Str("component", "downloader").
        Msg("Download failed")
    return err
}
```

## Migration Guide

### Before

```go
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}
```

### After

```go
if err != nil {
    return cerrors.Wrap(cerrors.ErrClassConfig, "LoadConfig",
        fmt.Errorf("failed to load config: %w", err))
}
```

## Performance Considerations

- Error classification has minimal overhead
- Context maps are allocated lazily
- No reflection or runtime type checking
- Compatible with existing error handling code

## Future Enhancements

- Automatic retry mechanisms based on classification
- Error metrics and statistics collection
- Circuit breaker integration
- Error aggregation and reporting
