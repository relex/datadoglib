# datadoglib
Datadog simulation tool / mock

## Usage
```go run main.go```

### Flags:
- `port` - overrides the application default port (8083)
- `random_no_auth` - chance to mock a failed auth error and get a 403 response. float 0.0 to 1.0
- `random_no_receive` - chance for a request to hang for a while before responding. float 0.0 to 1.0
- `random_bad_response` - chance to receive a status 500 response. float 0.0 to 1.0
